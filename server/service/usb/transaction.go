package usb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"
)

const (
	hidReopenTimeout     = 2 * time.Second
	hidReopenDelay       = 100 * time.Millisecond
	usbPHYRestartTimeout = 10 * time.Second
)

type HIDController interface {
	Lock()
	Unlock()
	CloseNoLock()
	OpenNoLockWithRetry(time.Duration, time.Duration) error
}

func WithDetachedUDC(h HIDController, mutate func() error) error {
	h.Lock()
	h.CloseNoLock()
	defer h.Unlock()

	detachErr := DetachUDC()
	if detachErr != nil {
		openErr := h.OpenNoLockWithRetry(hidReopenTimeout, hidReopenDelay)
		return errors.Join(detachErr, openErr)
	}

	mutateErr := mutate()
	attachErr := AttachUDC()
	openErr := h.OpenNoLockWithRetry(hidReopenTimeout, hidReopenDelay)

	return errors.Join(detachErr, mutateErr, attachErr, openErr)
}

func Rebind(h HIDController) error {
	return WithDetachedUDC(h, func() error { return nil })
}

func RestartPHY(h HIDController) error {
	h.Lock()
	h.CloseNoLock()
	defer h.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), usbPHYRestartTimeout)
	defer cancel()

	if err := exec.CommandContext(ctx, ActiveInitScript, "restart_phy").Run(); err != nil {
		return fmt.Errorf("restart usb phy: %w", err)
	}

	if err := h.OpenNoLockWithRetry(hidReopenTimeout, hidReopenDelay); err != nil {
		return fmt.Errorf("reopen HID devices after usb phy reset: %w", err)
	}

	return nil
}

func DetachUDC() error {
	return ClearString(UDCPath)
}

func AttachUDC() error {
	udc, err := FirstUDC()
	if err != nil {
		return err
	}

	if err := WriteString(UDCPath, udc); err != nil {
		return err
	}

	if Exists(OTGRole) {
		return WriteString(OTGRole, "device")
	}
	return nil
}

func FirstUDC() (string, error) {
	entries, err := os.ReadDir(UDCClass)
	if err != nil {
		return "", fmt.Errorf("read UDC list: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		return name, nil
	}
	return "", fmt.Errorf("no UDC found in %s", UDCClass)
}
