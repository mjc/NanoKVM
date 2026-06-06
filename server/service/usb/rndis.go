package usb

import (
	"errors"
	"fmt"
	"os"
)

func SetRNDISEnabled(h HIDController, enabled bool) error {
	return WithDetachedUDC(h, func() error {
		if enabled {
			if err := EnsureFile(RNDISFlag); err != nil {
				return err
			}
			if err := prepareRNDISFunction(); err != nil {
				return err
			}
			return nil
		}

		return errors.Join(
			RemoveIfExists(RNDISLink),
			RemoveIfExists(RNDISFlag),
		)
	})
}

func RNDISEnabled() bool {
	return Exists(RNDISLink)
}

func ensureRNDISFunction() error {
	if Exists(RNDISFunction) {
		return nil
	}
	if err := os.Mkdir(RNDISFunction, 0o777); err != nil {
		return fmt.Errorf("create RNDIS function: %w", err)
	}
	return nil
}

func prepareRNDISFunction() error {
	if err := ensureRNDISFunction(); err != nil {
		return err
	}
	return linkRNDISFunction()
}

func linkRNDISFunction() error {
	if !Exists(RNDISLink) {
		return os.Symlink(RNDISFunction, RNDISLink)
	}
	return nil
}
