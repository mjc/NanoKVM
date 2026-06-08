package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFilePreservesContentAndMode(t *testing.T) {
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "script.sh")
	if err := os.WriteFile(srcPath, []byte("echo copied\n"), 0o755); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	dstPath := filepath.Join(t.TempDir(), "nested", "script.sh")
	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copy file: %v", err)
	}

	data, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(data) != "echo copied\n" {
		t.Fatalf("unexpected copied content: %q", string(data))
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("stat copied file: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("expected copied mode 0755, got %o", info.Mode().Perm())
	}
}
