package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"syscall"

	"github.com/virtru/cork/server/definition"
	"github.com/virtru/cork/server/streamer"
)

// StepHandler - Handles steps
type StepHandler func(corkDir string, executor *StepsExecutor, stream streamer.StepStream, step *definition.Step) (map[string]string, error)

var StepHandlers map[string]StepHandler

func RegisterHandler(handlerName string, handler StepHandler) {
	if StepHandlers == nil {
		StepHandlers = make(map[string]StepHandler)
	}
	StepHandlers[handlerName] = handler
}

type Step struct {
	Name string
	Path string
}

type StepDoesNotExist struct {
	Name    string
	Message string
}

type StepInvalid struct {
	Name    string
	Message string
}

func (si StepInvalid) Error() string {
	return si.Message
}

func (sdne StepDoesNotExist) Error() string {
	return sdne.Message
}

func createStepDoesNotExistError(name string) StepDoesNotExist {
	return StepDoesNotExist{
		Name:    name,
		Message: fmt.Sprintf("Step %s does not exist", name),
	}
}

func createStepInvalidError(name string, message string) StepInvalid {
	return StepInvalid{
		Name:    name,
		Message: message,
	}
}

func IsStepDoesNotExist(err error) bool {
	switch err.(type) {
	case StepDoesNotExist:
		return true
	default:
		return false
	}
}

func checkStepPath(name string, stepPath string, depth int) error {
	if depth > 10 {
		return createStepInvalidError(
			name,
			fmt.Sprintf("Invalid step '%s'. Max symlink depth reached", name),
		)
	}
	stat, err := os.Stat(stepPath)
	if err != nil {
		if os.IsNotExist(err) {
			return createStepDoesNotExistError(name)
		}
		return createStepInvalidError(
			name,
			fmt.Sprintf("Invalid step '%s'. Got err: %v", name, err),
		)
	}
	mode := stat.Mode()
	if mode&os.ModeSymlink != 0 {
		linkPath, err := os.Readlink(stepPath)
		if err != nil {
			return createStepInvalidError(
				name,
				fmt.Sprintf("Invalid step '%s'. Symlink encountered error: %v", name, err),
			)
		}
		return checkStepPath(name, linkPath, depth+1)
	}
	if !(mode.IsRegular()) {
		return createStepInvalidError(
			name,
			fmt.Sprintf("Invalid step '%s'. Step is not executable", name),
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
		return createStepInvalidError(
			name,
			fmt.Sprintf("Invalid step '%s'. Step is not owned by the cork server's uid %d or gid %d", name, corkUID, corkGID),
		)
	}

	if mode&0110 == 0 {
		return createStepInvalidError(
			name,
			fmt.Sprintf("Invalid step '%s'. Step is not executable", name),
		)
	}
	return nil
}

func Load(name string) (*Step, error) {
	corkDir := os.Getenv("CORK_DIR")
	if corkDir == "" {
		corkDir = "/cork"
	}

	stepPath := path.Join(corkDir, "steps", name)

	err := checkStepPath(name, stepPath, 0)
	if err != nil {
		return nil, err
	}

	return &Step{
		Name: name,
		Path: stepPath,
	}, nil
}

func (s *Step) Command() *exec.Cmd {
	cmd := exec.Command(s.Path)
	cmd.Env = os.Environ()
	return cmd
}
