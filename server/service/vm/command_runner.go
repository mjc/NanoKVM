package vm

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type commandSpec struct {
	name string
	args []string
}

func runCommandSpecs(commands []commandSpec, delay time.Duration) error {
	for _, command := range commands {
		if err := exec.Command(command.name, command.args...).Run(); err != nil {
			return fmt.Errorf("%s %s: %w", command.name, strings.Join(command.args, " "), err)
		}
		if delay > 0 {
			time.Sleep(delay)
		}
	}
	return nil
}
