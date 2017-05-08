package executor

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/fatih/color"
	"github.com/virtru/cork/server/definition"
	"github.com/virtru/cork/server/streamer"
)

type StepsExecutor struct {
	Renderer *definition.CorkTemplateRenderer
	CorkDir  string
	Stream   streamer.StepStream
}

func NewExecutor(corkDir string, renderer *definition.CorkTemplateRenderer, stream streamer.StepStream) *StepsExecutor {
	return &StepsExecutor{
		Renderer: renderer,
		CorkDir:  corkDir,
		Stream:   stream,
	}
}

func (se *StepsExecutor) ExecuteStep(step *definition.Step) error {
	stepName := ""
	if step.Name != "" {
		stepName = fmt.Sprintf("\"%s\"", step.Name)
	}
	color.Green("\n>>> Executing %s step %s\n", step.Type, stepName)

	// Resolve the arguments for the current step
	handler, ok := StepHandlers[step.Type]
	if !ok {
		return fmt.Errorf("Unknown step type '%s'", step.Type)
	}
	outputs, err := handler(se.CorkDir, se, se.Stream, step)
	if err != nil {
		color.Red("\n>>> Failed while executing %s step %s\n", step.Type, stepName)
		return err
	}

	log.Debugf("Step execution completed for step %s", step.Name)

	for _, key := range step.Outputs {
		log.Debugf("Retrieving output value for %s from step %s", key, step.Name)
		value, ok := outputs[key]
		if !ok {
			return fmt.Errorf("Expected output value %s missing from step %s", key, step.Name)
		}
		log.Debugf("Retrieved output %s=%s from step %s", key, value, step.Name)
		se.Renderer.AddOutput(step.Name, key, value)
	}
	return nil
}

func (se *StepsExecutor) AddOutput(stepName string, varName string, value string) {
	se.Renderer.AddOutput(stepName, varName, value)
}
