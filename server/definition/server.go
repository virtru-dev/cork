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
	AddOutput(stepName string, varName string, value string)
}

// StepTypes - Defines step types
var StepTypes = map[string]bool{
	"stage":     true,
	"container": true,
	"command":   true,
	"export":    true,
}

type Param struct {
	Type        string  `yaml:"type"`
	Default     *string `yaml:"default,omitempty"`
	Description string  `yaml:"description"`
	IsSensitive bool    `yaml:"is_sensitive"`
}

func (p Param) HasDefault() bool {
	return p.Default != nil
}

// ServerDefinition - Defines a cork server
type ServerDefinition struct {
	Stages  map[string]Stage `yaml:"stages"`
	Params  map[string]Param `yaml:"params"`
	Tags    []string         `yaml:"tags"`
	Version int              `yaml:"version"`

	// Internal data
	requiredUserParamsByStage map[string][]string `yaml:"-"`
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
	def := ServerDefinition{
		requiredUserParamsByStage: make(map[string][]string),
	}
	err := yaml.Unmarshal(defBytes, &def)
	if err != nil {
		return nil, err
	}
	err = def.Validate()
	return &def, err
}

func (sd *ServerDefinition) ListStages() []string {
	stageNames := make([]string, len(sd.Stages))
	i := 0
	for name := range sd.Stages {
		stageNames[i] = name
		i++
	}
	return stageNames
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

// RequiredUserParamsForStage gathers the required user params for a specific stage
func (sd *ServerDefinition) RequiredUserParamsForStage(stageName string) ([]string, error) {
	requiredUserParams, ok := sd.requiredUserParamsByStage[stageName]

	if !ok {
		return nil, fmt.Errorf(`stage "%s" does not exist`, stageName)
	}

	return requiredUserParams, nil
}

func (sd *ServerDefinition) walkSteps(stageName string) ([]string, error) {
	steps, err := sd.resolveSteps(stageName, 0)
	if err != nil {
		return nil, err
	}

	renderer := NewTemplateRenderer()
	requiredUserParamsMap := map[string]bool{}
	availableOutputs := map[string]bool{}
	usedStepNames := map[string]bool{}

	for _, step := range steps {
		if step.Name != "" {
			_, ok := usedStepNames[step.Name]
			if ok {
				return nil, fmt.Errorf(`Invalid Definition: step names must be unique in a stage. Step "%s" is not unique in stage "%s"`, step.Name, stageName)
			}
		}

		step.Args.ResolveArgs(renderer)
		requiredVarsForStep := renderer.ListRequiredVars()

		for _, requiredVar := range requiredVarsForStep {
			switch requiredVar.Type {
			case "user":
				requiredUserParamsMap[requiredVar.Lookup] = true
			case "output":
				_, ok := availableOutputs[requiredVar.Lookup]
				if !ok {
					return nil, fmt.Errorf(`Invalid Definition: Output variable "%s" used before available to step "%s"`, requiredVar.Lookup, step.ReferenceName())
				}
			}
			renderer.ResetRequiredVarTracker()
		}

		for _, availableOutputName := range step.Outputs {
			availableOutputs[fmt.Sprintf("%s.%s", step.Name, availableOutputName)] = true
		}
	}

	var requiredUserParams []string

	for requiredUserParam := range requiredUserParamsMap {
		_, ok := sd.Params[requiredUserParam]
		if !ok {
			return nil, fmt.Errorf(`Invalid Definition: Variable "%s" is not defined. All expected variables need to have a definition.`, requiredUserParam)
		}
		requiredUserParams = append(requiredUserParams, requiredUserParam)
	}

	return requiredUserParams, nil
}

// Validate validates a definition file by running through the stages
func (sd *ServerDefinition) Validate() error {
	if sd.Version == 0 {
		return fmt.Errorf("Invalid Definition: version must be specified")
	}

	if sd.Version != 1 {
		return fmt.Errorf("Invalid Definition: only version 1 is support")
	}

	for stageName := range sd.Stages {
		requiredUserParams, err := sd.walkSteps(stageName)
		if err != nil {
			return err
		}
		sd.requiredUserParamsByStage[stageName] = requiredUserParams
	}
	return nil
}
