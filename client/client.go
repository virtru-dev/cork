package client

import (
	"fmt"
	"io"
	"time"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/net/context"

	"os"

	pb "github.com/virtru/cork/protocol"
	"google.golang.org/grpc"
)

type Client struct {
	GClient pb.CorkTypeServiceClient
	Conn    *grpc.ClientConn
}

type ParamProvider interface {
	LoadParams(paramDefinitions map[string]*pb.ParamDefinition) (map[string]string, error)
}

type StdinStreamer struct {
	Stream pb.CorkTypeService_StageExecuteClient
}

func NewStreamer(stream pb.CorkTypeService_StageExecuteClient) *StdinStreamer {
	return &StdinStreamer{
		Stream: stream,
	}
}

func (s *StdinStreamer) Write(inputBytes []byte) (int, error) {
	s.Stream.Send(&pb.ExecuteInputEvent{
		Type: "input",
		Body: &pb.ExecuteInputEvent_Input{
			Input: &pb.InputEvent{
				Bytes: inputBytes,
			},
		},
	})
	return len(inputBytes), nil
}

func New(serverAddress string) (*Client, error) {
	connection, err := grpc.Dial(serverAddress, grpc.WithInsecure(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %v", err)
	}
	gClient := pb.NewCorkTypeServiceClient(connection)
	client := Client{
		GClient: gClient,
		Conn:    connection,
	}
	return &client, nil
}

func (c *Client) Close() error {
	return c.Conn.Close()
}

func (c *Client) Kill() error {
	res, err := c.GClient.Kill(context.Background(), &pb.KillRequest{})
	if err != nil {
		return err
	}

	if res.Status != 200 {
		return fmt.Errorf("Request failed with code: %d", res.Status)
	}

	err = c.Close()
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Status() error {
	res, err := c.GClient.Status(context.Background(), &pb.StatusRequest{})
	if err != nil {
		return err
	}

	if res.Status != 200 {
		return fmt.Errorf("Request failed with code: %d", res.Status)
	}

	return nil
}

func (c *Client) StageExecute(name string, paramProvider ParamProvider) (map[string]string, error) {
	stream, err := c.GClient.StageExecute(context.Background())

	// Send initial message to start the stage
	stream.Send(&pb.ExecuteInputEvent{
		Type: "stageExecuteRequest",
		Body: &pb.ExecuteInputEvent_StageExecuteRequest{
			StageExecuteRequest: &pb.StageExecuteRequestEvent{
				Stage: name,
			},
		},
	})

	if err != nil {
		return nil, err
	}
	streamer := NewStreamer(stream)

	exports := make(map[string]string)

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			log.Debugf("End of stream from the server")
			break
		}
		if event != nil {
			log.Debugf("Receieved event type: %s", event.Type)
			if event.Type == "end" {
				break
			}
			switch event.Type {
			case "output":
				switch body := event.GetBody().(type) {
				case *pb.ExecuteOutputEvent_Output:
					_, err = os.Stdout.Write(body.Output.Bytes)
					if err != nil {
						return nil, err
					}
				case *pb.ExecuteOutputEvent_Empty:
					log.Debug("Got empty from: %s", event.Type)
				}
			case "paramsRequest":
				paramsRequest := event.GetBody().(*pb.ExecuteOutputEvent_ParamsRequest)
				params, err := paramProvider.LoadParams(paramsRequest.ParamsRequest.ParamDefinitions)
				if err != nil {
					return nil, err
				}
				stream.Send(&pb.ExecuteInputEvent{
					Type: "paramsResponse",
					Body: &pb.ExecuteInputEvent_ParamsResponse{
						ParamsResponse: &pb.ParamsResponseEvent{
							Params: params,
						},
					},
				})

				go io.Copy(streamer, os.Stdin)
			case "error":
				errMessage := event.GetBody().(*pb.ExecuteOutputEvent_Error).Error.GetMessage()
				return nil, fmt.Errorf("%s", errMessage)
			case "export":
				switch body := event.GetBody().(type) {
				case *pb.ExecuteOutputEvent_Export:
					exportName := body.Export.Name
					exportValue := body.Export.Value
					exports[exportName] = exportValue
				default:
					return nil, fmt.Errorf("Unexpected export response")
				}
			default:
				log.Errorf(`UnknownEventType: Unknown event type "%s" from server`, event.Type)
				return nil, fmt.Errorf(`UnknownEventType: Unknown event type "%s" from server`, event.Type)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	return exports, nil
}
