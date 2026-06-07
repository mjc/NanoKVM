package application

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"NanoKVM-Server/proto"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var validFilenameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func (s *Service) OfflineUpdate(c *gin.Context) {
	var rsp proto.Response

	if !acquireUpdateLock() {
		rsp.ErrRsp(c, -1, "update already in progress")
		return
	}
	defer releaseUpdateLock()

	if err := offlineUpdate(c); err != nil {
		rsp.ErrRsp(c, -1, fmt.Sprintf("update failed: %s", err))
		return
	}

	rsp.OkRsp(c)
	log.Debugf("offline update application success")

	time.Sleep(1 * time.Second)
	_ = restartNanoKVM()
}

func offlineUpdate(c *gin.Context) error {
	_ = os.RemoveAll(CacheDir)
	_ = os.MkdirAll(CacheDir, 0o700)
	defer func() {
		_ = os.RemoveAll(CacheDir)
	}()

	if err := checkDownloadInProgress(); err != nil {
		return err
	}

	if err := createSentinelFile(); err != nil {
		return err
	}
	defer removeSentinelFile()

	reader, err := c.Request.MultipartReader()
	if err != nil {
		log.Errorf("Invalid multipart data: %v", err)
		return fmt.Errorf("invalid multipart data: %w", err)
	}

	target, err := processUpload(reader, c.Request.ContentLength)
	if err != nil {
		log.Errorf("failed to upload install package: %v", err)
		return err
	}

	if err := installPackage(target); err != nil {
		log.Errorf("failed to install package: %v", err)
		return err
	}

	return nil
}

func checkDownloadInProgress() error {
	if _, err := os.Stat(sentinelPath); err == nil {
		log.Debug("Download in progress")
		return fmt.Errorf("download already in progress")
	}
	return nil
}

func createSentinelFile() error {
	if err := os.WriteFile(sentinelPath, []byte("downloading"), sentinelPermission); err != nil {
		log.Errorf("Failed to create sentinel file: %v", err)
		return fmt.Errorf("failed to create sentinel file: %w", err)
	}
	return nil
}

func processUpload(reader *multipart.Reader, contentLength int64) (string, error) {
	var outPath string

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read multipart: %w", err)
		}

		if part.FormName() != "file" {
			continue
		}

		outPath, err = saveUploadedFile(part, contentLength)
		if err != nil {
			return "", err
		}
	}

	if outPath == "" {
		return "", fmt.Errorf("no file uploaded")
	}

	return outPath, nil
}

func saveUploadedFile(part *multipart.Part, contentLength int64) (string, error) {
	filename := part.FileName()
	if filename == "" {
		return "", fmt.Errorf("no filename provided")
	}

	if err := validateFilename(filename); err != nil {
		return "", err
	}

	outPath, err := safeCachePath(filename)
	if err != nil {
		return "", err
	}

	if err := copyUploadedFile(part, outPath, contentLength, maxUpdatePackageBytes); err != nil {
		return "", err
	}

	return outPath, nil
}

func copyUploadedFile(part io.Reader, outPath string, contentLength int64, maxBytes int64) error {
	out, err := os.OpenFile(outPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	pw := newProgressWriter(out, contentLength)
	defer pw.Stop()

	limited := &io.LimitedReader{R: part, N: maxBytes + 1}
	if _, err := io.Copy(pw, limited); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	if limited.N == 0 {
		return fmt.Errorf("uploaded file exceeds maximum size")
	}

	return nil
}

func validateFilename(filename string) error {
	baseName := filepath.Base(filename)

	// Check if the path contains directory components
	if baseName != filename {
		log.Warn("Path detected in update filename")
		return fmt.Errorf("path detected in filename")
	}

	// Check for path traversal attempts
	if strings.Contains(filename, "..") {
		log.Warn("Path traversal attempt in update filename")
		return fmt.Errorf("invalid filename: path traversal detected")
	}

	// Validate filename characters
	if !validFilenameRegex.MatchString(filename) {
		log.Warn("Invalid update filename characters")
		return fmt.Errorf("invalid filename: contains invalid characters")
	}

	return nil
}

func removeSentinelFile() {
	_ = os.Remove(sentinelPath)
}
