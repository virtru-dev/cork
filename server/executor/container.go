package executor

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/renstrom/fuzzysearch/fuzzy"
	"github.com/virtru/cork/server/definition"
	"github.com/virtru/cork/server/streamer"
)

var authHTTPRegex = regexp.MustCompile("^https?://")

func init() {
	RegisterHandler("container", ContainerStepHandler)
}

func narrowAuthSearch(repo string, auths *docker.AuthConfigurations) []docker.AuthConfiguration {
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

func tryPullImage(client *docker.Client, image string) error {
	auths, err := docker.NewAuthConfigurationsFromDockerCfg()
	if err != nil {
		return err
	}

	repo, tag := docker.ParseRepositoryTag(image)

	authConfigs := narrowAuthSearch(repo, auths)

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

// ContainerStepHandler - Handles executing a command step
func ContainerStepHandler(corkDir string, executor *StepsExecutor, stream streamer.StepStream, step *definition.Step) (map[string]string, error) {
	log.Debugf("Running container step %s", step.Name)
	return nil, nil
}
