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

	time.Sleep(restartDelay)
	_ = restartCommand("sh", "-c", "/etc/init.d/S95nanokvm restart").Run()
}

func offlineUpdate(c *gin.Context) error {
	if err := removeAll(cacheDir); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}
	if err := mkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}
	defer func() {
		_ = removeAll(cacheDir)
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
	if _, err := statFile(sentinelFilePath); err == nil {
		log.Debug("Download in progress")
		return fmt.Errorf("download already in progress")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check download progress: %w", err)
	}
	return nil
}

func createSentinelFile() error {
	if err := writeFile(sentinelFilePath, []byte("downloading"), sentinelPermission); err != nil {
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

	outPath := filepath.Join(cacheDir, filename)
	out, err := createFile(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	pw := newProgressWriter(out, contentLength)
	defer pw.Stop()

	if _, err := io.Copy(pw, part); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return outPath, nil
}

func validateFilename(filename string) error {
	baseName := filepath.Base(filename)
	if baseName == "." {
		log.Warnf("Reserved filename: %s", filename)
		return fmt.Errorf("reserved filename")
	}

	// Check if the path contains directory components
	if baseName != filename {
		log.Warnf("Path detected in filename: %s", filename)
		return fmt.Errorf("path detected in filename")
	}

	// Check for path traversal attempts
	if strings.Contains(filename, "..") {
		log.Warnf("Path traversal attempt: %s", filename)
		return fmt.Errorf("invalid filename: path traversal detected")
	}

	// Validate filename characters
	if !validFilenameRegex.MatchString(filename) {
		log.Warnf("Invalid filename characters: %s", filename)
		return fmt.Errorf("invalid filename: contains invalid characters")
	}
	if !strings.HasSuffix(filename, ".tar.gz") {
		log.Warnf("Invalid filename extension: %s", filename)
		return fmt.Errorf("invalid filename: must end with .tar.gz")
	}

	return nil
}

func removeSentinelFile() {
	_ = removeFile(sentinelFilePath)
}
