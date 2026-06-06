//go:build script_runner_regression

package vm

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

type scriptResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func TestRunScriptRejectsUnsafeScriptName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setScriptRunnerPasswordUpdatedForTest(t, true)

	marker := filepath.Join(t.TempDir(), "command-injected")
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

func TestScriptPathResolution(t *testing.T) {
	cases := []struct {
		label      string
		resolve    func(string) (string, string, error)
		allowed    []string
		disallowed []string
	}{
		{
			label:   "strict",
			resolve: resolveScriptPath,
			allowed: []string{"safe.sh", "SAFE.PY", "script-name_1.2.sh", "backup..sh"},
			disallowed: []string{
				"../escape.sh", "nested/escape.sh", "/tmp/escape.sh", "./escape.sh", " safe.sh",
				"safe.sh ", "safe\x00.sh", "missing.sh;touch marker", "$(touch marker).sh",
				"`touch marker`.py", "pipe|name.sh", "ampersand&name.sh", "space name.sh",
				"unicode-é.sh", "not-script.txt",
			},
		},
		{
			label:      "delete",
			resolve:    resolveScriptDeletePath,
			allowed:    []string{"my script.sh", "unicode-é.py", "old.backup..sh"},
			disallowed: []string{"", "../escape.sh", "nested/escape.sh", "/tmp/escape.sh", "safe\x00.sh", "not-script.txt"},
		},
	}

	for _, tc := range cases {
		for _, name := range tc.allowed {
			assertResolved(t, tc.label+" allows "+name, tc.resolve, name)
		}
		for _, name := range tc.disallowed {
			assertRejected(t, tc.label+" rejects "+name, tc.resolve, name)
		}
	}
}

func TestGetScriptFiles(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	nested := filepath.Join(root, "nested")

	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	writeFile(t, filepath.Join(root, "safe.sh"), "#!/bin/sh\n", 0o600)
	writeFile(t, filepath.Join(root, "notes.txt"), "notes\n", 0o600)
	writeFile(t, filepath.Join(nested, "nested.sh"), "#!/bin/sh\n", 0o600)
	if err := os.Symlink(filepath.Join(root, "safe.sh"), filepath.Join(root, "linked.sh")); err != nil {
		t.Fatalf("create script symlink: %v", err)
	}

	assertScriptFiles(t, missing)
	assertScriptFiles(t, root, "safe.sh")
}

func TestScriptCommandAndExecution(t *testing.T) {
	for _, tc := range []struct {
		name       string
		target     string
		wantBinary string
		wantArgs   int
	}{
		{"bash-specific.sh", filepath.Join(ScriptDirectory, "bash-specific.sh"), "bash-specific.sh", 1},
		{"tool.py", filepath.Join(ScriptDirectory, "tool.py"), "python", 2},
		{"tool.PY", filepath.Join(ScriptDirectory, "tool.PY"), "python", 2},
	} {
		cmd := scriptCommand(tc.name, tc.target)
		if filepath.Base(cmd.Path) != tc.wantBinary || len(cmd.Args) != tc.wantArgs || cmd.Args[len(cmd.Args)-1] != tc.target {
			t.Fatalf("%s cmd = (%q, %#v), want %s with target", tc.name, cmd.Path, cmd.Args, tc.wantBinary)
		}
	}

	target := filepath.Join(t.TempDir(), "plain.sh")
	writeFile(t, target, "echo script-ran\n", 0o700)
	output, err := runScriptForeground("plain.sh", target)
	if err != nil {
		t.Fatalf("run no-shebang script: %v, output=%s", err, output)
	}
	if string(output) != "script-ran\n" {
		t.Fatalf("output = %q, want script-ran", output)
	}
}

func TestScriptTargetAndSizeGuards(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "outside.sh")
	writeFile(t, outside, "#!/bin/sh\necho outside\n", 0o700)

	runLink := filepath.Join(t.TempDir(), "run.sh")
	if err := os.Symlink(outside, runLink); err != nil {
		t.Fatalf("symlink run target: %v", err)
	}
	_, err := runScriptForeground("run.sh", runLink)
	if err == nil {
		t.Fatal("expected symlink run target to be rejected")
	}

	uploadLink := filepath.Join(t.TempDir(), "upload.sh")
	if err := os.Symlink(outside, uploadLink); err != nil {
		t.Fatalf("symlink upload target: %v", err)
	}
	if err := validateScriptTargetForWrite(uploadLink); err == nil {
		t.Fatal("expected upload through symlink target to be rejected")
	}

	if err := validateUploadedScriptSize(MaxScriptFileSize); err != nil {
		t.Fatalf("max sized upload should be accepted: %v", err)
	}
	if err := validateUploadedScriptSize(MaxScriptFileSize + 1); err == nil {
		t.Fatal("expected oversized script upload to be rejected")
	}
}

func TestLimitedScriptOutputTruncatesOversizedOutput(t *testing.T) {
	var output limitedScriptOutput

	n, err := output.Write(bytes.Repeat([]byte("x"), MaxScriptOutputSize+1))

	if err != nil {
		t.Fatalf("write oversized output: %v", err)
	}
	if n != MaxScriptOutputSize+1 {
		t.Fatalf("written bytes = %d, want %d", n, MaxScriptOutputSize+1)
	}
	if output.Len() != MaxScriptOutputSize {
		t.Fatalf("output length = %d, want %d", output.Len(), MaxScriptOutputSize)
	}
	if !output.truncated {
		t.Fatal("expected output to be marked truncated")
	}
}

func TestRunScriptHandlerGuards(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tt := range []struct {
		name     string
		password func() (bool, error)
		payload  map[string]string
		wantCode int
		wantMsg  string
	}{
		{
			name: "invalid run type",
			password: func() (bool, error) {
				return true, nil
			},
			payload:  map[string]string{"name": "safe.sh", "type": "later"},
			wantCode: -1,
			wantMsg:  "invalid arguments",
		},
		{
			name: "missing background script",
			password: func() (bool, error) {
				return true, nil
			},
			payload:  map[string]string{"name": "missing-background.sh", "type": "background"},
			wantCode: -2,
			wantMsg:  "run script failed",
		},
		{
			name: "default password unchanged",
			password: func() (bool, error) {
				return false, nil
			},
			payload:  map[string]string{"name": "safe.sh", "type": "foreground"},
			wantCode: -3,
			wantMsg:  "change default password first",
		},
		{
			name: "password check failure",
			password: func() (bool, error) {
				return false, errors.New("read password state")
			},
			payload:  map[string]string{"name": "safe.sh", "type": "foreground"},
			wantCode: -3,
			wantMsg:  "check password failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setScriptRunnerPasswordCheckForTest(t, tt.password)
			w := runScriptRequest(t, tt.payload)
			assertScriptResponse(t, w, tt.wantCode, tt.wantMsg)
		})
	}
}

func assertResolved(t *testing.T, label string, resolve func(string) (string, string, error), name string) {
	t.Helper()
	t.Run(label, func(t *testing.T) {
		filename, target, err := resolve(name)
		if err != nil {
			t.Fatalf("resolve %q: %v", name, err)
		}
		if filename != name {
			t.Fatalf("filename = %q, want %q", filename, name)
		}
		if target != filepath.Join(ScriptDirectory, name) {
			t.Fatalf("target = %q, want script directory target", target)
		}
	})
}

func assertRejected(t *testing.T, label string, resolve func(string) (string, string, error), name string) {
	t.Helper()
	t.Run(label, func(t *testing.T) {
		if _, _, err := resolve(name); err == nil {
			t.Fatalf("expected %q to be rejected", name)
		}
	})
}

func assertScriptFiles(t *testing.T, root string, want ...string) {
	t.Helper()
	files, err := getScriptFiles(root)
	if err != nil {
		t.Fatalf("get scripts from %s: %v", root, err)
	}
	if len(files) != len(want) || len(want) > 0 && files[0] != want[0] {
		t.Fatalf("files = %#v, want %#v", files, want)
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
	if response.Code != wantCode || response.Msg != wantMsg {
		t.Fatalf("response = (%d, %q), want (%d, %q); body=%s", response.Code, response.Msg, wantCode, wantMsg, w.Body.String())
	}
}

func setScriptRunnerPasswordUpdatedForTest(t *testing.T, updated bool) {
	t.Helper()
	setScriptRunnerPasswordCheckForTest(t, func() (bool, error) {
		return updated, nil
	})
}

func setScriptRunnerPasswordCheckForTest(t *testing.T, check func() (bool, error)) {
	t.Helper()

	previous := isPasswordUpdatedForScriptRunner
	isPasswordUpdatedForScriptRunner = check
	t.Cleanup(func() {
		isPasswordUpdatedForScriptRunner = previous
	})
}

func writeFile(t *testing.T, path string, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
