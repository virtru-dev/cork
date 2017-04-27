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

func (c *Client) StageExecute(name string) error {
	req := &pb.StageExecuteRequest{
		Stage: name,
	}
	stream, err := c.GClient.StageExecute(context.Background(), req)
	if err != nil {
		return err
	}

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if event != nil {
			log.Debugf("Receieved event type: %s", event.Type)
			if event.Type == "end" {
				break
			}
			if event.Type == "output" {
				switch body := event.GetBody().(type) {
				case *pb.ExecuteEvent_Output:
					_, err = os.Stdout.Write(body.Output.Bytes)
					if err != nil {
						return err
					}
				case *pb.ExecuteEvent_Empty:
					log.Debug("Got empty from: %s", event.Type)
				}
			}
			if event.Type == "error" {
				errMessage := event.GetBody().(*pb.ExecuteEvent_Error).Error.GetMessage()
				return fmt.Errorf("%s", errMessage)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}
