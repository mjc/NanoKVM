package usb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func EnsureFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return file.Close()
}

func RemoveIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func ReadTrimmed(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func WriteString(path, value string) error {
	if err := os.WriteFile(filepath.Clean(path), []byte(value), 0o666); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func ClearString(path string) error {
	if err := os.WriteFile(filepath.Clean(path), []byte("\n"), 0o666); err != nil {
		return fmt.Errorf("clear %s: %w", path, err)
	}
	return nil
}
