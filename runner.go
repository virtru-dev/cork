package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"os"
	"os/user"

	log "github.com/Sirupsen/logrus"

	"github.com/virtru/cork/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/phayes/freeport"
	uuid "github.com/satori/go.uuid"
	"github.com/virtru/cork/utils/dockerutils"
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
	ForcePullImage  bool
	Debug           bool
	Commander       *dockerutils.DockerCommander
}

type CorkTypeContainerOptions struct {
	Debug           bool
	ImageName       string
	CacheVolumeName string
	ProjectName     string
	ForcePullImage  bool
}

// Creates a new cork runner
func New(dockerClient *docker.Client, control *Control, options CorkTypeContainerOptions) (*CorkTypeContainer, error) {
	dockerHostPath := os.Getenv("DOCKER_HOST")
	if dockerHostPath == "" {
		dockerHostPath = "/var/run/docker.sock"
	}

	if options.ImageName == "" {
		return nil, fmt.Errorf("ImageName must be defined")
	}

	if options.ProjectName == "" {
		return nil, fmt.Errorf("ProjectName must be defined")
	}

	if options.CacheVolumeName == "" {
		return nil, fmt.Errorf("CacheVolumeName must be defined")
	}

	runner := CorkTypeContainer{
		DockerClient:    dockerClient,
		Image:           options.ImageName,
		Name:            fmt.Sprintf("cork-%s", uuid.NewV4()),
		DockerHostPath:  dockerHostPath,
		CacheVolumeName: options.CacheVolumeName,
		Control:         control,
		ProjectName:     options.ProjectName,
		ForcePullImage:  options.ForcePullImage,
		Debug:           options.Debug,
	}
	return &runner, nil
}

func (c *CorkTypeContainer) Start(stageName string) error {
	sshPort := freeport.GetPort()
	corkPort := freeport.GetPort()
	c.SSHPort = sshPort
	c.CorkPort = corkPort

	log.Debugf("sshPort=%d corkPort=%d", sshPort, corkPort)
	commander, err := c.createCommander()
	if err != nil {
		return err
	}
	c.Commander = commander

	err = commander.Start()
	if err != nil {
		return err
	}
	defer commander.Kill()

	c.Control.OnTerminate(func() {
		commander.Kill()
	})

	err = c.startSSHCommand(stageName)
	if err != nil {
		log.Debugf("Error occured running SSH Command")
		return err
	}
	return nil
}

func (c *CorkTypeContainer) connectClient() (*client.Client, error) {
	log.Debugf("Connecting to cork server on port %d", c.CorkPort)
	//time.Sleep(200 * time.Second)
	var err error
	for i := 0; i < maxRetries; i++ {
		corkClient, err := client.New(fmt.Sprintf(":%d", c.CorkPort))
		if err == nil {
			statusErr := corkClient.Status()
			if statusErr == nil {
				return corkClient, nil
			}
			if grpc.Code(statusErr) == codes.Internal {
				if strings.Contains(statusErr.Error(), "InitializationError") {
					log.Debugf("An InitializationError occured. The startup hook probably failed")
					return nil, statusErr
				}
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
		if err != nil {
			log.Debugf("Error occured connecting to the client")
			clientErrChan <- err
			return
		}
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

	log.Debugf("Connecting to docker container %s ssh on port %d", c.Commander.Container.ID, c.SSHPort)
	debugFlag := ""
	if c.Debug {
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

func (c *CorkTypeContainer) createCommander() (*dockerutils.DockerCommander, error) {
	pwd, err := c.Pwd()
	if err != nil {
		return nil, err
	}

	usr, err := user.Current()
	if err != nil {
		return nil, err
	}
	homeDir := usr.HomeDir

	setCorkVars := []string{
		"DOCKER_HOST",
		"CORK_PORT",
		"CORK_WORK_DIR",
		"CORK_CACHE_DIR",
		"CORK_HOST_WORK_DIR",
		"CORK_PROJECT_NAME",
	}

	options := dockerutils.DockerCommanderOptions{
		Image:          c.Image,
		ForcePullImage: c.ForcePullImage,
		Env: []string{
			"DOCKER_HOST=unix:///var/run/docker.sock",
			"CORK_PORT=11900",
			"CORK_WORK_DIR=/work",
			"CORK_CACHE_DIR=/cork-cache",
			fmt.Sprintf("CORK_HOST_WORK_DIR=%s", pwd),
			fmt.Sprintf("CORK_PROJECT_NAME=%s", c.ProjectName),
		},
		Expose: []int{
			22,
			11900,
		},
		Binds: []string{
			fmt.Sprintf("%s:/var/run/docker.sock", c.DockerHostPath),
			fmt.Sprintf("%s:/work", pwd),
			fmt.Sprintf("%s:/source_root", homeDir),
			fmt.Sprintf("%s:/cork-cache", c.CacheVolumeName),
		},
		Privileged: true,
		AutoRemove: true,
		Ports: []string{
			fmt.Sprintf("%d:11900", c.CorkPort),
			fmt.Sprintf("%d:22", c.SSHPort),
		},
		EnsureNamedVolumes: []string{
			c.CacheVolumeName,
		},
	}

	if c.Debug {
		fmt.Println("DEBUg???")
		options.Env = append(options.Env, "CORK_DEBUG=true")
		setCorkVars = append(setCorkVars, "CORK_DEBUG")
	}

	setCorkVarsStr := strings.Join(setCorkVars, ",")
	options.Env = append(options.Env, fmt.Sprintf("CORK_VARS=%s", setCorkVarsStr))

	commander := dockerutils.NewCommander(c.DockerClient, options)
	if c.Debug {
		commander.SetStdio()
	}
	return commander, nil
}
