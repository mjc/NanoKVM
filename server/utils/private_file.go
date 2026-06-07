package utils

import (
	"errors"
	"os"
	"path/filepath"
)

const privateFileMode os.FileMode = 0o600
const privateDirMode os.FileMode = 0o700

func ReadPrivateFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("refuse to read private symlink")
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("private path is not a regular file")
	}
	if info.Mode().Perm() != privateFileMode {
		if err := os.Chmod(path, privateFileMode); err != nil {
			return nil, err
		}
	}
	return os.ReadFile(path)
}

func WritePrivateFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), privateDirMode); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if err := tmp.Chmod(privateFileMode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
