package utils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"testing/quick"
)

var quickConfig = &quick.Config{MaxCount: 32}

type payload []byte

func (payload) Generate(rand *rand.Rand, _ int) reflect.Value {
	size := rand.Intn(256)
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}
	return reflect.ValueOf(payload(data))
}

type safeName string

func (safeName) Generate(rand *rand.Rand, _ int) reflect.Value {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
	size := 1 + rand.Intn(32)
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return reflect.ValueOf(safeName(buf))
}

func TestDownloadProperties(t *testing.T) {
	prop := func(body payload, contentType string) bool {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", contentType)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		req, err := http.NewRequest(http.MethodGet, server.URL, nil)
		if err != nil {
			t.Fatal(err)
		}

		target := filepath.Join(t.TempDir(), "nested", "download.bin")
		if err := Download(req, target); err != nil {
			t.Logf("Download failed for %q: %v", contentType, err)
			return false
		}
		return bytes.Equal(mustReadUtilsFile(t, target), []byte(body))
	}

	for _, contentType := range []string{"application/octet-stream", "application/zip", "application/gzip"} {
		t.Run(contentType, func(t *testing.T) {
			if err := quick.Check(func(body payload) bool {
				return prop(body, contentType)
			}, quickConfig); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDownloadFailures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    string
		closeFirst bool
	}{
		{
			name: "status",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "nope", http.StatusServiceUnavailable)
			},
			wantErr: "inaccessible",
		},
		{
			name: "content-type",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write([]byte("not firmware"))
			},
			wantErr: "unsupported",
		},
		{
			name: "short-body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/gzip")
				w.Header().Set("Content-Length", "8")
				_, _ = w.Write([]byte("short"))
			},
			wantErr: "unexpected EOF",
		},
		{
			name: "request",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("closed"))
			},
			closeFirst: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			if tc.closeFirst {
				server.Close()
			} else {
				defer server.Close()
			}

			req, err := http.NewRequest(http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			assertErrContains(t, Download(req, filepath.Join(t.TempDir(), "download.bin")), tc.wantErr)
		})
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	parentFile := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertErrContains(t, Download(req, filepath.Join(parentFile, "target")), "not a directory")

	targetDir := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	assertErrContains(t, Download(req, targetDir), "is a directory")
}

func TestUnTarGzProperties(t *testing.T) {
	prop := func(name safeName, data payload) bool {
		tmp := t.TempDir()
		archive := filepath.Join(tmp, "package.tar.gz")
		fileName := string(name)
		writeTarGz(t, archive, []tarEntry{
			{name: "latest/", mode: 0o755, typeflag: tar.TypeDir},
			{name: "latest/" + fileName, body: []byte(data), mode: 0o644, typeflag: tar.TypeReg},
			{name: "latest/link", linkname: fileName, mode: 0o777, typeflag: tar.TypeSymlink},
		})

		dest := filepath.Join(tmp, "dest")
		root, err := UnTarGz(archive, dest)
		if err != nil {
			t.Logf("UnTarGz failed: %v", err)
			return false
		}
		if root != filepath.Join(dest, "latest") {
			t.Logf("root = %q", root)
			return false
		}
		if got := mustReadUtilsFile(t, filepath.Join(root, fileName)); !bytes.Equal(got, []byte(data)) {
			t.Logf("file body mismatch")
			return false
		}
		link, err := os.Readlink(filepath.Join(root, "link"))
		return err == nil && link == fileName
	}

	if err := quick.Check(prop, quickConfig); err != nil {
		t.Fatal(err)
	}
}

func TestUnTarGzFailures(t *testing.T) {
	tmp := t.TempDir()
	validArchive := filepath.Join(tmp, "package.tar.gz")
	writeTarGz(t, validArchive, []tarEntry{
		{name: "latest/", mode: 0o755, typeflag: tar.TypeDir},
		{name: "latest/version", body: []byte("1.0.0"), mode: 0o644, typeflag: tar.TypeReg},
		{name: "latest/fifo", mode: 0o644, typeflag: tar.TypeFifo},
	})
	if root, err := UnTarGz(validArchive, filepath.Join(tmp, "valid-dest")); err != nil || !strings.HasSuffix(root, "latest") {
		t.Fatalf("valid archive = %q, %v", root, err)
	}

	parentFile := filepath.Join(tmp, "not-dir")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertUntarFails(t, validArchive, filepath.Join(parentFile, "dest"), "not a directory")
	assertErrIsNotExist(t, untarErr(filepath.Join(tmp, "missing.tar.gz"), filepath.Join(tmp, "missing-dest")))

	plain := filepath.Join(tmp, "plain.tar.gz")
	if err := os.WriteFile(plain, []byte("not gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertUntarFails(t, plain, filepath.Join(tmp, "plain-dest"), "unexpected EOF")

	truncatedTar := filepath.Join(tmp, "truncated.tar.gz")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte("not a tar stream")); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(truncatedTar, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	assertUntarFails(t, truncatedTar, filepath.Join(tmp, "truncated-dest"), "unexpected EOF")

	for _, tc := range []struct {
		name    string
		entries []tarEntry
		want    string
	}{
		{
			name: "missing-parent",
			entries: []tarEntry{
				{name: "latest/missing/file", body: []byte("x"), mode: 0o644, typeflag: tar.TypeReg},
			},
			want: "no such file or directory",
		},
		{
			name: "dir-conflict",
			entries: []tarEntry{
				{name: "latest/", mode: 0o755, typeflag: tar.TypeDir},
				{name: "latest/conflict", body: []byte("file"), mode: 0o644, typeflag: tar.TypeReg},
				{name: "latest/conflict/", mode: 0o755, typeflag: tar.TypeDir},
			},
			want: "not a directory",
		},
		{
			name: "symlink-conflict",
			entries: []tarEntry{
				{name: "latest/", mode: 0o755, typeflag: tar.TypeDir},
				{name: "latest/link", body: []byte("file"), mode: 0o644, typeflag: tar.TypeReg},
				{name: "latest/link", linkname: "target", mode: 0o777, typeflag: tar.TypeSymlink},
			},
			want: "file exists",
		},
		{
			name: "parent-traversal-file",
			entries: []tarEntry{
				{name: "latest/", mode: 0o755, typeflag: tar.TypeDir},
				{name: "latest/../escape", body: []byte("x"), mode: 0o644, typeflag: tar.TypeReg},
			},
			want: "invalid tar path",
		},
		{
			name: "parent-traversal-dir",
			entries: []tarEntry{
				{name: "latest/", mode: 0o755, typeflag: tar.TypeDir},
				{name: "latest/../escape/", mode: 0o755, typeflag: tar.TypeDir},
			},
			want: "invalid tar path",
		},
		{
			name: "absolute-symlink",
			entries: []tarEntry{
				{name: "latest/", mode: 0o755, typeflag: tar.TypeDir},
				{name: "latest/link", linkname: "/etc/passwd", mode: 0o777, typeflag: tar.TypeSymlink},
			},
			want: "invalid symlink target",
		},
		{
			name: "parent-traversal-symlink",
			entries: []tarEntry{
				{name: "latest/", mode: 0o755, typeflag: tar.TypeDir},
				{name: "latest/link", linkname: "../escape", mode: 0o777, typeflag: tar.TypeSymlink},
			},
			want: "invalid symlink target",
		},
		{
			name: "wrong-root",
			entries: []tarEntry{
				{name: "current/", mode: 0o755, typeflag: tar.TypeDir},
				{name: "current/version", body: []byte("x"), mode: 0o644, typeflag: tar.TypeReg},
			},
			want: "invalid tar root",
		},
		{
			name: "sibling-root",
			entries: []tarEntry{
				{name: "latest/", mode: 0o755, typeflag: tar.TypeDir},
				{name: "sibling/", mode: 0o755, typeflag: tar.TypeDir},
			},
			want: "invalid tar root",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			archive := filepath.Join(tmp, tc.name+".tar.gz")
			writeTarGz(t, archive, tc.entries)
			assertUntarFails(t, archive, filepath.Join(tmp, tc.name+"-dest"), tc.want)
		})
	}

	shortFile := filepath.Join(tmp, "short-file.tar.gz")
	writeShortFileTarGz(t, shortFile)
	assertUntarFails(t, shortFile, filepath.Join(tmp, "short-file-dest"), "no such file or directory")

	copyFailure := filepath.Join(tmp, "copy-failure.tar.gz")
	writeTarGz(t, copyFailure, []tarEntry{
		{name: "latest/", mode: 0o755, typeflag: tar.TypeDir},
		{name: "latest/file", body: []byte("x"), mode: 0o644, typeflag: tar.TypeReg},
	})
	withUntarCopy(t, func(io.Writer, io.Reader) (int64, error) {
		return 0, errors.New("copy failed")
	})
	assertUntarFails(t, copyFailure, filepath.Join(tmp, "copy-failure-dest"), "copy failed")
}

func TestMoveProperties(t *testing.T) {
	if err := quick.Check(func(data payload, name safeName) bool {
		tmp := t.TempDir()
		src := filepath.Join(tmp, string(name))
		dst := filepath.Join(tmp, "nested", string(name))
		if err := os.WriteFile(src, []byte(data), 0o640); err != nil {
			t.Fatal(err)
		}
		if err := MoveFile(src, dst); err != nil {
			t.Logf("MoveFile failed: %v", err)
			return false
		}
		if _, err := os.Stat(src); !os.IsNotExist(err) {
			t.Logf("source still exists: %v", err)
			return false
		}
		return bytes.Equal(mustReadUtilsFile(t, dst), []byte(data))
	}, quickConfig); err != nil {
		t.Fatal(err)
	}

	if err := quick.Check(func(data payload, name safeName) bool {
		tmp := t.TempDir()
		src := filepath.Join(tmp, "src", string(name))
		dst := filepath.Join(tmp, "dst", string(name))
		if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(src, []byte(data), 0o640); err != nil {
			t.Fatal(err)
		}
		if err := MoveFileCrossFS(src, dst); err != nil {
			t.Logf("MoveFileCrossFS failed: %v", err)
			return false
		}
		if _, err := os.Stat(src); !os.IsNotExist(err) {
			t.Logf("source still exists: %v", err)
			return false
		}
		return bytes.Equal(mustReadUtilsFile(t, dst), []byte(data))
	}, quickConfig); err != nil {
		t.Fatal(err)
	}

	if err := quick.Check(func(data payload) bool {
		tmp := t.TempDir()
		srcTree := filepath.Join(tmp, "tree")
		srcFile := filepath.Join(srcTree, "dir", "file.txt")
		if err := os.MkdirAll(filepath.Dir(srcFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(srcFile, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		dstTree := filepath.Join(tmp, "tree-dst")
		if err := MoveFilesRecursively(srcTree, dstTree); err != nil {
			t.Logf("MoveFilesRecursively failed: %v", err)
			return false
		}
		return bytes.Equal(mustReadUtilsFile(t, filepath.Join(dstTree, "dir", "file.txt")), []byte(data))
	}, quickConfig); err != nil {
		t.Fatal(err)
	}
}

func TestMoveFailures(t *testing.T) {
	tmp := t.TempDir()

	assertErrIsNotExist(t, MoveFile(filepath.Join(tmp, "missing"), filepath.Join(tmp, "dst")))

	parentFile := filepath.Join(tmp, "parent-file")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(tmp, "src")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertErrContains(t, MoveFile(src, filepath.Join(parentFile, "dst")), "not a directory")

	assertErrIsNotExist(t, MoveFileCrossFS(filepath.Join(tmp, "missing-cross"), filepath.Join(tmp, "x")))

	noTmpParent := filepath.Join(tmp, "no-tmp-parent")
	if err := os.WriteFile(noTmpParent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertErrContains(t, MoveFileCrossFS(src, filepath.Join(noTmpParent, "dst")), "not a directory")

	assertErrIsNotExist(t, MoveFilesRecursively(filepath.Join(tmp, "missing-tree"), filepath.Join(tmp, "nope")))
}

func TestMoveInjectedFailures(t *testing.T) {
	tmp := t.TempDir()
	resetMoveHooks := func() {
		moveMkdirAll = os.MkdirAll
		moveRename = os.Rename
		moveOpen = os.Open
		moveCreate = os.Create
		moveCopy = io.Copy
		moveStat = os.Stat
		moveChmod = os.Chmod
		moveRemove = os.Remove
		moveWalk = filepath.Walk
	}
	t.Cleanup(resetMoveHooks)

	for _, tc := range []struct {
		name string
		run  func(src, dst string) error
		hook func()
		want string
	}{
		{
			name: "cross-device-fallback",
			run:  MoveFile,
			hook: func() {
				renames := 0
				moveRename = func(oldpath, newpath string) error {
					renames++
					if renames == 1 {
						return errors.New("invalid cross-device link")
					}
					return os.Rename(oldpath, newpath)
				}
			},
		},
		{
			name: "copy",
			run:  MoveFileCrossFS,
			hook: func() {
				moveCopy = func(io.Writer, io.Reader) (int64, error) {
					return 0, errors.New("copy failed")
				}
			},
			want: "copy failed",
		},
		{
			name: "stat",
			run:  MoveFileCrossFS,
			hook: func() {
				moveStat = func(string) (os.FileInfo, error) {
					return nil, errors.New("stat failed")
				}
			},
			want: "stat failed",
		},
		{
			name: "chmod",
			run:  MoveFileCrossFS,
			hook: func() {
				moveChmod = func(string, os.FileMode) error {
					return errors.New("chmod failed")
				}
			},
			want: "chmod failed",
		},
		{
			name: "rename",
			run:  MoveFileCrossFS,
			hook: func() {
				moveRename = func(string, string) error {
					return errors.New("rename failed")
				}
			},
			want: "rename failed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetMoveHooks()
			src := filepath.Join(tmp, tc.name+"-src")
			dst := filepath.Join(tmp, tc.name+"-dst")
			if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			tc.hook()
			err := tc.run(src, dst)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				if string(mustReadUtilsFile(t, dst)) != "x" {
					t.Fatal("fallback did not move file")
				}
				return
			}
			assertErrContains(t, err, tc.want)
			if tc.name == "rename" && string(mustReadUtilsFile(t, src)) != "x" {
				t.Fatal("source was removed after final rename failure")
			}
			if tc.name == "copy" || tc.name == "stat" || tc.name == "chmod" || tc.name == "rename" {
				if _, statErr := os.Stat(dst + ".tmp"); !os.IsNotExist(statErr) {
					t.Fatalf("temporary destination remains after %s failure: %v", tc.name, statErr)
				}
			}
		})
	}

	resetMoveHooks()
	srcTree := filepath.Join(tmp, "stat-tree")
	if err := os.MkdirAll(srcTree, 0o755); err != nil {
		t.Fatal(err)
	}
	moveStat = func(string) (os.FileInfo, error) {
		return nil, errors.New("walk stat failed")
	}
	assertErrContains(t, MoveFilesRecursively(srcTree, filepath.Join(tmp, "stat-tree-dst")), "walk stat failed")
}

func TestChmodRecursivelyProperties(t *testing.T) {
	if err := quick.Check(func(data payload) bool {
		tmp := t.TempDir()
		file := filepath.Join(tmp, "dir", "file")
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := ChmodRecursively(tmp, 0o755); err != nil {
			t.Logf("ChmodRecursively failed: %v", err)
			return false
		}
		info, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		return runtime.GOOS == "windows" || info.Mode().Perm() == 0o755
	}, quickConfig); err != nil {
		t.Fatal(err)
	}
}

func TestChmodRecursivelyFailures(t *testing.T) {
	tmp := t.TempDir()
	link := filepath.Join(tmp, "dangling")
	if err := os.Symlink(filepath.Join(tmp, "missing-target"), link); err != nil {
		t.Fatal(err)
	}
	assertErrIsNotExist(t, ChmodRecursively(link, 0o755))
	assertErrIsNotExist(t, ChmodRecursively(filepath.Join(tmp, "missing"), 0o755))
}

func withUntarCopy(t *testing.T, copyFunc func(io.Writer, io.Reader) (int64, error)) {
	t.Helper()

	old := untarCopy
	untarCopy = copyFunc
	t.Cleanup(func() {
		untarCopy = old
	})
}

func assertUntarFails(t *testing.T, src, dst, want string) {
	t.Helper()
	assertErrContains(t, untarErr(src, dst), want)
}

func untarErr(src, dst string) error {
	_, err := UnTarGz(src, dst)
	return err
}

func assertErrIsNotExist(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
}

func assertErrContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	if want != "" && !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %v", want, err)
	}
}

func mustReadUtilsFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type tarEntry struct {
	name     string
	body     []byte
	linkname string
	mode     int64
	typeflag byte
}

func writeTarGz(t *testing.T, path string, entries []tarEntry) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		header := &tar.Header{
			Name:     entry.name,
			Linkname: entry.linkname,
			Mode:     entry.mode,
			Size:     int64(len(entry.body)),
			Typeflag: entry.typeflag,
		}
		if entry.typeflag == tar.TypeDir || entry.typeflag == tar.TypeSymlink || entry.typeflag == tar.TypeFifo {
			header.Size = 0
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Size > 0 {
			if _, err := tw.Write(entry.body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeShortFileTarGz(t *testing.T, path string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "latest/file",
		Mode:     0o644,
		Size:     10,
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("short")); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}
