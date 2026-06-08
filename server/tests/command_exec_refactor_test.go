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
	files := []string{
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
	}

	for _, rel := range files {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			content := readServerFile(t, rel)
			if shellFormPattern.MatchString(content) {
				t.Fatalf("%s still contains shell-form exec.Command(\"sh\", \"-c\", ...)", rel)
			}
		})
	}
}

func TestRefactorUsesArgvStyleInKeyPaths(t *testing.T) {
	expectations := map[string][]string{
		"service/application/update.go":          {`utils.RestartNanoKVM()`},
		"service/application/update_offline.go":  {`utils.RestartNanoKVM()`},
		"service/vm/tls.go":                      {`utils.RestartNanoKVM()`},
		"service/vm/ssh.go":                      {`exec.Command(SSHScript, "permanent_on")`, `exec.Command(SSHScript, "permanent_off")`},
		"service/network/wifi.go":                {`exec.Command(WiFiScript, "stop")`},
		"service/extensions/tailscale/cli.go":    {`runCommandSpecs(initScriptCommandSpecs("start"))`, `tailscaleCommandSpec("status", "--json")`},
		"service/picoclaw/runtime_start_stop.go": {`runPicoclawScript(ctx, scriptPath, "start")`, `exec.CommandContext(ctx, scriptPath, action).CombinedOutput()`},
		"service/storage/image.go":               {`resetUSBGadgetUDC()`, `os.ReadDir("/sys/class/udc")`},
		"service/vm/mdns.go":                     {`exec.Command("cp", "-f", AvahiDaemonBackupScript, AvahiDaemonScript)`, `exec.Command("kill", "-9", validPID)`},
		"service/vm/swap.go":                     {`runCommandSpecs(commands, 300*time.Millisecond)`, `runCommandSpecs([]commandSpec{{name: "swapoff", args: []string{"-a"}}}, 0)`},
		"service/vm/virtual-device.go":           {`runCommandSpecs(commands, 0)`},
		"service/hid/status.go":                  {`exec.Command(USBDevScript, "restart_phy").Run()`},
	}

	for rel, snippets := range expectations {
		rel := rel
		snippets := snippets
		t.Run(rel, func(t *testing.T) {
			content := readServerFile(t, rel)
			for _, snippet := range snippets {
				if !strings.Contains(content, snippet) {
					t.Fatalf("%s is missing expected argv-style snippet: %s", rel, snippet)
				}
			}
		})
	}
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
