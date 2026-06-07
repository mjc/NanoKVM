package storage

import "testing"

func TestSafeImagePathRejectsTraversal(t *testing.T) {
	if path, err := safeImagePath("../etc/passwd"); err == nil {
		t.Fatalf("expected traversal path to be rejected, got %q", path)
	}
}

func TestSafeImagePathAllowsDataImage(t *testing.T) {
	path, err := safeImagePath("/data/install.iso")
	if err != nil {
		t.Fatalf("expected /data image to be accepted: %v", err)
	}
	if path != "/data/install.iso" {
		t.Fatalf("unexpected image path %q", path)
	}
}
