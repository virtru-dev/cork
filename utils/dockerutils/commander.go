package dockerutils

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	log "github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/renstrom/fuzzysearch/fuzzy"
)

// DockerCommander - A high level docker interface
type DockerCommander struct {
	Client *docker.Client
	Image  string
}

type DockerCommanderRunOptions struct {
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

func NewCommander(client *docker.Client, image string) *DockerCommander {
	return &DockerCommander{
		Client: client,
		Image:  image,
	}
}

func (dc *DockerCommander) Run() {

}
