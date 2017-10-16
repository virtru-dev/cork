// Copyright 2015 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dockerutils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

// dockerConfig represents a registry authentation configuration from the
// .dockercfg file.
type dockerConfig struct {
	Auth       string `json:"auth"`
	Email      string `json:"email"`
	CredsStore string
}

type credentialHelperResponse struct {
	ServerURL string `json:"ServerURL"`
	Username  string `json:"Username"`
	Secret    string `json:"Secret"`
}

// NewAuthConfigurationsFromFile returns AuthConfigurations from a path containing JSON
// in the same format as the .dockercfg file.
func NewAuthConfigurationsFromFile(path string) (*docker.AuthConfigurations, error) {
	r, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return NewAuthConfigurations(r)
}

func cfgPaths(dockerConfigEnv string, homeEnv string) []string {
	var paths []string
	if dockerConfigEnv != "" {
		paths = append(paths, path.Join(dockerConfigEnv, "config.json"))
	}
	if homeEnv != "" {
		paths = append(paths, path.Join(homeEnv, ".docker", "config.json"))
		paths = append(paths, path.Join(homeEnv, ".dockercfg"))
	}
	return paths
}

// NewAuthConfigurationsFromDockerCfg returns AuthConfigurations from
// system config files. The following files are checked in the order listed:
// - $DOCKER_CONFIG/config.json if DOCKER_CONFIG set in the environment,
// - $HOME/.docker/config.json
// - $HOME/.dockercfg
func NewAuthConfigurationsFromDockerCfg() (*docker.AuthConfigurations, error) {
	err := fmt.Errorf("No docker configuration found")
	var auths *docker.AuthConfigurations

	pathsToTry := cfgPaths(os.Getenv("DOCKER_CONFIG"), os.Getenv("HOME"))
	for _, path := range pathsToTry {
		auths, err = NewAuthConfigurationsFromFile(path)
		if err == nil {
			return auths, nil
		}
	}
	return auths, err
}

// NewAuthConfigurations returns AuthConfigurations from a JSON encoded string in the
// same format as the .dockercfg file.
func NewAuthConfigurations(r io.Reader) (*docker.AuthConfigurations, error) {
	var auth *docker.AuthConfigurations
	confs, err := parseDockerConfig(r)
	if err != nil {
		return nil, err
	}
	log.Debugf("Docker Configs %+v", confs)
	auth, err = authConfigs(confs)
	if err != nil {
		return nil, err
	}
	return auth, nil
}

func parseDockerConfig(r io.Reader) (map[string]dockerConfig, error) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
	byteData := buf.Bytes()

	confsWrapper := struct {
		Auths      map[string]dockerConfig `json:"auths"`
		CredsStore string                  `json:"credsStore"`
	}{}
	if err := json.Unmarshal(byteData, &confsWrapper); err == nil {
		if len(confsWrapper.Auths) > 0 {
			log.Debug("here i am 1")
			for name, auth := range confsWrapper.Auths {
				auth.CredsStore = confsWrapper.CredsStore
				confsWrapper.Auths[name] = auth
			}
			return confsWrapper.Auths, nil
		}
	}

	var confs map[string]dockerConfig
	if err := json.Unmarshal(byteData, &confs); err != nil {
		return nil, err
	}
	return confs, nil
}

func loadCredentialsFromCredsStore(registryUrl string, config dockerConfig) (*docker.AuthConfiguration, error) {
	subProc := exec.Command(fmt.Sprintf("docker-credential-%s", config.CredsStore), "get")

	subProc.Stdin = strings.NewReader(fmt.Sprintf("%s\n", registryUrl))
	var output bytes.Buffer
	subProc.Stdout = &output

	if err := subProc.Run(); err != nil {
		return nil, err
	}

	log.Debug("loading raw response")
	rawResponse := output.Bytes()

	var credsResponse credentialHelperResponse
	if err := json.Unmarshal(rawResponse, &credsResponse); err != nil {
		return nil, err
	}
	return &docker.AuthConfiguration{
		Username:      credsResponse.Username,
		Password:      credsResponse.Secret,
		ServerAddress: credsResponse.ServerURL,
	}, nil
}

// authConfigs converts a dockerConfigs map to a AuthConfigurations object.
func authConfigs(confs map[string]dockerConfig) (*docker.AuthConfigurations, error) {
	c := &docker.AuthConfigurations{
		Configs: make(map[string]docker.AuthConfiguration),
	}
	for reg, conf := range confs {
		log.Debugf("docker confs %+v", conf)
		if conf.CredsStore != "" {
			auth, err := loadCredentialsFromCredsStore(reg, conf)
			if err != nil {
				return nil, err
			}
			c.Configs[reg] = *auth
		} else {
			data, err := base64.StdEncoding.DecodeString(conf.Auth)
			if err != nil {
				return nil, err
			}
			userpass := strings.SplitN(string(data), ":", 2)
			if len(userpass) != 2 {
				return nil, docker.ErrCannotParseDockercfg
			}
			c.Configs[reg] = docker.AuthConfiguration{
				Email:         conf.Email,
				Username:      userpass[0],
				Password:      userpass[1],
				ServerAddress: reg,
			}
		}
	}
	log.Debugf("AuthConfigs=%+v", c)
	return c, nil
}
