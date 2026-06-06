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

type scriptResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func TestRunScriptRejectsUnsafeScriptName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	marker := filepath.Join(tmpDir, "command-injected")
	w := runScriptRequest(t, map[string]string{
		"name": "missing.sh; touch " + marker,
		"type": "foreground",
	})

	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("unsafe script name was executed through a shell: %s", marker)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat marker: %v", err)
	}

	assertScriptResponse(t, w, -1, "invalid arguments")
}

func TestRunScriptRejectsInvalidRunType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := runScriptRequest(t, map[string]string{
		"name": "safe.sh",
		"type": "later",
	})

	assertScriptResponse(t, w, -1, "invalid arguments")
}

func TestResolveScriptPathAllowsSafeBasenames(t *testing.T) {
	tests := []string{
		"safe.sh",
		"SAFE.PY",
		"script-name_1.2.sh",
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
		"./escape.sh",
		" safe.sh",
		"safe.sh ",
		"safe\x00.sh",
		"missing.sh;touch marker",
		"$(touch marker).sh",
		"`touch marker`.py",
		"pipe|name.sh",
		"ampersand&name.sh",
		"space name.sh",
		"unicode-é.sh",
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

func TestIsScriptIsCaseInsensitive(t *testing.T) {
	tests := map[string]bool{
		"tool.sh":  true,
		"tool.SH":  true,
		"tool.py":  true,
		"tool.PY":  true,
		"tool.txt": false,
		"tool":     false,
	}

	for name, want := range tests {
		t.Run(name, func(t *testing.T) {
			if got := isScript(name); got != want {
				t.Fatalf("isScript(%q) = %t, want %t", name, got, want)
			}
		})
	}
}

func TestEnsurePathInsideRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "safe.sh")
	outside := filepath.Join(root, "..", "escape.sh")

	if err := ensurePathInside(root, inside); err != nil {
		t.Fatalf("expected inside path to be accepted: %v", err)
	}

	if err := ensurePathInside(root, outside); err == nil {
		t.Fatal("expected escaping path to be rejected")
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

func TestScriptCommandTreatsUppercasePythonExtensionAsPython(t *testing.T) {
	target := path.Join(ScriptDirectory, "tool.PY")
	cmd := scriptCommand("tool.PY", target)

	if filepath.Base(cmd.Path) != "python" {
		t.Fatalf("cmd.Path = %q, want python", cmd.Path)
	}
}

func runScriptRequest(t *testing.T, payload map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/vm/script/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/vm/script/run", NewService().RunScript)
	router.ServeHTTP(w, req)

	return w
}

func assertScriptResponse(t *testing.T, w *httptest.ResponseRecorder, wantCode int, wantMsg string) {
	t.Helper()

	if w.Code != http.StatusOK {
		t.Fatalf("http status = %d, want %d", w.Code, http.StatusOK)
	}

	var response scriptResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if response.Code != wantCode {
		t.Fatalf("response code = %d, want %d; body=%s", response.Code, wantCode, w.Body.String())
	}

	if response.Msg != wantMsg {
		t.Fatalf("response msg = %q, want %q; body=%s", response.Msg, wantMsg, w.Body.String())
	}
}
