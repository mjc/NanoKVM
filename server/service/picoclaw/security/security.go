package security

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

var validSessionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func CopyWithProgress(ctx context.Context, dst io.Writer, src io.Reader, total int64, maxBytes int64, onProgress func(downloaded int64, total int64)) error {
	if total > maxBytes {
		return fmt.Errorf("download exceeds maximum size")
	}

	buffer := make([]byte, 32*1024)
	var downloaded int64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := src.Read(buffer)
		if n > 0 {
			if _, writeErr := dst.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)
			if downloaded > maxBytes {
				return fmt.Errorf("download exceeds maximum size")
			}
			if onProgress != nil {
				onProgress(downloaded, total)
			}
		}
		if readErr == io.EOF {
			if onProgress != nil {
				onProgress(downloaded, total)
			}
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func IsPicoclawBinaryEntry(header *tar.Header) bool {
	if header == nil || header.Typeflag != tar.TypeReg {
		return false
	}

	name := filepath.Clean(header.Name)
	if filepath.IsAbs(name) || name == "." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
		return false
	}
	return filepath.Base(name) == "picoclaw"
}

func IsValidSessionID(sessionID string) bool {
	return validSessionID.MatchString(sessionID)
}

func PicoclawScriptArgs(scriptPath string, action string) ([]string, bool) {
	switch action {
	case "start", "stop", "onboard":
		return []string{scriptPath, action}, true
	default:
		return nil, false
	}
}

func SanitizeRuntimeOutput(output []byte) string {
	if len(output) == 0 {
		return ""
	}
	return "picoclaw command failed"
}
