package usb

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadAndClearString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "value")

	if err := WriteString(path, "  abc  \n"); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTrimmed(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc" {
		t.Fatalf("ReadTrimmed() = %q, want trimmed value", got)
	}

	if err := ClearString(path); err != nil {
		t.Fatal(err)
	}
	assertFile(t, path, "\n")
}

func TestEnsureFileCreatesAndPreservesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flag")

	if err := EnsureFile(path); err != nil {
		t.Fatal(err)
	}
	if !Exists(path) {
		t.Fatal("EnsureFile did not create file")
	}
	if err := WriteString(path, "keep"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureFile(path); err != nil {
		t.Fatal(err)
	}
	assertFile(t, path, "keep")
}

func TestRemoveIfExistsIgnoresMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")

	if err := RemoveIfExists(path); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveIfExistsReportsNonFileError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "child"), "")

	if err := RemoveIfExists(dir); err == nil {
		t.Fatal("RemoveIfExists succeeded on non-empty directory")
	}
}

func TestReadTrimmedReportsMissingFile(t *testing.T) {
	_, err := ReadTrimmed(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("ReadTrimmed succeeded on missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadTrimmed error = %v, want os.ErrNotExist", err)
	}
}
