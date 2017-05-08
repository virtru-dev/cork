package dockerutils

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"io"

	log "github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/kballard/go-shellquote"
	"github.com/renstrom/fuzzysearch/fuzzy"
	uuid "github.com/satori/go.uuid"
)

// DockerCommander - A high level docker interface
type DockerCommander struct {
	Client      *docker.Client
	Container   *docker.Container
	CloseWaiter docker.CloseWaiter
	Options     DockerCommanderOptions
}

type DockerCommanderOptions struct {
	// The name of the docker container
	Name string

	// The image to use
	Image string

	// Overrides the command
	Cmd string

	// Which ports to bind to the host in the form HOST:CONTAINER
	Ports []string

	// Which ports to expose
	Expose []int

	// Volumes to bind
	Binds []string

	// Named volumes to ensure exist
	EnsureNamedVolumes []string

	// Forces the image to get pulled
	ForcePullImage bool

	// The environment
	Env []string

	// Run the docker container in privileged mode
	Privileged bool

	// Auto remove the docker container when it quits
	AutoRemove bool

	Stdin  bool
	Stdout bool
	Stderr bool

	InputStream  io.Reader
	OutputStream io.Writer
	ErrorStream  io.Writer

	PropagateKillError bool
}

var authHTTPRegex = regexp.MustCompile("^https?://")

// NarrowAuthSearch - Narrows down auth search
func NarrowAuthSearch(repo string, auths *docker.AuthConfigurations) []docker.AuthConfiguration {
	var authConfigs []docker.AuthConfiguration
	used := make(map[string]bool)

	splitRepo := strings.Split(repo, "/")
	if len(splitRepo) == 0 {
		return authConfigs
	}

	var authKeys []string
	for key := range auths.Configs {
		authKeys = append(authKeys, key)
	}

	matches := fuzzy.Find(splitRepo[0], authKeys)
	for _, match := range matches {
		used[match] = true
		authConfigs = append(authConfigs, auths.Configs[match])
	}
	if !authHTTPRegex.MatchString(splitRepo[0]) {
		searchStr := fmt.Sprintf("http://%s", splitRepo[0])
		httpMatches := fuzzy.Find(searchStr, authKeys)
		for _, match := range httpMatches {
			_, ok := used[match]
			if !ok {
				authConfigs = append(authConfigs, auths.Configs[match])
			}
		}
	}
	return authConfigs
}

// TryImagePull - Attempts to pull an image
func TryImagePull(client *docker.Client, image string) error {
	auths, err := docker.NewAuthConfigurationsFromDockerCfg()
	if err != nil {
		return err
	}

	repo, tag := docker.ParseRepositoryTag(image)

	authConfigs := NarrowAuthSearch(repo, auths)

	if len(authConfigs) == 0 {
		authConfigs = []docker.AuthConfiguration{
			docker.AuthConfiguration{},
		}
	}

	pullImageOptions := docker.PullImageOptions{
		Repository:   repo,
		Tag:          tag,
		OutputStream: os.Stdout,
	}

	for _, authConfig := range authConfigs {
		err = client.PullImage(pullImageOptions, authConfig)
		if err != nil {
			log.Debugf("Error: %+v", err)
			continue
		}
		return nil
	}
	return fmt.Errorf("PullImage failed.")
}

func NewCommander(client *docker.Client, options DockerCommanderOptions) *DockerCommander {
	return &DockerCommander{
		Client:  client,
		Options: options,
	}
}

func (dc *DockerCommander) pullImage() error {
	err := TryImagePull(dc.Client, dc.Options.Image)
	if err != nil {
		return err
	}
	return nil
}

func (dc *DockerCommander) ensureImage() error {
	if dc.Options.ForcePullImage {
		return dc.pullImage()
	}
	_, err := dc.Client.InspectImage(dc.Options.Image)
	if err != nil {
		if err == docker.ErrNoSuchImage {
			// Try to pull the image
			return dc.pullImage()
		}
		return err
	}
	return nil
}

func (dc *DockerCommander) createHostConfig() (*docker.HostConfig, error) {
	config := docker.HostConfig{
		Binds:      dc.Options.Binds,
		Privileged: dc.Options.Privileged,
		AutoRemove: dc.Options.AutoRemove,
	}

	portBindings := make(map[docker.Port][]docker.PortBinding)

	for _, port := range dc.Options.Ports {
		portSplit := strings.Split(port, ":")
		if len(portSplit) > 2 {
			return nil, fmt.Errorf("Invalid port definition must be PORT_NUM or HOST:CONTAINER")
		}
		if len(portSplit) == 1 {
			portSplit = []string{portSplit[0], portSplit[0]}
		}
		portToBind := docker.Port(fmt.Sprintf("%s/tcp", portSplit[1]))
		portBindings[portToBind] = []docker.PortBinding{docker.PortBinding{
			HostPort: portSplit[0],
		}}
	}

	log.Debugf("PortBindings %+v", portBindings)

	config.PortBindings = portBindings
	return &config, nil
}

func (dc *DockerCommander) createContainerConfig() (*docker.Config, error) {
	config := docker.Config{
		Image: dc.Options.Image,
		Env:   dc.Options.Env,
	}

	if dc.Options.Cmd != "" {
		cmdSplit, err := shellquote.Split(dc.Options.Cmd)
		if err != nil {
			return nil, err
		}
		config.Cmd = cmdSplit
	}

	exposedPorts := make(map[docker.Port]struct{})

	for _, exposed := range dc.Options.Expose {
		exposedPort := docker.Port(fmt.Sprintf("%d/tcp", exposed))
		exposedPorts[exposedPort] = struct{}{}
	}

	log.Debugf("ExposedPorts %+v", exposedPorts)

	config.ExposedPorts = exposedPorts
	return &config, nil
}

func (dc *DockerCommander) ensureVolumesExist() error {
	for _, namedVolume := range dc.Options.EnsureNamedVolumes {
		log.Debugf("Ensuring volume %s", namedVolume)
		_, err := dc.Client.InspectVolume(namedVolume)
		if err != nil {
			if err != docker.ErrNoSuchVolume {
				log.Errorf("Error inspecting volume: %v", err)
				return err
			}
		}
		options := docker.CreateVolumeOptions{
			Name: namedVolume,
		}
		_, err = dc.Client.CreateVolume(options)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dc *DockerCommander) createContainer() error {
	containerConfig, err := dc.createContainerConfig()
	if err != nil {
		return err
	}

	hostConfig, err := dc.createHostConfig()
	if err != nil {
		return err
	}

	params := docker.CreateContainerOptions{
		Name:       dc.name(),
		Config:     containerConfig,
		HostConfig: hostConfig,
	}

	container, err := dc.Client.CreateContainer(params)
	if err != nil {
		return err
	}

	dc.Container = container
	return nil
}

func (dc *DockerCommander) name() string {
	if dc.Options.Name == "" {
		uuidName := fmt.Sprintf("cork-%s", uuid.NewV4())
		dc.Options.Name = uuidName
	}
	return dc.Options.Name
}

func (dc *DockerCommander) attachToContainer() error {
	options := docker.AttachToContainerOptions{
		Container:   dc.Container.ID,
		RawTerminal: true,
		Logs:        true,
		Stream:      true,
	}
	if dc.Options.Stderr {
		options.Stderr = true
		options.ErrorStream = dc.Options.ErrorStream
	}
	if dc.Options.Stdout {
		options.Stdout = true
		options.OutputStream = dc.Options.OutputStream
	}
	if dc.Options.Stdin {
		options.Stdin = true
		options.InputStream = dc.Options.InputStream
	}

	closeWaiter, err := dc.Client.AttachToContainerNonBlocking(options)
	if err != nil {
		return err
	}
	dc.CloseWaiter = closeWaiter

	return nil
}

func (dc *DockerCommander) SetStdio() {
	dc.Options.Stdin = true
	dc.Options.Stdout = true
	dc.Options.Stderr = true
	dc.Options.ErrorStream = os.Stderr
	dc.Options.InputStream = os.Stdin
	dc.Options.OutputStream = os.Stdout
}

func (dc *DockerCommander) Start() error {
	err := dc.ensureImage()
	if err != nil {
		return err
	}

	err = dc.ensureVolumesExist()
	if err != nil {
		return err
	}

	err = dc.createContainer()
	if err != nil {
		return err
	}

	err = dc.attachToContainer()
	if err != nil {
		return err
	}

	err = dc.Client.StartContainer(dc.Container.ID, dc.Container.HostConfig)
	if err != nil {
		return err
	}
	return nil
}

func (dc *DockerCommander) Kill() error {
	log.Debugf("Killing container %s", dc.Container.ID)

	err := dc.CloseWaiter.Close()
	if err != nil {
		return err
	}

	err = dc.CloseWaiter.Wait()
	if err != nil {
		return err
	}

	params := docker.KillContainerOptions{
		ID: dc.Container.ID,
	}
	err = dc.Client.KillContainer(params)
	if err != nil {
		if !dc.Options.PropagateKillError && strings.Contains(err.Error(), "is not running") {
			return nil
		}
		return err
	}
	return nil
}
