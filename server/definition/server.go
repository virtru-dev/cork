package definition

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"gopkg.in/yaml.v2"
)

const maxDepth = 50

// TemplateRenderer - Interface to user for the template rendering
type TemplateRenderer interface {
	Render(templateStr string)
}

// StepTypes - Defines step types
var StepTypes = map[string]bool{
	"stage":     true,
	"container": true,
	"command":   true,
	"export":    true,
}

type Volumes struct {
	Names  []string `yaml:"names"`
	Mounts []string `yaml:"mounts"`
}

// ServerDefinition - Defines a cork server
type ServerDefinition struct {
	Stages  map[string]Stage `yaml:"stages"`
	Volumes Volumes          `yaml:"volumes"`
	Tags    []string         `yaml:"tags"`
	Version int              `yaml:"version"`
}

// Load - Loads the server definition from the default location
func LoadFromDir(corkDir string) (*ServerDefinition, error) {
	defPath := path.Join(corkDir, "definition.yml")
	return LoadFromPath(defPath)
}

// LoadFromPath - Loads the server definition from the specified path
func LoadFromPath(defPath string) (*ServerDefinition, error) {
	defFile, err := os.Open(defPath)
	if err != nil {
		return nil, err
	}
	defer defFile.Close()
	defBytes, err := ioutil.ReadAll(defFile)
	if err != nil {
		return nil, err
	}
	return LoadFromBytes(defBytes)
}

// LoadFromString - Loads the server definition from a string
func LoadFromString(defStr string) (*ServerDefinition, error) {
	return LoadFromBytes([]byte(defStr))
}

// LoadFromBytes - Loads the server definition from bytes
func LoadFromBytes(defBytes []byte) (*ServerDefinition, error) {
	var def ServerDefinition
	err := yaml.Unmarshal(defBytes, &def)
	if err != nil {
		return nil, err
	}
	err = def.Validate()
	return &def, err
}

// ListSteps - Traverses the steps of a stage and resolves everything to a step
func (sd *ServerDefinition) ListSteps(stageName string) ([]*Step, error) {
	return sd.resolveSteps(stageName, 0)
}

func (sd *ServerDefinition) resolveSteps(stageName string, depth int) ([]*Step, error) {
	if depth > maxDepth {
		// FIXME. we should detect circular dependencies
		return nil, fmt.Errorf("Maximum stage recursion reached. You may have circular stage dependencies")
	}

	stage, ok := sd.Stages[stageName]
	if !ok {
		return nil, fmt.Errorf("Invalid definition. Cannot find stage '%s'", stageName)
	}
	var steps []*Step
	for _, step := range stage {
		if _, ok := StepTypes[step.Type]; !ok {
			return nil, fmt.Errorf("Unknown step type: %s", step.Type)
		}
		if step.Type != "stage" {
			steps = append(steps, step)
			continue
		}

		if step.Args.Stage == "" {
			return nil, fmt.Errorf("'stage' step requires a 'stage' argument")
		}
		stageSteps, err := sd.resolveSteps(step.Args.Stage, depth+1)
		if err != nil {
			return nil, err
		}
		steps = append(steps, stageSteps...)
	}
	return steps, nil
}

// Validate - Validates a definition file
func (sd *ServerDefinition) Validate() error {
	for stageName := range sd.Stages {
		_, err := sd.ListSteps(stageName)
		if err != nil {
			return err
		}
	}
	return nil
}
