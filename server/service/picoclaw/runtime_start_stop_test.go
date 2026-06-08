package picoclaw

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsProcessRunningMatchesCommName(t *testing.T) {
	procRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(procRoot, "123"), 0o755); err != nil {
		t.Fatalf("mkdir pid dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(procRoot, "123", "comm"), []byte("picoclaw\n"), 0o644); err != nil {
		t.Fatalf("write comm file: %v", err)
	}

	running, err := isProcessRunning(procRoot, "picoclaw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !running {
		t.Fatalf("expected process to be detected")
	}
}

func TestIsProcessRunningIgnoresNonMatchingEntries(t *testing.T) {
	procRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(procRoot, "not-a-pid"), 0o755); err != nil {
		t.Fatalf("mkdir non-pid dir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(procRoot, "456"), 0o755); err != nil {
		t.Fatalf("mkdir pid dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(procRoot, "456", "comm"), []byte("something-else\n"), 0o644); err != nil {
		t.Fatalf("write comm file: %v", err)
	}

	running, err := isProcessRunning(procRoot, "picoclaw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Fatalf("expected process to be absent")
	}
}
