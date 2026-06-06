package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	PrivateDirMode  os.FileMode = 0o700
	PrivateFileMode os.FileMode = 0o600
)

func ReadPrivateFile(path string) ([]byte, error) {
	if err := RepairPrivateFilePermissions(path); err != nil {
		return nil, err
	}

	return os.ReadFile(path)
}

func WritePrivateFile(path string, data []byte) error {
	if err := ensureNoSymlink(path); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, PrivateDirMode); err != nil {
		return err
	}
	if err := os.Chmod(dir, PrivateDirMode); err != nil {
		return err
	}

	if err := os.WriteFile(path, data, PrivateFileMode); err != nil {
		return err
	}

	return os.Chmod(path, PrivateFileMode)
}

func RepairPrivateFilePermissions(path string) error {
	if err := ensureNoSymlink(path); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if info, err := os.Stat(dir); err != nil {
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	if err := os.Chmod(dir, PrivateDirMode); err != nil {
		return err
	}

	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must not be a symlink", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}

	return os.Chmod(path, PrivateFileMode)
}

func RemoveFileIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func ensureNoSymlink(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must not be a symlink", path)
	}

	return nil
}
