package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"time"

	"google.golang.org/grpc"

	log "github.com/sirupsen/logrus"
	pb "github.com/virtru/cork/protocol"
	"github.com/virtru/cork/server/definition"
	"github.com/virtru/cork/server/environment"
	"github.com/virtru/cork/server/executor"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"gopkg.in/urfave/cli.v1"
)

// Commands
var Commands []cli.Command

// CorkTypeServer - Runs cork type tasks
type CorkTypeServer struct {
	ServerDefinition    *definition.ServerDefinition
	CorkDir             string
	WorkDir             string
	HostWorkDir         string
	CacheDir            string
	ProjectName         string
	IsInitialized       bool
	InitializationError error
}

type ServerStream interface {
	Send(event *pb.ExecuteOutputEvent) error
}

func (c *CorkTypeServer) Kill(ctx context.Context, req *pb.KillRequest) (*pb.Response, error) {
	go func() {
		log.Debug("Asked to shutdown")
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
	res := pb.Response{
		Status: 200,
		Res: &pb.Response_Empty{
			Empty: &pb.Empty{},
		},
	}
	return &res, nil
}

func (c *CorkTypeServer) Status(ctx context.Context, req *pb.StatusRequest) (*pb.Response, error) {
	if err := c.CheckInitialization(); err != nil {
		return nil, err
	}
	log.Debug("Got a status request")
	res := pb.Response{
		Status: 200,
		Res: &pb.Response_Empty{
			Empty: &pb.Empty{},
		},
	}
	return &res, nil
}

func (c *CorkTypeServer) StageExecute(stream pb.CorkTypeService_StageExecuteServer) error {
	if err := c.CheckInitialization(); err != nil {
		return err
	}
	inputEvent, err := stream.Recv()
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("Fatal error. Never started execution")
		}
		return err
	}
	if inputEvent.GetType() != "stageExecuteRequest" {
		return fmt.Errorf("Fatal error. Expected stage execution request before anything else")
	}
	stageExecuteRequest := inputEvent.GetBody().(*pb.ExecuteInputEvent_StageExecuteRequest)
	stage := stageExecuteRequest.StageExecuteRequest.GetStage()

	requiredParams, err := c.ServerDefinition.RequiredUserParamsForStage(stage)
	if err != nil {
		return err
	}
	paramDefinitions := make(map[string]*pb.ParamDefinition)
	for _, paramName := range requiredParams {
		variableDefinition, ok := c.ServerDefinition.Params[paramName]
		if !ok {
			return fmt.Errorf(`Fatal error. Param definition could not be found for "%s"`, paramName)
		}

		varDefault := ""
		if variableDefinition.Default != nil {
			varDefault = *variableDefinition.Default
		}

		paramDefinitions[paramName] = &pb.ParamDefinition{
			Type:        variableDefinition.Type,
			Default:     varDefault,
			HasDefault:  variableDefinition.HasDefault(),
			Description: variableDefinition.Description,
		}
	}

	stream.Send(&pb.ExecuteOutputEvent{
		Type: "paramsRequest",
		Body: &pb.ExecuteOutputEvent_ParamsRequest{
			ParamsRequest: &pb.ParamsRequestEvent{
				ParamDefinitions: paramDefinitions,
			},
		},
	})

	inputEvent, err = stream.Recv()
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("Fatal error. Never started execution")
		}
		return err
	}
	if inputEvent.GetType() != "paramsResponse" {
		return fmt.Errorf("Fatal error. Expected paramsResponse request before executing the stage")
	}

	paramsResponseEvent := inputEvent.GetParamsResponse()
	params := paramsResponseEvent.GetParams()

	steps, err := c.ServerDefinition.ListSteps(stage)
	log.Debugf("Executing stage: %s with %d steps", stage, len(steps))
	if err != nil {
		return err
	}

	renderer := c.createTemplateRenderer(params)

	stageExec := executor.NewExecutor(c.CorkDir, renderer, stream, steps)
	err = stageExec.Execute()
	if err != nil {
		log.Debugf("Error occurred executing stage")
		return err
	}
	return nil
}

func (c *CorkTypeServer) createTemplateRenderer(params map[string]string) *definition.CorkTemplateRenderer {
	return definition.NewTemplateRendererWithOptions(definition.CorkTemplateRendererOptions{
		WorkDir:     c.WorkDir,
		HostWorkDir: c.HostWorkDir,
		CacheDir:    c.CacheDir,
		UserParams:  params,
	})
}

func (c *CorkTypeServer) EventReact(ctx context.Context, req *pb.EventReactRequest) (*pb.Response, error) {
	return nil, nil
}

func (c *CorkTypeServer) Initialize() {
	log.Debug("Initializing cork-server")

	startupHookPath := path.Join(c.CorkDir, "hooks/startup")
	err := executor.CheckCommandPath("startup hook", startupHookPath, 0)
	if err != nil {
		c.IsInitialized = true
		log.Debugf("No valid startup hook found: %+v", err)
		return
	}

	cmd := exec.Command(startupHookPath)
	cmd.Env = os.Environ()

	log.Debug("Executing startup hook")
	err = cmd.Run()
	if err != nil {
		c.IsInitialized = false
		c.InitializationError = err
		log.Debugf("Error executing startup hook: %+v", err)
		return
	}
	c.IsInitialized = true
	log.Debug("Executed startup hook successfully")
}

func (c *CorkTypeServer) CheckInitialization() error {
	if !c.IsInitialized {
		errorStr := ""
		if c.InitializationError != nil {
			errorStr = fmt.Sprintf(": %+v", c.InitializationError)
		}
		return grpc.Errorf(codes.Internal, "InitializationError: An error occured initializing the cork-server%s", errorStr)
	}
	return nil
}

/*
func (c *CorkTypeServer) StepExecuteAll(req *pb.StepExecuteAllRequest, stream pb.CorkTypeService_StepExecuteAllServer) error {
	randomSize := rand.Intn(100)
	for i := 0; i < randomSize; i++ {
		current := pb.ExecuteEvent{
			Type: "output",
			Body: &pb.ExecuteEvent_Output{
				Output: &pb.OutputEvent{
					Bytes:  []byte("hello"),
					Stream: "stdout",
				},
			},
		}
		err := stream.Send(&current)
		if err != nil {
			return err
		}
	}
	final := pb.ExecuteEvent{
		Type: "end",
		Body: &pb.ExecuteEvent_Empty{
			Empty: &pb.Empty{},
		},
	}
	err := stream.Send(&final)
	if err != nil {
		return err
	}
	return nil
}
*/

func newServer(c *cli.Context) (*CorkTypeServer, error) {
	corkDir := c.String("dir")
	server := CorkTypeServer{
		CorkDir:     corkDir,
		WorkDir:     c.String("work-dir"),
		HostWorkDir: c.String("host-work-dir"),
		CacheDir:    c.String("cache-dir"),
		ProjectName: c.String("project"),
	}
	serverDef, err := definition.LoadFromDir(corkDir)
	if err != nil {
		return nil, err
	}
	server.ServerDefinition = serverDef
	return &server, nil
}

func setupApp() *cli.App {
	app := cli.NewApp()

	app.Usage = "<command> [subcommand] [options...] [args...]"
	app.Description = "cork - A container workflow tool"

	cli.HelpFlag = cli.BoolFlag{
		Name:  "help, h",
		Usage: "show help",
	}

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Set debug",
		},
	}

	app.Commands = Commands
	return app
}

func registerCommand(cmd cli.Command) {
	Commands = append(Commands, cmd)
}

func main() {
	app := setupApp()
	app.Name = "cork-server"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "Set debug",
			EnvVar: "CORK_DEBUG",
		},
		cli.BoolFlag{
			Name:  "load-env-from-file, e",
			Usage: "Load environment and args from a file",
		},
		cli.StringFlag{
			Name:  "load-env-path",
			Usage: "Load environment and args from a file",
			Value: "/cork.env.json",
		},
	}
	app.Before = func(c *cli.Context) error {
		if c.Bool("debug") {
			log.SetLevel(log.DebugLevel)
			log.Debug("Cork-server debug is on")
			os.Setenv("CORK_DEBUG", "true")
			err := os.Setenv("CORK_DEBUG", "true")
			if err != nil {
				return err
			}
		} else {
			log.SetLevel(log.InfoLevel)
		}
		if c.Bool("load-env-from-file") {
			env, err := environment.LoadEnvFile(c.String("load-env-path"))
			if err != nil {
				return err
			}
			for key, value := range env {
				err = os.Setenv(key, value)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	app.Run(os.Args)
}

func init() {
	registerCommand(cli.Command{
		Name:        "serve",
		Description: "Starts the cork server",
		Action:      cmdServe,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "dir, d",
				Usage:  "The path to the Cork Directory",
				Value:  "/cork",
				EnvVar: "CORK_DIR",
			},
			cli.StringFlag{
				Name:   "work-dir",
				Usage:  "The path of the working directory from the container's perspective for a project",
				EnvVar: "CORK_WORK_DIR",
			},
			cli.StringFlag{
				Name:   "host-work-dir",
				Usage:  "The path of the working directory from the host's perspective for a project",
				EnvVar: "CORK_HOST_WORK_DIR",
			},
			cli.StringFlag{
				Name:   "cache-dir",
				Usage:  "The path of the cache the host's perspective for a project",
				EnvVar: "CORK_CACHE_DIR",
			},
			cli.IntFlag{
				Name:   "port, p",
				Usage:  "The port for the cork container server",
				EnvVar: "CORK_PORT",
				Value:  11900,
			},
			cli.StringFlag{
				Name:   "project",
				Usage:  "The name of the project",
				EnvVar: "CORK_PROJECT_NAME",
			},
		},
	})
}

func cmdServe(c *cli.Context) error {
	port := c.Int("port")

	log.Debug("The following important options are set:")
	log.Debugf("CorkDir=%s", c.String("dir"))
	log.Debugf("WorkDir=%s", c.String("work-dir"))
	log.Debugf("HostWorkDir=%s", c.String("host-work-dir"))
	log.Debugf("CacheDir=%s", c.String("cache-dir"))
	log.Debugf("ProjectName=%s", c.String("project"))

	// Run startup hooks

	server, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	corkTypeServer, err := newServer(c)
	if err != nil {
		return err
	}
	corkTypeServer.Initialize()

	log.Debugf("Starting cork-server at %d", port)
	grpcServer := grpc.NewServer()
	pb.RegisterCorkTypeServiceServer(grpcServer, corkTypeServer)
	grpcServer.Serve(server)
	return nil
}
