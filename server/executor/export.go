package executor

import (
	log "github.com/sirupsen/logrus"
	pb "github.com/virtru/cork/protocol"
	"github.com/virtru/cork/server/definition"
	"github.com/virtru/cork/server/streamer"
)

func init() {
	RegisterHandler("export", ExportStepHandler)
	RegisterRunner("export", ExportStepRunnerFactory)
}

type ExportStepRunner struct {
	Params StepRunnerParams
}

func ExportStepRunnerFactory(params StepRunnerParams) (StepRunner, error) {
	runner := &ExportStepRunner{}

	err := runner.Initialize(params)
	if err != nil {
		return nil, err
	}
	return runner, nil
}

func (e *ExportStepRunner) Initialize(params StepRunnerParams) error {
	e.Params = params
	return nil
}

func (e *ExportStepRunner) Run() {
	export := e.Params.Args.Export

	fmt.Printf("E PARAMS: %+v\n", *e.Params.Args)

	exportEvent := pb.ExecuteOutputEvent{
		Type: "export",
		Body: &pb.ExecuteOutputEvent_Export{
			Export: &pb.ExportEvent{
				Name:  export.Name,
				Value: export.Value,
			},
		},
	}

	e.Params.Stream.Send(&exportEvent)

	e.Params.DoneChan <- true
	return
}

func (e *ExportStepRunner) HandleInput(bytes []byte) error {
	return nil
}

func (e *ExportStepRunner) HandleSignal(signal int32) error {
	return nil
}

// ExportStepHandler - Handles exporting variables from a stage execution
func ExportStepHandler(corkDir string, executor *StepsExecutor, stream streamer.StepStream, step *definition.Step) (map[string]string, error) {
	log.Debugf("Running export step %s", step.Name)
	args, err := step.Args.ResolveArgs(executor.Renderer)
	if err != nil {
		log.Debugf("Error resolving arguments: %v", err)
		return nil, err
	}

	err = stream.Send(&pb.ExecuteOutputEvent{
		Type: "export",
		Body: &pb.ExecuteOutputEvent_Export{
			Export: &pb.ExportEvent{
				Name:  args.Export.Name,
				Value: args.Export.Value,
			},
		},
	})

	if err != nil {
		return nil, err
	}

	return nil, nil
}
