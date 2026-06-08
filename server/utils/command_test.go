package utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunPassesArgumentsToCommand(t *testing.T) {
	t.Setenv("GO_WANT_COMMAND_HELPER_PROCESS", "1")

	outputPath := filepath.Join(t.TempDir(), "args.txt")
	err := Run(os.Args[0], "-test.run=TestCommandHelperProcess", "--", "write-args", outputPath, "alpha", "beta gamma")
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read helper output: %v", err)
	}
	if string(data) != "alpha\nbeta gamma" {
		t.Fatalf("unexpected forwarded args: %q", string(data))
	}
}

func TestRunOutputReturnsCombinedOutput(t *testing.T) {
	t.Setenv("GO_WANT_COMMAND_HELPER_PROCESS", "1")

	output, err := RunOutput(os.Args[0], "-test.run=TestCommandHelperProcess", "--", "emit", "stdout line\n", "stderr line\n", "7")
	if err == nil {
		t.Fatalf("expected command failure")
	}
	text := string(output)
	if !strings.Contains(text, "stdout line") {
		t.Fatalf("expected stdout in combined output, got %q", text)
	}
	if !strings.Contains(text, "stderr line") {
		t.Fatalf("expected stderr in combined output, got %q", text)
	}
}

func TestRunOutputContextHonorsCancellation(t *testing.T) {
	t.Setenv("GO_WANT_COMMAND_HELPER_PROCESS", "1")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := RunOutputContext(ctx, os.Args[0], "-test.run=TestCommandHelperProcess", "--", "sleep", "200")
	if err == nil {
		t.Fatalf("expected context cancellation error")
	}
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded context, got %v", ctx.Err())
	}
}

func TestRunSequenceStopsAfterFailure(t *testing.T) {
	t.Setenv("GO_WANT_COMMAND_HELPER_PROCESS", "1")

	tracePath := filepath.Join(t.TempDir(), "trace.txt")
	err := RunSequence([]CommandSpec{
		{Name: os.Args[0], Args: []string{"-test.run=TestCommandHelperProcess", "--", "append", tracePath, "first", "0"}},
		{Name: os.Args[0], Args: []string{"-test.run=TestCommandHelperProcess", "--", "append", tracePath, "second", "9"}},
		{Name: os.Args[0], Args: []string{"-test.run=TestCommandHelperProcess", "--", "append", tracePath, "third", "0"}},
	})
	if err == nil {
		t.Fatalf("expected sequence failure")
	}

	data, readErr := os.ReadFile(tracePath)
	if readErr != nil {
		t.Fatalf("read trace output: %v", readErr)
	}
	if string(data) != "first\nsecond\n" {
		t.Fatalf("unexpected trace after failure: %q", string(data))
	}
}

func TestCommandHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_COMMAND_HELPER_PROCESS") != "1" {
		return
	}

	args := helperCommandArgs(t)
	switch args[0] {
	case "write-args":
		if err := os.WriteFile(args[1], []byte(strings.Join(args[2:], "\n")), 0o644); err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	case "emit":
		fmt.Fprint(os.Stdout, args[1])
		fmt.Fprint(os.Stderr, args[2])
		exitCode, err := strconv.Atoi(args[3])
		if err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(exitCode)
	case "sleep":
		delayMs, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		os.Exit(0)
	case "append":
		filePath := args[1]
		entry := args[2]
		exitCode, err := strconv.Atoi(args[3])
		if err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
		if _, err := fmt.Fprintln(f, entry); err != nil {
			_ = f.Close()
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
		if err := f.Close(); err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(exitCode)
	default:
		fmt.Fprintf(os.Stderr, "unknown helper command: %s", args[0])
		os.Exit(1)
	}
}

func helperCommandArgs(t *testing.T) []string {
	t.Helper()

	for i, arg := range os.Args {
		if arg == "--" {
			return os.Args[i+1:]
		}
	}
	t.Fatalf("missing helper separator in args: %#v", os.Args)
	return nil
}
