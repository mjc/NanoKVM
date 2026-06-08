package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPassesArgumentsToCommand(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "args.txt")
	scriptPath := writeScript(t, "printf '%s\\n' \"$2\" \"$3\" > \"$1\"\n")

	if err := Run(scriptPath, outputPath, "alpha", "beta gamma"); err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read script output: %v", err)
	}
	if string(data) != "alpha\nbeta gamma\n" {
		t.Fatalf("unexpected forwarded args: %q", string(data))
	}
}

func TestRunOutputFallsBackToShellForScripts(t *testing.T) {
	scriptPath := writeScript(t, "printf 'out:%s\\n' \"$1\"\nprintf 'err:%s\\n' \"$2\" >&2\n")

	output, err := RunOutput(scriptPath, "alpha", "beta")
	if err != nil {
		t.Fatalf("unexpected run output error: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "out:alpha") || !strings.Contains(text, "err:beta") {
		t.Fatalf("unexpected combined output: %q", text)
	}
}

func TestRunSequenceStopsAfterFailure(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "trace.txt")
	first := writeScript(t, "printf 'first\\n' >> \"$1\"\n")
	second := writeScript(t, "printf 'second\\n' >> \"$1\"\nexit 9\n")
	third := writeScript(t, "printf 'third\\n' >> \"$1\"\n")

	err := RunSequence([]CommandSpec{
		{Name: first, Args: []string{tracePath}},
		{Name: second, Args: []string{tracePath}},
		{Name: third, Args: []string{tracePath}},
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

func writeScript(t *testing.T, content string) string {
	t.Helper()

	scriptPath := filepath.Join(t.TempDir(), "script")
	if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return scriptPath
}
