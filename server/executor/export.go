package executor

import (
	log "github.com/Sirupsen/logrus"
	pb "github.com/virtru/cork/protocol"
	"github.com/virtru/cork/server/definition"
	"github.com/virtru/cork/server/streamer"
)

func init() {
	RegisterHandler("container", ExportStepHandler)
}

// ExportStepHandler - Handles exporting variables from a stage execution
func ExportStepHandler(corkDir string, executor *StepsExecutor, stream streamer.StepStream, step *definition.Step) (map[string]string, error) {
	log.Debugf("Running export step %s", step.Name)
	args, err := step.Args.ResolveArgs(executor.Renderer)
	if err != nil {
		log.Debugf("Error resolving arguments: %v", err)
		return nil, err
	}

	err = stream.Send(&pb.ExecuteEvent{
		Type: "export",
		Body: &pb.ExecuteEvent_Export{
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
