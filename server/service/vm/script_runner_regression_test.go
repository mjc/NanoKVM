//go:build script_runner_regression

package vm

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
