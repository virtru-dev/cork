package main

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"os"
	"os/user"

	log "github.com/Sirupsen/logrus"

	"github.com/virtru/cork/client"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/phayes/freeport"
	uuid "github.com/satori/go.uuid"
)

type VolumeMap map[string]string

var serverCommandTemplate = "/cork-server -e %s serve"

// CorkTypeContainer - Runs a cork job in a container
type CorkTypeContainer struct {
	Name            string
	Image           string
	DockerClient    *docker.Client
	Container       *docker.Container
	DockerHostPath  string
	Failed          chan bool
	Control         *Control
	SSHPort         int
	CorkPort        int
	CacheVolumeName string
	Env             []string
	ProjectName     string
}

// Creates a new cork runner
func New(projectName string, dockerClient *docker.Client, imageName string, cacheVolumeName string, control *Control) (*CorkTypeContainer, error) {
	dockerHostPath := os.Getenv("DOCKER_HOST")
	if dockerHostPath == "" {
		dockerHostPath = "/var/run/docker.sock"
	}

	runner := CorkTypeContainer{
		DockerClient:    dockerClient,
		Image:           imageName,
		Name:            fmt.Sprintf("cork-%s", uuid.NewV4()),
		DockerHostPath:  dockerHostPath,
		CacheVolumeName: cacheVolumeName,
		Control:         control,
		ProjectName:     projectName,
	}
	return &runner, nil
}

func (c *CorkTypeContainer) CreateContainer() error {
	containerConfig, err := c.CreateContainerConfig()
	if err != nil {
		return err
	}

	hostConfig, err := c.CreateHostConfig()
	if err != nil {
		return err
	}

	params := docker.CreateContainerOptions{
		Name:       c.Name,
		Config:     containerConfig,
		HostConfig: hostConfig,
	}
	container, err := c.DockerClient.CreateContainer(params)
	if err != nil {
		return err
	}
	c.Container = container
	return nil
}

func (c *CorkTypeContainer) Start(stageName string) error {
	sshPort := freeport.GetPort()
	corkPort := freeport.GetPort()
	c.SSHPort = sshPort
	c.CorkPort = corkPort

	log.Debugf("sshPort=%d corkPort=%d", sshPort, corkPort)
	err := c.CreateContainer()
	if err != nil {
		return err
	}

	err = c.EnsureCacheVolumeExists()
	if err != nil {
		return err
	}

	err = c.DockerClient.StartContainer(c.Container.ID, c.Container.HostConfig)
	if err != nil {
		return err
	}
	defer c.KillContainer()

	c.Control.OnTerminate(func() {
		c.KillContainer()
	})

	err = c.startSSHCommand(stageName)
	if err != nil {
		log.Debugf("Error occured running SSH Command")
		return err
	}
	return nil
}

func (c *CorkTypeContainer) EnsureCacheVolumeExists() error {
	log.Debugf("Ensuring cache volume %s", c.CacheVolumeName)
	_, err := c.DockerClient.InspectVolume(c.CacheVolumeName)
	if err != nil {
		log.Errorf("Error inspecting volume: %v", err)
		return err
	}

	options := docker.CreateVolumeOptions{
		Name: c.CacheVolumeName,
	}
	_, err = c.DockerClient.CreateVolume(options)
	return err
}

func (c *CorkTypeContainer) KillContainer() {
	log.Debugf("Killing container %s", c.Container.ID)
	params := docker.KillContainerOptions{
		ID: c.Container.ID,
	}
	_ = c.DockerClient.KillContainer(params)
}

func (c *CorkTypeContainer) connectClient() (*client.Client, error) {
	log.Debugf("Connecting to cork server on port %d", c.CorkPort)
	var err error
	for i := 0; i < maxRetries; i++ {
		corkClient, err := client.New(fmt.Sprintf(":%d", c.CorkPort))
		if err == nil {
			statusErr := corkClient.Status()
			if statusErr == nil {
				return corkClient, nil
			}
			corkClient.Close()
		}
		time.Sleep(1 * time.Second)
		log.Debugf("Retrying connection to cork server on port %d", c.CorkPort)
	}
	log.Fatalf("Failed to connect to grpc client on port %d", c.CorkPort)
	return nil, err
}

func (c *CorkTypeContainer) runClient(stageName string, clientErrChan chan error) {
	go func() {
		corkClient, err := c.connectClient()
		defer corkClient.Close()

		log.Debugf("Running stage %s", stageName)
		err = corkClient.StageExecute(stageName)
		if err != nil {
			log.Debugf("Error occured running StageExecute")
			clientErrChan <- err
			return
		}

		log.Debugf("Stage executed successfully. Killing cork server")
		err = corkClient.Kill()
		if err != nil {
			clientErrChan <- err
			return
		}
		clientErrChan <- io.EOF
	}()
}

func (c *CorkTypeContainer) startSSHCommand(stageName string) error {
	failed := make(chan bool)

	//time.Sleep(100 * time.Second)
	log.Debugf("Connecting to docker container %s ssh on port %d", c.Container.ID, c.SSHPort)
	debugFlag := ""
	if os.Getenv("CORK_DEBUG") == "true" {
		debugFlag = "--debug"
	}
	serverCommand := fmt.Sprintf(serverCommandTemplate, debugFlag)
	command := NewDockerSSHCommand("127.0.0.1", c.SSHPort, serverCommand, failed)

	log.Debugf("Running SSH with env %v", c.Env)
	command.Start(c.Env)
	clientErrChan := make(chan error)

	c.runClient(stageName, clientErrChan)

	for {
		select {
		case clientErr := <-clientErrChan:
			if clientErr != io.EOF {
				return clientErr
			}
		case commandStatusFailed := <-failed:
			if commandStatusFailed {
				return fmt.Errorf("Command failed")
			}
			return nil
		}
	}
}

func (c *CorkTypeContainer) Pwd() (string, error) {
	return os.Getwd()
}

func (c *CorkTypeContainer) CreateContainerConfig() (*docker.Config, error) {
	pwd, err := c.Pwd()
	if err != nil {
		return nil, err
	}

	config := docker.Config{
		Image: c.Image,
		Env: []string{
			"CORK_VARS=DOCKER_HOST,CORK_PORT,CORK_WORK_DIR,CORK_HOST_WORK_DIR,CORK_CACHE_DIR,CORK_PROJECT_NAME",
			"DOCKER_HOST=unix:///var/run/docker.sock",
			"CORK_PORT=11900",
			"CORK_WORK_DIR=/work",
			"CORK_CACHE_DIR=/cork-cache",
			fmt.Sprintf("CORK_HOST_WORK_DIR=%s", pwd),
			fmt.Sprintf("CORK_PROJECT_NAME=%s", c.ProjectName),
		},
		ExposedPorts: map[docker.Port]struct{}{"22/tcp": {}, "11900/tcp": {}},
	}
	if os.Getenv("CORK_DEBUG") == "true" {
		config.Env = append(config.Env, "CORK_DEBUG=true")
	}
	c.Env = config.Env

	log.Debugf("Setting Docker Config %+v", config)
	return &config, nil
}

func (c *CorkTypeContainer) CreateHostConfig() (*docker.HostConfig, error) {
	pwd, err := c.Pwd()
	if err != nil {
		return nil, err
	}

	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	homeDir := usr.HomeDir

	config := docker.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/var/run/docker.sock", c.DockerHostPath),
			fmt.Sprintf("%s:/work", pwd),
			fmt.Sprintf("%s:/source_root", homeDir),
			fmt.Sprintf("%s:/cork-cache", c.CacheVolumeName),
		},
		Privileged: true,
		AutoRemove: true,
		PortBindings: map[docker.Port][]docker.PortBinding{
			"11900/tcp": []docker.PortBinding{docker.PortBinding{HostPort: strconv.Itoa(c.CorkPort)}},
			"22/tcp":    []docker.PortBinding{docker.PortBinding{HostPort: strconv.Itoa(c.SSHPort)}},
		},
	}
	log.Debugf("Setting Docker HostConfig %+v", config)

	return &config, nil
}
