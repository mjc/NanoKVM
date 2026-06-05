package utils

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	moveMkdirAll = os.MkdirAll
	moveRename   = os.Rename
	moveOpen     = os.Open
	moveCreate   = os.Create
	moveCopy     = io.Copy
	moveStat     = os.Stat
	moveChmod    = os.Chmod
	moveRemove   = os.Remove
	moveWalk     = filepath.Walk
)

func MoveFile(src, dst string) error {
	if err := moveMkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	err := moveRename(src, dst)
	if err != nil {
		if strings.Contains(err.Error(), "invalid cross-device link") {
			return MoveFileCrossFS(src, dst)
		}
		return err
	}
	return nil
}

func MoveFileCrossFS(src, dst string) error {
	tmp := dst + ".tmp"
	srcFile, err := moveOpen(src)
	if err != nil {
		return err
	}

	tmpFile, err := moveCreate(tmp)
	if err != nil {
		_ = srcFile.Close()
		return err
	}
	_, err = moveCopy(tmpFile, srcFile)
	if err != nil {
		_ = srcFile.Close()
		_ = tmpFile.Close()
		return err
	}
	_ = srcFile.Close()
	_ = tmpFile.Close()
	fi, err := moveStat(src)
	if err != nil {
		return err
	}
	err = moveChmod(tmp, fi.Mode())
	if err != nil {
		return err
	}
	err = moveRename(tmp, dst)
	if err != nil {
		return err
	}
	_ = moveRemove(src)
	return nil
}

func MoveFilesRecursively(src, dst string) error {
	return moveWalk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		fileName := strings.Replace(path, src, "", 1)
		dstName := dst + fileName
		fileInfo, err := moveStat(path)
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			return moveMkdirAll(dstName, fileInfo.Mode())
		}
		return MoveFile(path, dstName)
	})
}
