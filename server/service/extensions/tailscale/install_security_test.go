package tailscale

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateTailscaleDownloadURLRejectsUnexpectedSources(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"http://pkgs.tailscale.com/stable/tailscale.tgz",
		"https://example.com/tailscale.tgz",
		"https://127.0.0.1/tailscale.tgz",
	} {
		t.Run(raw, func(t *testing.T) {
			if err := validateTailscaleDownloadURL(raw); err == nil {
				t.Fatalf("validateTailscaleDownloadURL(%q) succeeded", raw)
			}
		})
	}
}

func TestValidateTailscaleDownloadURLAllowsTailscaleHTTPS(t *testing.T) {
	t.Parallel()

	if err := validateTailscaleDownloadURL(OriginalURL); err != nil {
		t.Fatalf("validateTailscaleDownloadURL returned error: %v", err)
	}
}

func TestWorkspacePathRejectsEscapes(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../tailscale.tgz", "/tmp/tailscale.tgz", "nested/tailscale.tgz", "tailscale..tgz"} {
		t.Run(name, func(t *testing.T) {
			if _, err := workspacePath(name); err == nil {
				t.Fatalf("workspacePath(%q) succeeded", name)
			}
		})
	}
}

func TestDownloadToFileCreatesOwnerOnlyArtifact(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("tailscale"))
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "tailscale.tgz")
	if err := downloadToFile(server.URL, target, maxTailscaleDownloadBytes); err != nil {
		t.Fatalf("downloadToFile returned error: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat artifact: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("artifact mode = %v, want 0600", got)
	}
}

func TestDownloadToFileRejectsOversizedArtifact(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("too-large"))
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "tailscale.tgz")
	if err := downloadToFile(server.URL, target, 3); err == nil {
		t.Fatal("downloadToFile succeeded for oversized artifact")
	}
}
