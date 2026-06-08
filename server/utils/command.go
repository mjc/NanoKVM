package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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
	err := Command(name, args...).Run()
	if shouldRetryWithShell(name, err) {
		return exec.Command("sh", append([]string{name}, args...)...).Run()
	}
	return err
}

func RunOutput(name string, args ...string) ([]byte, error) {
	output, err := Command(name, args...).CombinedOutput()
	if shouldRetryWithShell(name, err) {
		return exec.Command("sh", append([]string{name}, args...)...).CombinedOutput()
	}
	return output, err
}

func RunOutputContext(ctx context.Context, name string, args ...string) ([]byte, error) {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if shouldRetryWithShell(name, err) {
		return exec.CommandContext(ctx, "sh", append([]string{name}, args...)...).CombinedOutput()
	}
	return output, err
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

func shouldRetryWithShell(name string, err error) bool {
	return err != nil &&
		errors.Is(err, syscall.ENOEXEC) &&
		(strings.ContainsRune(name, os.PathSeparator) || strings.HasSuffix(strings.ToLower(name), ".sh"))
}

func RestoreAndRunInitScriptAction(scriptPath string, backupPath string, action string) error {
	srcFile, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(scriptPath), filepath.Base(scriptPath)+".tmp-")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(info.Mode()); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, scriptPath); err != nil {
		return err
	}
	return Run(scriptPath, action)
}
