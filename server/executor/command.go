package executor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"syscall"

	"strings"

	"io/ioutil"

	log "github.com/Sirupsen/logrus"
	"github.com/virtru/cork/server/definition"
	"github.com/virtru/cork/server/streamer"
)

func init() {
	RegisterHandler("command", CommandStepHandler)
	RegisterRunner("command", CommandStepRunnerFactory)
}

func CommandStepRunnerFactory(params StepRunnerParams) (StepRunner, error) {
	runner := &CommandStepRunner{}

	log.Debugf("Init params : %+v", params)

	err := runner.Initialize(params)
	if err != nil {
		return nil, err
	}
	return runner, nil
}

// CommandStepHandler - Handles executing a command step
func CommandStepHandler(corkDir string, executor *StepsExecutor, stream streamer.StepStream, step *definition.Step) (map[string]string, error) {
	log.Debugf("Running command step %s", step.Name)
	args, err := step.Args.ResolveArgs(executor.Renderer)
	if err != nil {
		log.Debugf("Error resolving arguments: %v", err)
		return nil, err
	}
	log.Debugf("Resolved Args: %+v", *args)

	log.Debugf("Loading command: %s", args.Command)
	command, err := LoadCommand(corkDir, args.Command)
	if err != nil {
		log.Debugf("Error loading command %s: %v", args.Command, err)
		return nil, err
	}
	log.Debugf("Executing command: %s", args.Command)
	cmd := command.ExecCommand()
	stepStreamer := streamer.New(stream)
	defer stepStreamer.Close()

	for key, value := range args.Params {
		upperKey := strings.ToUpper(key)
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", upperKey, value))
	}

	cmd.Dir = executor.Renderer.WorkDir

	outputsDir, err := ioutil.TempDir("", "cork-command-outputs-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(outputsDir)

	cmd.Env = append(cmd.Env, fmt.Sprintf("CORK_DIR=%s", corkDir))
	cmd.Env = append(cmd.Env, fmt.Sprintf("CORK_WORK_DIR=%s", executor.Renderer.WorkDir))
	cmd.Env = append(cmd.Env, fmt.Sprintf("CORK_HOST_WORK_DIR=%s", executor.Renderer.HostWorkDir))
	cmd.Env = append(cmd.Env, fmt.Sprintf("CACHE_DIR=%s", executor.Renderer.CacheDir))
	cmd.Env = append(cmd.Env, fmt.Sprintf("CORK_OUTPUTS_DIR=%s", outputsDir))

	log.Debugf("Env for %s: %v", step.Name, cmd.Env)

	err = stepStreamer.Run(cmd)
	if err != nil {
		log.Debugf("Command %s encountered an error: %v", args.Command, err)
		return nil, err
	}

	err = cmd.Wait()
	if err != nil {
		log.Debugf("Command %s encountered an error: %v", args.Command, err)
		return nil, err
	}

	// Collect any output
	return getOutputs(args.Command, outputsDir, step.Outputs)
}

func getOutputs(commandName string, outputsDir string, outputKeys []string) (map[string]string, error) {
	outputs := make(map[string]string)
	for _, key := range outputKeys {
		outputPath := path.Join(outputsDir, key)
		_, err := os.Stat(outputPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, createCommandInvalidError(
					commandName,
					fmt.Sprintf("Invalid command '%s'. Expected output value '%s' could not be found", commandName, key),
				)
			}
			return nil, err
		}
		data, err := ioutil.ReadFile(outputPath)
		if err != nil {
			return nil, err
		}
		outputs[key] = string(data)
	}
	return outputs, nil
}

type StdinPiper struct {
	InputBytesChan chan []byte
	KillInput      chan bool
}

func NewStdinPiper() *StdinPiper {
	return &StdinPiper{
		InputBytesChan: make(chan []byte),
		KillInput:      make(chan bool),
	}
}

func (s *StdinPiper) Read(p []byte) (n int, err error) {
	select {
	case newInputBytes := <-s.InputBytesChan:
		for i, b := range newInputBytes {
			p[i] = b
		}
		return len(newInputBytes), nil
	case <-s.KillInput:
		return 0, io.EOF
	}
}

func (s *StdinPiper) Kill() {
	s.KillInput <- true
}

func (s *StdinPiper) Write(bytes []byte) {
	s.InputBytesChan <- bytes
}

type Command struct {
	Name string
	Path string
}

type CommandDoesNotExist struct {
	Name    string
	Message string
}

type CommandInvalid struct {
	Name    string
	Message string
}

func (si CommandInvalid) Error() string {
	return si.Message
}

func (sdne CommandDoesNotExist) Error() string {
	return sdne.Message
}

func createCommandDoesNotExistError(name string) CommandDoesNotExist {
	return CommandDoesNotExist{
		Name:    name,
		Message: fmt.Sprintf("Command %s does not exist", name),
	}
}

func createCommandInvalidError(name string, message string) CommandInvalid {
	return CommandInvalid{
		Name:    name,
		Message: message,
	}
}

func IsCommandDoesNotExist(err error) bool {
	switch err.(type) {
	case CommandDoesNotExist:
		return true
	default:
		return false
	}
}

func CheckCommandPath(name string, commandPath string, depth int) error {
	if depth > 10 {
		return createCommandInvalidError(
			name,
			fmt.Sprintf("Invalid command '%s'. Max symlink depth reached", name),
		)
	}
	stat, err := os.Stat(commandPath)
	if err != nil {
		if os.IsNotExist(err) {
			return createCommandDoesNotExistError(name)
		}
		return createCommandInvalidError(
			name,
			fmt.Sprintf("Invalid command '%s'. Got err: %v", name, err),
		)
	}
	mode := stat.Mode()
	if mode&os.ModeSymlink != 0 {
		linkPath, err := os.Readlink(commandPath)
		if err != nil {
			return createCommandInvalidError(
				name,
				fmt.Sprintf("Invalid command '%s'. Symlink encountered error: %v", name, err),
			)
		}
		return CheckCommandPath(name, linkPath, depth+1)
	}
	if !(mode.IsRegular()) {
		return createCommandInvalidError(
			name,
			fmt.Sprintf("Invalid command '%s'. Command is not executable", name),
		)
	}
	// This file is executable by anyone
	if mode&0001 != 0 {
		return nil
	}

	statT := stat.Sys().(*syscall.Stat_t)
	uid := statT.Uid
	gid := statT.Gid
	corkUID := uint32(os.Geteuid())
	corkGID := uint32(os.Getegid())
	if uid != corkUID && gid != corkGID {
		return createCommandInvalidError(
			name,
			fmt.Sprintf("Invalid command '%s'. Command is not owned by the cork server's uid %d or gid %d", name, corkUID, corkGID),
		)
	}

	if mode&0110 == 0 {
		return createCommandInvalidError(
			name,
			fmt.Sprintf("Invalid command '%s'. Command is not executable", name),
		)
	}
	return nil
}

func LoadCommand(corkDir string, name string) (*Command, error) {
	commandPath := path.Join(corkDir, "commands", name)
	log.Debugf("Loading command %s from %s", name, commandPath)

	err := CheckCommandPath(name, commandPath, 0)
	if err != nil {
		return nil, err
	}

	return &Command{
		Name: name,
		Path: commandPath,
	}, nil
}

func (s *Command) ExecCommand() *exec.Cmd {
	cmd := exec.Command(s.Path)
	cmd.Env = os.Environ()
	return cmd
}

type CommandStepRunner struct {
	Params       StepRunnerParams
	Cmd          *exec.Cmd
	StdinPiper   *StdinPiper
	StepStreamer *streamer.StepStreamer
}

func (c *CommandStepRunner) Initialize(params StepRunnerParams) error {
	c.Params = params
	log.Debugf("Loading command: %s", c.Params.Args.Command)

	command, err := LoadCommand(c.Params.Context.CorkDir, c.Params.Args.Command)
	if err != nil {
		return err
	}

	c.Cmd = command.ExecCommand()
	return nil
}

func (c *CommandStepRunner) Run() {
	context := c.Params.Context
	log.Debugf("Executing command: %s", c.Params.Args.Command)

	stepStreamer := streamer.New(c.Params.Stream)
	defer stepStreamer.Close()

	cmd := c.Cmd

	for key, value := range c.Params.Args.Params {
		upperKey := strings.ToUpper(key)
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORK_PARAM_%s=%s", upperKey, value))

		// Support the old style until we get all existing cork servers using this
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", upperKey, value))
	}

	cmd.Dir = context.WorkDir

	cmd.Env = append(cmd.Env, fmt.Sprintf("CORK_DIR=%s", context.CorkDir))
	cmd.Env = append(cmd.Env, fmt.Sprintf("CORK_WORK_DIR=%s", context.WorkDir))
	cmd.Env = append(cmd.Env, fmt.Sprintf("CORK_HOST_WORK_DIR=%s", context.HostWorkDir))
	cmd.Env = append(cmd.Env, fmt.Sprintf("CACHE_DIR=%s", context.CacheDir))
	cmd.Env = append(cmd.Env, fmt.Sprintf("CORK_OUTPUTS_DIR=%s", context.OutputsDir))

	log.Debugf("Env for command %s: %v", c.Params.Args.Command, cmd.Env)

	stdinPiper := NewStdinPiper()

	c.StdinPiper = stdinPiper
	c.StepStreamer = stepStreamer
	cmd.Stdin = stdinPiper

	err := stepStreamer.Run(cmd)
	if err != nil {
		log.Debugf("Command %s encountered an error")
		c.Params.ErrorChan <- err
		c.Params.DoneChan <- true
		return
	}

	err = cmd.Wait()
	if err != nil {
		c.Params.ErrorChan <- err
		c.Params.DoneChan <- true
		return
	}
	c.Params.DoneChan <- true
	return
}

func (c *CommandStepRunner) HandleInput(bytes []byte) error {
	err := c.StepStreamer.Write(bytes)
	if err != nil {
		return err
	}
	return nil
}

func (c *CommandStepRunner) HandleSignal(signal int32) error {
	return nil
}
