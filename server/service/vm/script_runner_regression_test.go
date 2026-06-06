//go:build script_runner_regression

package vm

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRunScriptRejectsUnsafeScriptName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	marker := filepath.Join(tmpDir, "command-injected")
	body, err := json.Marshal(map[string]string{
		"name": "missing.sh; touch " + marker,
		"type": "foreground",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/vm/script/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/vm/script/run", NewService().RunScript)
	router.ServeHTTP(w, req)

	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("unsafe script name was executed through a shell: %s", marker)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat marker: %v", err)
	}
}

func TestResolveScriptPathAllowsDoubleDotBasenames(t *testing.T) {
	tests := []string{
		"backup..sh",
		"v1..debug.py",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			filename, target, err := resolveScriptPath(name)
			if err != nil {
				t.Fatalf("resolve script path: %v", err)
			}

			if filename != name {
				t.Fatalf("filename = %q, want %q", filename, name)
			}

			expected := path.Join(ScriptDirectory, name)
			if target != expected {
				t.Fatalf("target = %q, want %q", target, expected)
			}
		})
	}
}

func TestResolveScriptPathRejectsUnsafeNames(t *testing.T) {
	tests := []string{
		"../escape.sh",
		"nested/escape.sh",
		"/tmp/escape.sh",
		"missing.sh;touch marker",
		"$(touch marker).sh",
		"`touch marker`.py",
		"not-script.txt",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			if runtime.GOOS == "windows" && strings.HasPrefix(name, "/") {
				t.Skip("absolute path syntax differs on Windows")
			}

			if _, _, err := resolveScriptPath(name); err == nil {
				t.Fatal("expected unsafe script name to be rejected")
			}
		})
	}
}

func TestScriptCommandPreservesShellScriptInterpreter(t *testing.T) {
	target := path.Join(ScriptDirectory, "bash-specific.sh")
	cmd := scriptCommand("bash-specific.sh", target)

	if cmd.Path != target {
		t.Fatalf("cmd.Path = %q, want %q", cmd.Path, target)
	}

	if len(cmd.Args) != 1 || cmd.Args[0] != target {
		t.Fatalf("cmd.Args = %#v, want only %q", cmd.Args, target)
	}
}

func TestScriptCommandRunsPythonViaInterpreter(t *testing.T) {
	target := path.Join(ScriptDirectory, "tool.py")
	cmd := scriptCommand("tool.py", target)

	if filepath.Base(cmd.Path) != "python" {
		t.Fatalf("cmd.Path = %q, want python", cmd.Path)
	}

	if len(cmd.Args) != 2 || cmd.Args[0] != "python" || cmd.Args[1] != target {
		t.Fatalf("cmd.Args = %#v, want python and target", cmd.Args)
	}
}
