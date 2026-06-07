package utils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func UnTarGz(srcFile string, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	fr, err := os.Open(srcFile)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = fr.Close()
	}()

	gr, err := gzip.NewReader(fr)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = gr.Close()
	}()

	tr := tar.NewReader(gr)

	targetFile := ""
	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return "", err
		}

		if targetFile == "" {
			parts := strings.Split(header.Name, "/")
			if len(parts) > 0 {
				targetFile = filepath.Join(destDir, parts[0])
			}
		}

		filename, err := archiveTargetPath(destDir, header.Name)
		if err != nil {
			return "", err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filename, 0o700); err != nil {
				return "", err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
				return "", err
			}
			file, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_RDWR, os.FileMode(header.Mode)&0o700)
			if err != nil {
				return "", err
			}

			if _, err := io.Copy(file, tr); err != nil {
				_ = file.Close()
				return "", err
			}
			_ = file.Close()

		case tar.TypeSymlink:
			return "", fmt.Errorf("archive symlinks are not supported")
		}
	}

	return targetFile, nil
}

func archiveTargetPath(destDir, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("invalid archive path")
	}

	cleanDest := filepath.Clean(destDir)
	target := filepath.Join(cleanDest, filepath.Clean(name))
	rel, err := filepath.Rel(cleanDest, target)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("archive path escapes destination")
	}
	return target, nil
}
