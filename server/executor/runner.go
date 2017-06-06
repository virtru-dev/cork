package executor

import (
	"fmt"

	"github.com/virtru/cork/server/definition"
	"github.com/virtru/cork/server/streamer"
)

type StepRunnersMap map[string]StepRunnerFactory

var StepRunners StepRunnersMap

type StepRunnerFactory func(params StepRunnerParams) (StepRunner, error)

type StepRunner interface {
	Initialize(params StepRunnerParams) error
	Run()
	HandleInput(bytes []byte) error
	HandleSignal(signal int32) error
}

func (s StepRunnersMap) GetRunner(name string, params StepRunnerParams) (StepRunner, error) {
	runnerFactory, ok := s[name]
	if !ok {
		return nil, fmt.Errorf(`No runner "%s" exists`, name)
	}
	return runnerFactory(params)
}

type StepRunnerParams struct {
	Context   ExecContext
	Args      *definition.StepArgs
	Stream    streamer.StepStream
	ErrorChan chan error
	DoneChan  chan bool
}

func RegisterRunner(name string, factory StepRunnerFactory) {
	if StepRunners == nil {
		StepRunners = make(map[string]StepRunnerFactory)
	}
	StepRunners[name] = factory
}
