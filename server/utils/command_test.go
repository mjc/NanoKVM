package utils

import (
	"path/filepath"
	"testing"
)

func TestCommandBuildsArgs(t *testing.T) {
	cmd := Command("echo", "a", "b")
	if filepath.Base(cmd.Path) != "echo" {
		t.Fatalf("unexpected command path: %q", cmd.Path)
	}
	if len(cmd.Args) != 3 || cmd.Args[0] != "echo" || cmd.Args[1] != "a" || cmd.Args[2] != "b" {
		t.Fatalf("unexpected command args: %#v", cmd.Args)
	}
}

func TestRunSequenceWithDelayReturnsCommandContext(t *testing.T) {
	err := RunSequenceWithDelay([]CommandSpec{
		{Name: "definitely-missing-command-nanokvm", Args: []string{"x"}},
	}, 0)
	if err == nil {
		t.Fatalf("expected missing command error")
	}
}
