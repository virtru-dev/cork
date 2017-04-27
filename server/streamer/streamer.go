package streamer

import (
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/kr/pty"
	pb "github.com/virtru/cork/protocol"
	"github.com/virtru/cork/server/capture"
)

type StepStream interface {
	Send(event *pb.ExecuteEvent) error
}

type StepStreamer struct {
	Stream StepStream
}

func New(stream StepStream) *StepStreamer {
	return &StepStreamer{
		Stream: stream,
	}
}

func (c *StepStreamer) Run(cmd *exec.Cmd) error {
	pty, err := pty.Start(cmd)
	if err != nil {
		return err
	}

	stdoutCapture := capture.New(func(p []byte) error {
		current := pb.ExecuteEvent{
			Type: "output",
			Body: &pb.ExecuteEvent_Output{
				Output: &pb.OutputEvent{
					Bytes:  p,
					Stream: "stdout",
				},
			},
		}
		return c.Stream.Send(&current)
	})

	_, err = io.Copy(stdoutCapture, pty)
	if e, ok := err.(*os.PathError); ok && e.Err == syscall.EIO {
		err = nil
	}
	if err != nil {
		return err
	}
	return nil
}

func (c *StepStreamer) Close() error {
	/*
		final := pb.ExecuteEvent{
			Type: "end",
			Body: &pb.ExecuteEvent_Empty{
				Empty: &pb.Empty{},
			},
		}
		err := c.Stream.Send(&final)
		if err != nil {
			return err
		}
	*/
	return nil
}
