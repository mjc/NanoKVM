package vm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

type autostartResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data string `json:"data"`
}

func TestGetAutostartContentReturnsFileContents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	oldDir := autostartDirectory
	autostartDirectory = tempDir
	t.Cleanup(func() {
		autostartDirectory = oldDir
	})

	const want = "#!/bin/sh\necho hello\n"
	if err := os.WriteFile(filepath.Join(tempDir, "boot.sh"), []byte(want), 0o600); err != nil {
		t.Fatalf("write autostart file: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/vm/autostart/boot.sh", nil)
	ctx.Params = gin.Params{{Key: "name", Value: "boot.sh"}}

	NewService().GetAutostartContent(ctx)

	var rsp autostartResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &rsp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if rsp.Code != 0 {
		t.Fatalf("expected success, got code=%d msg=%q", rsp.Code, rsp.Msg)
	}
	if rsp.Data != want {
		t.Fatalf("expected full content %q, got %q", want, rsp.Data)
	}
}

func TestGetAutostartContentRejectsTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	oldDir := autostartDirectory
	autostartDirectory = tempDir
	t.Cleanup(func() {
		autostartDirectory = oldDir
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/vm/autostart/../boot.sh", nil)
	ctx.Params = gin.Params{{Key: "name", Value: "../boot.sh"}}

	NewService().GetAutostartContent(ctx)

	var rsp autostartResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &rsp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if rsp.Code != -1 {
		t.Fatalf("expected invalid arguments error, got code=%d msg=%q", rsp.Code, rsp.Msg)
	}
	if rsp.Msg != "invalid arguments" {
		t.Fatalf("expected invalid arguments message, got %q", rsp.Msg)
	}
}
