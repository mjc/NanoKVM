package application

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeCachePathRejectsEscapes(t *testing.T) {
	t.Parallel()

	rejected := []string{
		"../nanokvm.tar.gz",
		"/tmp/nanokvm.tar.gz",
		"nested/nanokvm.tar.gz",
		"nanokvm..tar.gz",
		"nanokvm tar.gz",
	}
	for _, name := range rejected {
		t.Run(name, func(t *testing.T) {
			if _, err := safeCachePath(name); err == nil {
				t.Fatalf("safeCachePath(%q) succeeded", name)
			}
		})
	}
}

func TestSafeCachePathKeepsArtifactInsideCache(t *testing.T) {
	t.Parallel()

	got, err := safeCachePath("nanokvm_2.4.2.tar.gz")
	if err != nil {
		t.Fatalf("safeCachePath returned error: %v", err)
	}
	if !strings.HasPrefix(got, filepath.Clean(CacheDir)+string(filepath.Separator)) {
		t.Fatalf("path %q is not inside cache dir %q", got, CacheDir)
	}
}

func TestValidateUpdateURLRejectsLocalOrPlainHTTP(t *testing.T) {
	t.Parallel()

	rejected := []string{
		"http://cdn.sipeed.com/nanokvm/update.tar.gz",
		"https://127.0.0.1/update.tar.gz",
		"https://[::1]/update.tar.gz",
		"https://192.168.1.10/update.tar.gz",
		"https://169.254.169.254/latest/meta-data",
	}
	for _, raw := range rejected {
		t.Run(raw, func(t *testing.T) {
			if err := validateUpdateURL(raw); err == nil {
				t.Fatalf("validateUpdateURL(%q) succeeded", raw)
			}
		})
	}
}

func TestValidateUpdateURLAllowsHTTPSPublicHost(t *testing.T) {
	t.Parallel()

	if err := validateUpdateURL("https://cdn.sipeed.com/nanokvm/update.tar.gz"); err != nil {
		t.Fatalf("validateUpdateURL returned error: %v", err)
	}
}

func TestCopyUploadedFileCreatesOwnerOnlyArtifact(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "nanokvm.tar.gz")
	if err := copyUploadedFile(strings.NewReader("update"), target, int64(len("update")), 32); err != nil {
		t.Fatalf("copyUploadedFile returned error: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat uploaded artifact: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("uploaded artifact mode = %v, want 0600", got)
	}
}

func TestCopyUploadedFileRejectsOversizedArtifact(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "nanokvm.tar.gz")
	if err := copyUploadedFile(strings.NewReader("too-large"), target, int64(len("too-large")), 3); err == nil {
		t.Fatal("copyUploadedFile succeeded for oversized artifact")
	}
}

func TestChecksumErrorDoesNotExposeActualDigest(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "nanokvm.tar.gz")
	if err := os.WriteFile(target, []byte("artifact"), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	actual := sha512.Sum512([]byte("artifact"))
	actualDigest := base64.StdEncoding.EncodeToString(actual[:])

	err := checksum(target, "wrong")
	if err == nil {
		t.Fatal("checksum succeeded with wrong digest")
	}
	if strings.Contains(err.Error(), actualDigest) {
		t.Fatalf("checksum error leaked actual digest: %v", err)
	}
}

func makeTarGz(t *testing.T, entries map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, body := range entries {
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
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}
