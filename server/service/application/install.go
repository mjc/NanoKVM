package application

import (
	"fmt"
	"sync"

	"NanoKVM-Server/utils"
	log "github.com/sirupsen/logrus"
)

var (
	mutex      sync.Mutex
	isUpdating bool

	untarGz              = utils.UnTarGz
	moveFilesRecursively = utils.MoveFilesRecursively
	chmodRecursively     = utils.ChmodRecursively
)

func acquireUpdateLock() bool {
	mutex.Lock()
	defer mutex.Unlock()

	if isUpdating {
		return false
	}
	isUpdating = true
	return true
}

func releaseUpdateLock() {
	mutex.Lock()
	defer mutex.Unlock()
	isUpdating = false
}

func installPackage(source string) error {
	dir, err := untarGz(source, cacheDir)
	if err != nil {
		return fmt.Errorf("failed to decompress app: %w", err)
	}

	if err := backupCurrentApp(); err != nil {
		return err
	}

	if err := applyUpdate(dir); err != nil {
		return err
	}

	if err := chmodRecursively(appDir, 0o755); err != nil {
		return fmt.Errorf("failed to chmod: %w", err)
	}

	return nil
}

func backupCurrentApp() error {
	if err := removeAll(backupDir); err != nil {
		return fmt.Errorf("failed to remove backup: %w", err)
	}

	if err := moveFilesRecursively(appDir, backupDir); err != nil {
		return fmt.Errorf("failed to backup app: %w", err)
	}

	return nil
}

func applyUpdate(sourceDir string) error {
	if err := moveFilesRecursively(sourceDir, appDir); err != nil {
		// Try to restore backup on failure
		if restoreErr := moveFilesRecursively(backupDir, appDir); restoreErr != nil {
			log.Errorf("Failed to restore backup after update failure: %v", restoreErr)
		}
		return fmt.Errorf("failed to move update in place: %w", err)
	}
	return nil
}
