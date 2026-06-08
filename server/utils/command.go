package utils

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type CommandSpec struct {
	Name string
	Args []string
}

func Command(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func Run(name string, args ...string) error {
	return Command(name, args...).Run()
}

func RunOutput(name string, args ...string) ([]byte, error) {
	return Command(name, args...).CombinedOutput()
}

func RunContext(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

func RunOutputContext(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func RunSequence(commands []CommandSpec) error {
	return RunSequenceWithDelay(commands, 0)
}

func RunSequenceWithDelay(commands []CommandSpec, delay time.Duration) error {
	for _, command := range commands {
		if err := Run(command.Name, command.Args...); err != nil {
			return fmt.Errorf("%s %s: %w", command.Name, strings.Join(command.Args, " "), err)
		}
		if delay > 0 {
			time.Sleep(delay)
		}
	}
	return nil
}
