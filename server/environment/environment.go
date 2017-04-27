package environment

import (
	"encoding/json"
	"os"
	"strings"

	"io/ioutil"
)

type EnvironRetriever func(string) string

func SaveEnvFileWithRetriever(getenv EnvironRetriever, envPath string) error {
	corkVarsRaw := getenv("CORK_VARS")
	corkVars := strings.Split(corkVarsRaw, ",")

	corkVarsMap := make(map[string]string)
	for _, corkVar := range corkVars {
		corkVarsMap[corkVar] = getenv(corkVar)
	}
	corkVarsMapJSONBytes, err := json.Marshal(corkVarsMap)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(envPath, corkVarsMapJSONBytes, 0700)
	if err != nil {
		return err
	}
	return nil
}

func SaveEnvFile(envPath string) error {
	return SaveEnvFileWithRetriever(os.Getenv, envPath)
}

func LoadEnvFile(envPath string) (map[string]string, error) {
	var corkVarsMap map[string]string

	corkVarsJSONBytes, err := ioutil.ReadFile(envPath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(corkVarsJSONBytes, &corkVarsMap)
	if err != nil {
		return nil, err
	}

	return corkVarsMap, nil
}
