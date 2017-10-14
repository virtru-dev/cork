package main

import (
	"fmt"
	"strings"

	"os"

	"regexp"

	log "github.com/sirupsen/logrus"

	"github.com/fsouza/go-dockerclient"
	"github.com/renstrom/fuzzysearch/fuzzy"
)

var authHTTPRegex = regexp.MustCompile("^https?://")

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
			log.Errorf("Error: %+v", err)
			continue
		}
		return nil
	}
	return fmt.Errorf("PullImage failed.")
}

func createContainer(client *docker.Client, options docker.CreateContainerOptions) (*docker.Container, error) {
	auth, err := docker.NewAuthConfigurationsFromDockerCfg()
	if err != nil {
		return nil, err
	}
	repo, tag := docker.ParseRepositoryTag(options.Config.Image)
	err = client.PullImage(docker.PullImageOptions{
		Repository: repo,
		Tag:        tag,
	}, auth.Configs["hello"])
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func main() {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		panic(err)
	}

	err = tryPullImage(client, "redis:latest")
	if err != nil {
		panic(err)
	}

	options := docker.CreateContainerOptions{
		Name: "this-is-a-test",
		Config: &docker.Config{
			Image: "redis:latest",
			Cmd:   []string{"echo", "'hello'"},
		},
	}

	container, err := client.CreateContainer(options)
	if err != nil {
		panic(err)
	}

	waiter, err := client.AttachToContainerNonBlocking(docker.AttachToContainerOptions{
		Container:    container.ID,
		OutputStream: os.Stdout,
		Logs:         true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		RawTerminal:  true,
	})
	defer waiter.Close()
	if err != nil {
		panic(err)
	}

	err = client.StartContainer(container.ID, &docker.HostConfig{})
	if err != nil {
		panic(err)
	}

	err = client.KillContainer(docker.KillContainerOptions{
		ID: container.ID,
	})
	if err != nil {
		panic(err)
	}

	err = client.RemoveContainer(docker.RemoveContainerOptions{
		ID: container.ID,
	})
	if err != nil {
		panic(err)
	}

	err = waiter.Wait()
	if err != nil {
		panic(err)
	}
}
