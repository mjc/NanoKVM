package utils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveTargetPathRejectsEscapes(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../outside", "/tmp/outside", "dir/../../outside"} {
		t.Run(name, func(t *testing.T) {
			if _, err := archiveTargetPath(t.TempDir(), name); err == nil {
				t.Fatalf("archiveTargetPath(%q) succeeded", name)
			}
		})
	}
}

func TestUnTarGzRejectsTraversalEntries(t *testing.T) {
	t.Parallel()

	src := filepath.Join(t.TempDir(), "artifact.tar.gz")
	if err := os.WriteFile(src, tarGzWithRegularFile(t, "../outside", "owned"), 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	if _, err := UnTarGz(src, t.TempDir()); err == nil {
		t.Fatal("UnTarGz succeeded for traversal entry")
	}
}

func TestUnTarGzRejectsSymlinkEntries(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "app/config",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
		Mode:     0o777,
	}); err != nil {
		t.Fatalf("write symlink header: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	src := filepath.Join(t.TempDir(), "artifact.tar.gz")
	if err := os.WriteFile(src, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	if _, err := UnTarGz(src, t.TempDir()); err == nil {
		t.Fatal("UnTarGz succeeded for symlink entry")
	}
}

func TestDownloadCreatesOwnerOnlyArtifact(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write([]byte("artifact"))
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "artifact.tar.gz")
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if err := Download(req, target); err != nil {
		t.Fatalf("Download returned error: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat downloaded artifact: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("downloaded artifact mode = %v, want 0600", got)
	}
}

func tarGzWithRegularFile(t *testing.T, name string, body string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o644,
		Size: int64(len(body)),
	}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatalf("write tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}
