package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResetUSBGadgetUDCPathsRebindsFirstController(t *testing.T) {
	tempDir := t.TempDir()
	udcPath := filepath.Join(tempDir, "gadget", "UDC")
	if err := os.MkdirAll(filepath.Dir(udcPath), 0o755); err != nil {
		t.Fatalf("mkdir udc dir: %v", err)
	}
	if err := os.WriteFile(udcPath, []byte("old-controller"), 0o644); err != nil {
		t.Fatalf("seed udc file: %v", err)
	}

	classDir := filepath.Join(tempDir, "class", "udc")
	if err := os.MkdirAll(classDir, 0o755); err != nil {
		t.Fatalf("mkdir class dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(classDir, "musb-hdrc.0"), nil, 0o644); err != nil {
		t.Fatalf("create controller entry: %v", err)
	}

	if err := resetUSBGadgetUDCPaths(udcPath, classDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(udcPath)
	if err != nil {
		t.Fatalf("read rebound udc file: %v", err)
	}
	if string(data) != "musb-hdrc.0" {
		t.Fatalf("expected rebound controller musb-hdrc.0, got %q", string(data))
	}
}

func TestResetUSBGadgetUDCPathsFailsWithoutController(t *testing.T) {
	tempDir := t.TempDir()
	udcPath := filepath.Join(tempDir, "gadget", "UDC")
	if err := os.MkdirAll(filepath.Dir(udcPath), 0o755); err != nil {
		t.Fatalf("mkdir udc dir: %v", err)
	}
	if err := os.WriteFile(udcPath, []byte("old-controller"), 0o644); err != nil {
		t.Fatalf("seed udc file: %v", err)
	}

	classDir := filepath.Join(tempDir, "class", "udc")
	if err := os.MkdirAll(classDir, 0o755); err != nil {
		t.Fatalf("mkdir class dir: %v", err)
	}

	if err := resetUSBGadgetUDCPaths(udcPath, classDir); err == nil {
		t.Fatalf("expected error when no UDC controllers are present")
	}
}
