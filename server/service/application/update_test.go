package application

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/gin-gonic/gin"
)

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read failed")
}

var appQuickConfig = &quick.Config{MaxCount: 16}

type appVersion string

func (appVersion) Generate(rand *rand.Rand, _ int) reflect.Value {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-"
	size := 1 + rand.Intn(24)
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return reflect.ValueOf(appVersion(buf))
}

type appFilename string

func (appFilename) Generate(rand *rand.Rand, _ int) reflect.Value {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
	size := 1 + rand.Intn(24)
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return reflect.ValueOf(appFilename(string(buf) + ".tar.gz"))
}

type appPayload []byte

func (appPayload) Generate(rand *rand.Rand, _ int) reflect.Value {
	size := rand.Intn(256)
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = byte(rand.Intn(256))
	}
	return reflect.ValueOf(appPayload(buf))
}

func resetApplicationTestState(t *testing.T) string {
	t.Helper()

	gin.SetMode(gin.TestMode)

	tmp := t.TempDir()
	oldStableURL := stableURL
	oldPreviewURL := previewURL
	oldAppDir := appDir
	oldBackupDir := backupDir
	oldCacheDir := cacheDir
	oldPreviewUpdatesFlag := previewUpdatesFlag
	oldSentinelFilePath := sentinelFilePath
	oldRetryDelay := retryDelay
	oldRestartDelay := restartDelay
	oldRestartCommand := restartCommand
	oldDownloadRequest := downloadRequest
	oldHTTPGet := httpGet
	oldUntarGz := untarGz
	oldMoveFilesRecursively := moveFilesRecursively
	oldChmodRecursively := chmodRecursively
	oldCreateFile := createFile
	oldMkdirAll := mkdirAll
	oldReadFile := readFile
	oldRemoveAll := removeAll
	oldRemoveFile := removeFile
	oldStatFile := statFile
	oldWriteFile := writeFile

	stableURL = "http://example.test/stable"
	previewURL = "http://example.test/preview"
	appDir = filepath.Join(tmp, "kvmapp")
	backupDir = filepath.Join(tmp, "old")
	cacheDir = filepath.Join(tmp, "cache")
	previewUpdatesFlag = filepath.Join(tmp, "preview_updates")
	sentinelFilePath = filepath.Join(tmp, "download_in_progress")
	retryDelay = 0
	restartDelay = 0
	restartCommand = func(string, ...string) *exec.Cmd { return exec.Command("true") }
	downloadRequest = oldDownloadRequest
	httpGet = oldHTTPGet
	untarGz = oldUntarGz
	moveFilesRecursively = oldMoveFilesRecursively
	chmodRecursively = oldChmodRecursively
	createFile = oldCreateFile
	mkdirAll = oldMkdirAll
	readFile = oldReadFile
	removeAll = oldRemoveAll
	removeFile = oldRemoveFile
	statFile = oldStatFile
	writeFile = oldWriteFile

	mutex.Lock()
	isUpdating = false
	mutex.Unlock()

	t.Cleanup(func() {
		stableURL = oldStableURL
		previewURL = oldPreviewURL
		appDir = oldAppDir
		backupDir = oldBackupDir
		cacheDir = oldCacheDir
		previewUpdatesFlag = oldPreviewUpdatesFlag
		sentinelFilePath = oldSentinelFilePath
		retryDelay = oldRetryDelay
		restartDelay = oldRestartDelay
		restartCommand = oldRestartCommand
		downloadRequest = oldDownloadRequest
		httpGet = oldHTTPGet
		untarGz = oldUntarGz
		moveFilesRecursively = oldMoveFilesRecursively
		chmodRecursively = oldChmodRecursively
		createFile = oldCreateFile
		mkdirAll = oldMkdirAll
		readFile = oldReadFile
		removeAll = oldRemoveAll
		removeFile = oldRemoveFile
		statFile = oldStatFile
		writeFile = oldWriteFile
		mutex.Lock()
		isUpdating = false
		mutex.Unlock()
	})

	return tmp
}

func makePackage(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		content := files[name]
		if strings.HasSuffix(name, "/") {
			if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
				t.Fatal(err)
			}
			continue
		}

		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func sha512Base64(data []byte) string {
	sum := sha512.Sum512(data)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func responseRecorder(method, path string, body io.Reader, contentType string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	c.Request = req
	return c, w
}

func responseCode(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()

	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Code
}

func TestNewService(t *testing.T) {
	if NewService() == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestUpdateLock(t *testing.T) {
	resetApplicationTestState(t)

	if !acquireUpdateLock() {
		t.Fatal("first lock acquisition failed")
	}
	if acquireUpdateLock() {
		t.Fatal("second lock acquisition succeeded")
	}
	releaseUpdateLock()
	if !acquireUpdateLock() {
		t.Fatal("lock was not released")
	}
	releaseUpdateLock()
}

func TestUpdateHandlerSuccessProperty(t *testing.T) {
	prop := func(oldVersion, newVersion appVersion, filename appFilename) bool {
		resetApplicationTestState(t)
		if err := os.MkdirAll(appDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appDir, "version"), []byte(oldVersion), 0o644); err != nil {
			t.Fatal(err)
		}

		pkg := makePackage(t, map[string]string{
			"latest/":        "",
			"latest/version": string(newVersion),
		})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/latest.json":
				_ = json.NewEncoder(w).Encode(&Latest{
					Version: string(newVersion),
					Name:    string(filename),
					Sha512:  sha512Base64(pkg),
				})
			case "/" + string(filename):
				w.Header().Set("Content-Type", "application/gzip")
				_, _ = w.Write(pkg)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()
		stableURL = server.URL

		restarts := 0
		restartCommand = func(string, ...string) *exec.Cmd {
			restarts++
			return exec.Command("true")
		}

		c, w := responseRecorder(http.MethodPost, "/update", nil, "")
		NewService().Update(c)

		return responseCode(t, w) == 0 &&
			restarts == 1 &&
			strings.TrimSpace(string(mustReadFile(t, filepath.Join(appDir, "version")))) == string(newVersion) &&
			strings.TrimSpace(string(mustReadFile(t, filepath.Join(backupDir, "version")))) == string(oldVersion) &&
			pathIsMissing(cacheDir)
	}

	if err := quick.Check(prop, appQuickConfig); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateHandlerLockAndFailure(t *testing.T) {
	resetApplicationTestState(t)

	mutex.Lock()
	isUpdating = true
	mutex.Unlock()
	c, w := responseRecorder(http.MethodPost, "/update", nil, "")
	NewService().Update(c)
	if responseCode(t, w) == 0 {
		t.Fatal("locked update succeeded")
	}

	mutex.Lock()
	isUpdating = false
	mutex.Unlock()
	httpGet = func(string) (*http.Response, error) {
		return nil, errors.New("latest failed")
	}
	c, w = responseRecorder(http.MethodPost, "/update", nil, "")
	NewService().Update(c)
	if responseCode(t, w) == 0 {
		t.Fatal("failed update succeeded")
	}
}

func TestUpdateDownloadChecksumAndInstallFailures(t *testing.T) {
	resetApplicationTestState(t)

	httpGet = latestResponder(t, &Latest{Name: "app.tar.gz", Sha512: "hash"})
	downloadRequest = func(*http.Request, string) error {
		return errors.New("download failed")
	}
	if err := update(); err == nil || !strings.Contains(err.Error(), "download failed") {
		t.Fatalf("expected download failure, got %v", err)
	}

	pkg := []byte("not a tarball")
	httpGet = latestResponder(t, &Latest{Name: "app.tar.gz", Sha512: "wrong"})
	downloadRequest = writeDownload(pkg)
	if err := update(); err == nil || !strings.Contains(err.Error(), "invalid sha512") {
		t.Fatalf("expected checksum failure, got %v", err)
	}

	httpGet = latestResponder(t, &Latest{Name: "app.tar.gz", Sha512: sha512Base64(pkg)})
	if err := update(); err == nil || !strings.Contains(err.Error(), "failed to decompress") {
		t.Fatalf("expected install failure, got %v", err)
	}
}

func TestDownload(t *testing.T) {
	resetApplicationTestState(t)

	if err := download("http://[::1", filepath.Join(cacheDir, "bad")); err == nil {
		t.Fatal("invalid URL unexpectedly downloaded")
	}

	calls := 0
	downloadRequest = func(*http.Request, string) error {
		calls++
		return errors.New("temporary failure")
	}
	if err := download("http://example.test/pkg", filepath.Join(cacheDir, "pkg")); err == nil {
		t.Fatal("download unexpectedly succeeded")
	}
	if calls != maxTries {
		t.Fatalf("expected %d attempts, got %d", maxTries, calls)
	}

	downloadRequest = writeDownload([]byte("ok"))
	if err := download("http://example.test/pkg", filepath.Join(cacheDir, "pkg")); err != nil {
		t.Fatal(err)
	}
}

func TestChecksumProperty(t *testing.T) {
	resetApplicationTestState(t)

	if err := checksum(filepath.Join(cacheDir, "missing"), ""); err == nil {
		t.Fatal("missing file checksum succeeded")
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := checksum(cacheDir, ""); err == nil {
		t.Fatal("directory checksum succeeded")
	}

	prop := func(data []byte) bool {
		file := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(file, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := checksum(file, "wrong"); err == nil {
			t.Log("bad hash succeeded")
			return false
		}
		if err := checksum(file, sha512Base64(data)); err != nil {
			t.Logf("valid hash failed: %v", err)
			return false
		}
		return true
	}

	if err := quick.Check(prop, appQuickConfig); err != nil {
		t.Fatal(err)
	}
}

func TestInstallPackageErrors(t *testing.T) {
	resetApplicationTestState(t)

	untarGz = func(string, string) (string, error) {
		return "", errors.New("untar failed")
	}
	if err := installPackage("pkg"); err == nil || !strings.Contains(err.Error(), "untar failed") {
		t.Fatalf("expected untar failure, got %v", err)
	}

	untarGz = func(string, string) (string, error) {
		return "source", nil
	}
	moveFilesRecursively = func(string, string) error {
		return errors.New("backup failed")
	}
	if err := installPackage("pkg"); err == nil || !strings.Contains(err.Error(), "failed to backup") {
		t.Fatalf("expected backup failure, got %v", err)
	}

	removeAll = func(string) error {
		return errors.New("remove failed")
	}
	if err := backupCurrentApp(); err == nil || !strings.Contains(err.Error(), "failed to remove backup") {
		t.Fatalf("expected remove failure, got %v", err)
	}
	removeAll = os.RemoveAll

	calls := 0
	moveFilesRecursively = func(string, string) error {
		calls++
		if calls == 1 {
			return errors.New("apply failed")
		}
		return nil
	}
	if err := applyUpdate("source"); err == nil || !strings.Contains(err.Error(), "failed to move update") {
		t.Fatalf("expected apply failure, got %v", err)
	}

	untarGz = func(string, string) (string, error) {
		return "source", nil
	}
	calls = 0
	moveFilesRecursively = func(string, string) error {
		calls++
		if calls == 2 {
			return errors.New("apply failed")
		}
		return nil
	}
	if err := installPackage("pkg"); err == nil || !strings.Contains(err.Error(), "failed to move update") {
		t.Fatalf("expected install apply failure, got %v", err)
	}

	moveFilesRecursively = func(string, string) error {
		return errors.New("move failed")
	}
	if err := applyUpdate("source"); err == nil || !strings.Contains(err.Error(), "failed to move update") {
		t.Fatalf("expected apply failure with restore failure, got %v", err)
	}

	untarGz = func(string, string) (string, error) {
		return "source", nil
	}
	moveFilesRecursively = func(string, string) error {
		return nil
	}
	chmodRecursively = func(string, uint32) error {
		return errors.New("chmod failed")
	}
	if err := installPackage("pkg"); err == nil || !strings.Contains(err.Error(), "failed to chmod") {
		t.Fatalf("expected chmod failure, got %v", err)
	}
}

func TestOfflineUpdateSuccessProperty(t *testing.T) {
	prop := func(oldVersion, newVersion appVersion, filename appFilename) bool {
		resetApplicationTestState(t)
		if err := os.MkdirAll(appDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appDir, "version"), []byte(oldVersion), 0o644); err != nil {
			t.Fatal(err)
		}

		pkg := makePackage(t, map[string]string{
			"latest/":        "",
			"latest/version": string(newVersion),
		})
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, err := mw.CreateFormFile("file", string(filename))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(pkg); err != nil {
			t.Fatal(err)
		}
		if err := mw.Close(); err != nil {
			t.Fatal(err)
		}

		restarts := 0
		restartCommand = func(string, ...string) *exec.Cmd {
			restarts++
			return exec.Command("true")
		}
		c, w := responseRecorder(http.MethodPost, "/offline", &body, mw.FormDataContentType())
		c.Request.ContentLength = int64(body.Len())
		NewService().OfflineUpdate(c)

		return responseCode(t, w) == 0 &&
			restarts == 1 &&
			strings.TrimSpace(string(mustReadFile(t, filepath.Join(appDir, "version")))) == string(newVersion) &&
			strings.TrimSpace(string(mustReadFile(t, filepath.Join(backupDir, "version")))) == string(oldVersion) &&
			pathIsMissing(sentinelFilePath)
	}

	if err := quick.Check(prop, appQuickConfig); err != nil {
		t.Fatal(err)
	}
}

func TestOfflineUpdateFailures(t *testing.T) {
	resetApplicationTestState(t)

	mutex.Lock()
	isUpdating = true
	mutex.Unlock()
	c, w := responseRecorder(http.MethodPost, "/offline", strings.NewReader(""), "multipart/form-data")
	NewService().OfflineUpdate(c)
	if responseCode(t, w) == 0 {
		t.Fatal("locked offline handler succeeded")
	}
	mutex.Lock()
	isUpdating = false
	mutex.Unlock()

	if err := os.WriteFile(sentinelFilePath, []byte("downloading"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, w = responseRecorder(http.MethodPost, "/offline", strings.NewReader(""), "multipart/form-data")
	NewService().OfflineUpdate(c)
	if responseCode(t, w) == 0 {
		t.Fatal("locked offline update succeeded")
	}
	if err := os.Remove(sentinelFilePath); err != nil {
		t.Fatal(err)
	}

	writeFile = func(string, []byte, os.FileMode) error {
		return errors.New("sentinel failed")
	}
	if err := createSentinelFile(); err == nil || !strings.Contains(err.Error(), "sentinel") {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	c, _ = responseRecorder(http.MethodPost, "/offline", strings.NewReader(""), "multipart/form-data")
	if err := offlineUpdate(c); err == nil || !strings.Contains(err.Error(), "sentinel") {
		t.Fatalf("expected offline sentinel error, got %v", err)
	}
	writeFile = os.WriteFile

	c, _ = responseRecorder(http.MethodPost, "/offline", strings.NewReader("not multipart"), "bad")
	if err := offlineUpdate(c); err == nil || !strings.Contains(err.Error(), "invalid multipart") {
		t.Fatalf("expected multipart error, got %v", err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("ignored", "value"); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	c, _ = responseRecorder(http.MethodPost, "/offline", &body, mw.FormDataContentType())
	if err := offlineUpdate(c); err == nil || !strings.Contains(err.Error(), "no file uploaded") {
		t.Fatalf("expected no file error, got %v", err)
	}

	body.Reset()
	mw = multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "bad name.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("x"))
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	c, _ = responseRecorder(http.MethodPost, "/offline", &body, mw.FormDataContentType())
	if err := offlineUpdate(c); err == nil || !strings.Contains(err.Error(), "invalid characters") {
		t.Fatalf("expected filename error, got %v", err)
	}

	body.Reset()
	mw = multipart.NewWriter(&body)
	part, err = mw.CreateFormFile("file", "bad.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("not a tarball"))
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	c, _ = responseRecorder(http.MethodPost, "/offline", &body, mw.FormDataContentType())
	if err := offlineUpdate(c); err == nil || !strings.Contains(err.Error(), "failed to decompress") {
		t.Fatalf("expected offline install error, got %v", err)
	}
}

func TestProcessUploadAndSaveFailures(t *testing.T) {
	resetApplicationTestState(t)

	if err := quick.Check(func(filename appFilename) bool {
		return validateFilename(string(filename)) == nil
	}, appQuickConfig); err != nil {
		t.Fatal(err)
	}

	for _, filename := range []string{"../bad.tar.gz", "..bad", "bad name.tar.gz", "."} {
		if err := validateFilename(filename); err == nil {
			t.Fatalf("filename %q unexpectedly valid", filename)
		}
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "upload.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("data"))
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/offline", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	reader, err := req.MultipartReader()
	if err != nil {
		t.Fatal(err)
	}
	createFile = func(string) (*os.File, error) {
		return nil, errors.New("create failed")
	}
	if _, err := processUpload(reader, int64(body.Len())); err == nil || !strings.Contains(err.Error(), "create output") {
		t.Fatalf("expected create failure, got %v", err)
	}
	createFile = os.Create

	body.Reset()
	mw = multipart.NewWriter(&body)
	part, err = mw.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": []string{`form-data; name="file"`},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("data"))
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/offline", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	reader, err = req.MultipartReader()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := processUpload(reader, int64(body.Len())); err == nil || !strings.Contains(err.Error(), "no filename") {
		t.Fatalf("expected missing filename, got %v", err)
	}

	if err := quick.Check(func(data appPayload, filename appFilename) bool {
		resetApplicationTestState(t)
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			t.Fatal(err)
		}
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, err := mw.CreateFormFile("file", string(filename))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
		if err := mw.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/offline", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		reader, err := req.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		outPath, err := processUpload(reader, int64(body.Len()))
		if err != nil {
			t.Logf("processUpload failed: %v", err)
			return false
		}
		return filepath.Base(outPath) == string(filename) && bytes.Equal(mustReadFile(t, outPath), []byte(data))
	}, appQuickConfig); err != nil {
		t.Fatal(err)
	}

	truncated := "--bad\r\nContent-Disposition: form-data; name=\"file\"; filename=\"upload.tar.gz\"\r\n\r\npartial"
	req = httptest.NewRequest(http.MethodPost, "/offline", strings.NewReader(truncated))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=bad")
	reader, err = req.MultipartReader()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := processUpload(reader, int64(len(truncated))); err == nil || !strings.Contains(err.Error(), "failed to write file") {
		t.Fatalf("expected write failure, got %v", err)
	}
}

func TestProgressWriter(t *testing.T) {
	resetApplicationTestState(t)

	var buf bytes.Buffer
	pw := newProgressWriter(&buf, 4)
	if n, err := pw.Write([]byte("ok")); err != nil || n != 2 {
		t.Fatalf("write = %d, %v", n, err)
	}
	pw.Stop()

	pw = &progressWriter{totalSize: 0}
	pw.updateSentinel()

	pw = &progressWriter{written: 2, totalSize: 4}
	readFile = func(string) ([]byte, error) {
		return nil, errors.New("read failed")
	}
	pw.updateSentinel()

	readFile = func(string) ([]byte, error) {
		return []byte("downloading"), nil
	}
	var wrote string
	writeFile = func(_ string, data []byte, _ os.FileMode) error {
		wrote = string(data)
		return nil
	}
	pw.updateSentinel()
	if wrote != "downloading;50.00%" {
		t.Fatalf("sentinel content = %q", wrote)
	}

	writeFile = func(string, []byte, os.FileMode) error {
		return errors.New("write failed")
	}
	pw.updateSentinel()
}

func TestVersionAndPreview(t *testing.T) {
	resetApplicationTestState(t)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "version"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	httpGet = latestResponder(t, &Latest{Version: "2.0.0", Name: "app.tar.gz"})
	c, w := responseRecorder(http.MethodGet, "/version", nil, "")
	NewService().GetVersion(c)
	if responseCode(t, w) != 0 || !strings.Contains(w.Body.String(), "1.2.3") || !strings.Contains(w.Body.String(), "2.0.0") {
		t.Fatalf("unexpected version response: %s", w.Body.String())
	}

	httpGet = func(string) (*http.Response, error) {
		return nil, errors.New("latest failed")
	}
	c, w = responseRecorder(http.MethodGet, "/version", nil, "")
	NewService().GetVersion(c)
	if responseCode(t, w) != 0 {
		t.Fatalf("unexpected version failure response: %s", w.Body.String())
	}

	c, w = responseRecorder(http.MethodGet, "/preview", nil, "")
	NewService().GetPreview(c)
	if responseCode(t, w) != 0 {
		t.Fatalf("unexpected preview response: %s", w.Body.String())
	}

	c, w = responseRecorder(http.MethodPost, "/preview", strings.NewReader("{"), "application/json")
	NewService().SetPreview(c)
	if responseCode(t, w) == 0 {
		t.Fatal("invalid preview request succeeded")
	}

	c, w = responseRecorder(http.MethodPost, "/preview", strings.NewReader(`{"enable":true}`), "application/json")
	NewService().SetPreview(c)
	if responseCode(t, w) != 0 {
		t.Fatalf("enable preview failed: %s", w.Body.String())
	}
	if !isPreviewEnabled() {
		t.Fatal("preview flag was not enabled")
	}

	c, w = responseRecorder(http.MethodPost, "/preview", strings.NewReader("enable=true"), "application/x-www-form-urlencoded")
	NewService().SetPreview(c)
	if responseCode(t, w) != 0 {
		t.Fatalf("idempotent enable failed: %s", w.Body.String())
	}

	c, w = responseRecorder(http.MethodPost, "/preview", strings.NewReader(`{"enable":false}`), "application/json")
	NewService().SetPreview(c)
	if responseCode(t, w) != 0 {
		t.Fatalf("disable preview failed: %s", w.Body.String())
	}

	writeFile = func(string, []byte, os.FileMode) error {
		return errors.New("write failed")
	}
	c, w = responseRecorder(http.MethodPost, "/preview", strings.NewReader(`{"enable":true}`), "application/json")
	NewService().SetPreview(c)
	if responseCode(t, w) == 0 {
		t.Fatal("preview enable write failure succeeded")
	}
	writeFile = os.WriteFile

	if err := os.WriteFile(previewUpdatesFlag, []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	removeFile = func(string) error {
		return errors.New("remove failed")
	}
	c, w = responseRecorder(http.MethodPost, "/preview", strings.NewReader(`{"enable":false}`), "application/json")
	NewService().SetPreview(c)
	if responseCode(t, w) == 0 {
		t.Fatal("preview disable remove failure succeeded")
	}
	removeFile = os.Remove
}

func TestGetLatest(t *testing.T) {
	resetApplicationTestState(t)

	prop := func(version appVersion, filename appFilename, preview bool) bool {
		resetApplicationTestState(t)
		if preview {
			if err := os.WriteFile(previewUpdatesFlag, []byte("1"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		var requested string
		httpGet = func(url string) (*http.Response, error) {
			requested = url
			return latestResponder(t, &Latest{Version: string(version), Name: string(filename)})(url)
		}
		latest, err := getLatest()
		if err != nil {
			t.Logf("getLatest failed: %v", err)
			return false
		}

		baseURL := stableURL
		if preview {
			baseURL = previewURL
		}
		return latest.Version == string(version) &&
			latest.Name == string(filename) &&
			latest.Url == baseURL+"/"+string(filename) &&
			strings.HasPrefix(requested, baseURL+"/latest.json?now=")
	}

	if err := quick.Check(prop, appQuickConfig); err != nil {
		t.Fatal(err)
	}

	httpGet = func(string) (*http.Response, error) {
		return nil, errors.New("request failed")
	}
	if _, err := getLatest(); err == nil {
		t.Fatal("request failure succeeded")
	}

	httpGet = func(string) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(errReader{}),
		}, nil
	}
	if _, err := getLatest(); err == nil {
		t.Fatal("body read failure succeeded")
	}

	httpGet = func(string) (*http.Response, error) {
		return jsonResponse(http.StatusInternalServerError, `nope`), nil
	}
	if _, err := getLatest(); err == nil || !strings.Contains(err.Error(), "status code") {
		t.Fatalf("expected status error, got %v", err)
	}

	httpGet = func(string) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{`), nil
	}
	if _, err := getLatest(); err == nil {
		t.Fatal("invalid json succeeded")
	}
}

func latestResponder(t *testing.T, latest *Latest) func(string) (*http.Response, error) {
	t.Helper()

	return func(string) (*http.Response, error) {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(latest); err != nil {
			t.Fatal(err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(&buf),
		}, nil
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func writeDownload(data []byte) func(*http.Request, string) error {
	return func(_ *http.Request, target string) error {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func pathIsMissing(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

func TestRemoveSentinelFile(t *testing.T) {
	resetApplicationTestState(t)

	if err := os.WriteFile(sentinelFilePath, []byte("downloading"), 0o644); err != nil {
		t.Fatal(err)
	}
	removeSentinelFile()
	if _, err := os.Stat(sentinelFilePath); !os.IsNotExist(err) {
		t.Fatalf("sentinel still exists: %v", err)
	}

	removeFile = func(string) error {
		return errors.New("ignored")
	}
	removeSentinelFile()
}

func TestProcessUploadReadMultipartError(t *testing.T) {
	resetApplicationTestState(t)

	req := httptest.NewRequest(http.MethodPost, "/offline", strings.NewReader("--bad\r\nbroken header\r\n\r\n"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=bad")
	reader, err := req.MultipartReader()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := processUpload(reader, 0); err == nil || !strings.Contains(err.Error(), "failed to read multipart") {
		t.Fatalf("expected multipart read error, got %v", err)
	}
}

func TestProgressWriterReportProgress(t *testing.T) {
	resetApplicationTestState(t)

	ticked := make(chan struct{})
	readFile = func(string) ([]byte, error) {
		return []byte("downloading"), nil
	}
	writeFile = func(string, []byte, os.FileMode) error {
		select {
		case <-ticked:
		default:
			close(ticked)
		}
		return nil
	}
	pw := &progressWriter{
		written:   1,
		totalSize: 2,
		ticker:    time.NewTicker(time.Nanosecond),
		done:      make(chan struct{}),
	}
	go pw.reportProgress()
	select {
	case <-ticked:
	case <-time.After(time.Second):
		t.Fatal("reportProgress did not process ticker")
	}
	close(pw.done)
	pw.ticker.Stop()

	pw = &progressWriter{
		ticker: time.NewTicker(time.Hour),
		done:   make(chan struct{}),
	}
	done := make(chan struct{})
	go func() {
		pw.reportProgress()
		close(done)
	}()
	close(pw.done)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reportProgress did not exit")
	}
	pw.ticker.Stop()
}
