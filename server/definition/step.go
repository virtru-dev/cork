package definition

// Export - used for export variable definition
type Export struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// StepArgs - Defines a step's arguments
type StepArgs struct {
	Image   string            `yaml:"image,omitempty"`
	Command string            `yaml:"command,omitempty"`
	Stage   string            `yaml:"stage,omitempty"`
	Params  map[string]string `yaml:"params,omitempty"`
	Export  Export            `yaml:"export,omitempty"`
}

// Step - Defines a step
type Step struct {
	Type      string   `yaml:"type"`
	Name      string   `yaml:"name,omitempty"`
	Args      StepArgs `yaml:"args,omitempty"`
	MatchTags []string `yaml:"match_tags,omitempty"`
	SkipTags  []string `yaml:"skip_tags,omitempty"`
	Outputs   []string `yaml:"outputs,omitempty"`
}

func (sa *StepArgs) ResolveArgs(renderer *CorkTemplateRenderer) (*StepArgs, error) {
	image, err := renderer.Render(sa.Image)
	if err != nil {
		return nil, err
	}

	command, err := renderer.Render(sa.Command)
	if err != nil {
		return nil, err
	}

	stage, err := renderer.Render(sa.Stage)
	if err != nil {
		return nil, err
	}

	exportName, err := renderer.Render(sa.Export.Name)
	if err != nil {
		return nil, err
	}

	exportValue, err := renderer.Render(sa.Export.Value)
	if err != nil {
		return nil, err
	}

	params := make(map[string]string)
	for key, value := range sa.Params {
		resolvedValue, err := renderer.Render(value)
		if err != nil {
			return nil, err
		}
		params[key] = resolvedValue
	}

	resolvedArgs := StepArgs{
		Image:   image,
		Stage:   stage,
		Command: command,
		Export: Export{
			Name:  exportName,
			Value: exportValue,
		},
		Params: params,
	}
	return &resolvedArgs, nil
}
