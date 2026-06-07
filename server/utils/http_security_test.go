package utils

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadRemovesPartialArtifactOnOversize(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write([]byte("too-large"))
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "artifact.tar.gz")
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if err := downloadWithLimit(req, target, 3); err == nil {
		t.Fatal("downloadWithLimit succeeded for oversized response")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("oversized download artifact still exists: %v", err)
	}
}
