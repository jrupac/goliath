package utils

import "os/exec"

// CommandRunner defines an interface for running external commands, allowing for mocking in tests.
type CommandRunner interface {
	Output(command string, args ...string) ([]byte, error)
}

// ExecCommandRunner is the real implementation of CommandRunner that executes commands on the system.
type ExecCommandRunner struct{}

// Output executes the command and returns its standard output.
func (e ExecCommandRunner) Output(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).Output()
}
