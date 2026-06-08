package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var shellFormPattern = regexp.MustCompile(`exec\.Command(?:Context)?\("sh",\s*"-c"`)

func TestRefactoredServiceFilesAvoidShellForm(t *testing.T) {
	for _, rel := range []string{
		"service/application/update.go",
		"service/application/update_offline.go",
		"service/extensions/tailscale/cli.go",
		"service/hid/status.go",
		"service/network/wifi.go",
		"service/picoclaw/runtime_start_stop.go",
		"service/storage/image.go",
		"service/vm/mdns.go",
		"service/vm/ssh.go",
		"service/vm/swap.go",
		"service/vm/tls.go",
		"service/vm/virtual-device.go",
	} {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			content := readServerFile(t, rel)
			if shellFormPattern.MatchString(content) {
				t.Fatalf("%s still contains shell-form exec.Command(\"sh\", \"-c\", ...)", rel)
			}
		})
	}
}

func TestRefactorKeySignalsRemain(t *testing.T) {
	assertContainsAll(t, "service/application/update.go", `utils.RestartNanoKVM()`)
	assertContainsAll(t, "service/application/update_offline.go", `utils.RestartNanoKVM()`)
	assertContainsAll(t, "service/vm/tls.go", `utils.RestartNanoKVM()`)
	assertContainsAll(t, "service/extensions/tailscale/cli.go", `runInitScriptAction("start")`, `runTailscaleOutput("status", "--json")`)
	assertContainsAll(t, "service/vm/swap.go", `runCommandSpecs(commands, 300*time.Millisecond)`)
	assertContainsAll(t, "service/vm/virtual-device.go", `runCommandSpecs(commands, 0)`)
	assertContainsAll(t, "service/storage/image.go", `resetUSBGadgetUDC()`)
	assertContainsAll(t, "service/picoclaw/runtime_start_stop.go", `runPicoclawScript(ctx, scriptPath, "start")`)
}

func TestScriptRunnerBranchBoundaryRemains(t *testing.T) {
	content := readServerFile(t, "service/vm/script.go")
	if !strings.Contains(content, `exec.Command("sh", "-c", command)`) {
		t.Fatalf("service/vm/script.go no longer has shell-form runner; this cleanup branch should not absorb script-runner branch changes")
	}
}

func readServerFile(t *testing.T, rel string) string {
	t.Helper()
	serverRoot := findServerRoot(t)
	absPath := filepath.Join(serverRoot, filepath.FromSlash(rel))
	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func assertContainsAll(t *testing.T, rel string, snippets ...string) {
	t.Helper()
	content := readServerFile(t, rel)
	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("%s is missing expected snippet: %s", rel, snippet)
		}
	}
}

func findServerRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod from %s", wd)
		}
		dir = parent
	}
}
