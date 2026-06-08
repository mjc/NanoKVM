package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreAndRunInitScriptActionRestoresBackupThenRunsAction(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "script")
	backupPath := filepath.Join(tempDir, "backup")
	outputPath := filepath.Join(tempDir, "action.txt")

	if err := os.WriteFile(scriptPath, []byte("old content\n"), 0o755); err != nil {
		t.Fatalf("write target script: %v", err)
	}
	if err := os.WriteFile(backupPath, []byte("printf '%s\\n' \"$1\" > \""+outputPath+"\"\n"), 0o755); err != nil {
		t.Fatalf("write backup script: %v", err)
	}

	if err := RestoreAndRunInitScriptAction(scriptPath, backupPath, "restart"); err != nil {
		t.Fatalf("restore and run init script: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read action output: %v", err)
	}
	if string(data) != "restart\n" {
		t.Fatalf("unexpected init script action output: %q", string(data))
	}

	scriptData, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read restored script: %v", err)
	}
	if string(scriptData) != "printf '%s\\n' \"$1\" > \""+outputPath+"\"\n" {
		t.Fatalf("expected restored script content, got %q", string(scriptData))
	}
}
