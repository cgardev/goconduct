package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/cgardev/goconduct/pkg/failure"
)

// Command describes one deterministic child-process invocation.
type Command struct {
	Path        string
	Arguments   []string
	Directory   string
	Environment []string
}

// CommandResult contains captured process output and its exit status.
type CommandResult struct {
	StandardOutput []byte
	StandardError  []byte
	ExitCode       int
}

// CommandRunner executes external quality tools.
type CommandRunner interface {
	Run(ctx context.Context, command Command) (CommandResult, error)
}

type commandRunner struct{}

var _ CommandRunner = commandRunner{}

// NewCommandRunner creates a child-process runner that does not use a command shell.
func NewCommandRunner() CommandRunner {
	return commandRunner{}
}

// Run executes one command and captures both output streams.
func (commandRunner) Run(ctx context.Context, command Command) (CommandResult, error) {
	if strings.TrimSpace(command.Path) == "" {
		return CommandResult{}, failure.Validation("command path is empty", nil)
	}
	invocation := exec.CommandContext(ctx, command.Path, slices.Clone(command.Arguments)...)
	invocation.Dir = command.Directory
	if len(command.Environment) != 0 {
		invocation.Env = append(os.Environ(), slices.Clone(command.Environment)...)
	}
	var standardOutput bytes.Buffer
	var standardError bytes.Buffer
	invocation.Stdout = &standardOutput
	invocation.Stderr = &standardError
	err := invocation.Run()
	result := CommandResult{
		StandardOutput: bytes.Clone(standardOutput.Bytes()),
		StandardError:  bytes.Clone(standardError.Bytes()),
	}
	if err == nil {
		return result, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
	} else {
		result.ExitCode = -1
	}
	return result, failure.Unavailable(fmt.Sprintf("run command %q", command.Path), err)
}
