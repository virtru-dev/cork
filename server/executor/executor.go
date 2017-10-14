package executor

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/fatih/color"
	pb "github.com/virtru/cork/protocol"
	"github.com/virtru/cork/server/definition"
	"github.com/virtru/cork/server/streamer"
)

type StepsExecutor struct {
	Renderer       *definition.CorkTemplateRenderer
	CorkDir        string
	Stream         streamer.StepStream
	Steps          []*definition.Step
	InputChan      chan *pb.ExecuteInputEvent
	InputErrorChan chan error
	InputWait      chan bool
}

func NewExecutor(corkDir string, renderer *definition.CorkTemplateRenderer, stream streamer.StepStream, steps []*definition.Step) *StepsExecutor {
	inputChan := make(chan *pb.ExecuteInputEvent)
	inputErrorChan := make(chan error)
	inputWait := make(chan bool)
	return &StepsExecutor{
		Renderer:       renderer,
		CorkDir:        corkDir,
		Stream:         stream,
		Steps:          steps,
		InputChan:      inputChan,
		InputErrorChan: inputErrorChan,
		InputWait:      inputWait,
	}
}

func (se *StepsExecutor) receiveInput() {
	go func() {
		for {
			input, err := se.Stream.Recv()
			log.Debugf("Received input or error")
			if err != nil {
				if err != io.EOF {
					se.InputErrorChan <- err
				}
				close(se.InputWait)
				return
			}
			se.InputChan <- input
		}
	}()
}

func (se *StepsExecutor) handleInput(input *pb.ExecuteInputEvent, runner StepRunner) error {
	inputType := input.GetType()
	switch inputType {
	case "signal":
		log.Debugf("Received a signal")
	case "input":
		log.Debugf("Received input data")
		runner.HandleInput(input.GetBody().(*pb.ExecuteInputEvent_Input).Input.Bytes)
	}
	return nil
}

func (se *StepsExecutor) sendOutput() error {
	return nil
}

func (se *StepsExecutor) Execute() error {
	se.receiveInput()

	for _, step := range se.Steps {
		log.Debugf("Step: %+v", step)
		err := se.ExecuteStep(step)
		if err != nil {
			log.Debugf("Step Error: %+v", err)
			return err
		}
	}
	return nil
}

func (se *StepsExecutor) makeStepRunnerParams(doneChan chan bool, errorChan chan error, args *definition.StepArgs, outputsDir string) StepRunnerParams {
	return StepRunnerParams{
		DoneChan:  doneChan,
		ErrorChan: errorChan,
		Args:      args,
		Context: ExecContext{
			CorkDir:     se.CorkDir,
			WorkDir:     se.Renderer.WorkDir,
			HostWorkDir: se.Renderer.HostWorkDir,
			CacheDir:    se.Renderer.CacheDir,
			OutputsDir:  outputsDir,
		},
		Stream: se.Stream,
	}
}

func (se *StepsExecutor) ExecuteStep(step *definition.Step) error {
	stepName := ""
	if step.Name != "" {
		stepName = fmt.Sprintf("\"%s\"", step.Name)
	}
	color.Green("\n>>> Executing %s step %s\n", step.Type, stepName)

	doneChan := make(chan bool)
	errorChan := make(chan error)
	args, err := step.Args.ResolveArgs(se.Renderer)
	if err != nil {
		return err
	}

	outputsDir, err := ioutil.TempDir("", "cork-command-outputs-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(outputsDir)

	params := se.makeStepRunnerParams(doneChan, errorChan, args, outputsDir)

	// Resolve the arguments for the current step
	runner, err := StepRunners.GetRunner(step.Type, params)
	if err != nil {
		return err
	}

	go runner.Run()

	done := false

	for {
		select {
		case input := <-se.InputChan:
			log.Debugf("Received input")
			se.handleInput(input, runner)
		case err := <-se.InputErrorChan:
			log.Debugf("Received error from user input")
			color.Red("\n>>> Failed while executing %s step %s\n", step.Type, stepName)
			return err
		case err := <-errorChan:
			log.Debugf("Received error from executing step")
			color.Red("\n>>> Failed while executing %s step %s\n", step.Type, stepName)
			return err
		case <-doneChan:
			log.Debugf("Step execution completed for step %s", step.Name)
			done = true
			break
		}

		if done {
			break
		}
	}

	outputs, err := getOutputs(args.Command, outputsDir, step.Outputs)
	if err != nil {
		return err
	}

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
