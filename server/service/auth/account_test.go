package auth

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestSetAccountCreatesPrivateSearchableAccountDirectory(t *testing.T) {
	accountPath := setTemporaryAccountFile(t)

	if err := SetAccount("admin", hashPassword(t, "changed-password")); err != nil {
		t.Fatalf("SetAccount failed: %v", err)
	}

	dirInfo, err := os.Stat(filepath.Dir(accountPath))
	if err != nil {
		t.Fatalf("stat account directory: %v", err)
	}
	if got, want := dirInfo.Mode().Perm(), fs.FileMode(0o700); got != want {
		t.Fatalf("account directory permissions = %v, want %v", got, want)
	}
}

func TestSetAccountCreatesOwnerOnlyAccountFile(t *testing.T) {
	accountPath := setTemporaryAccountFile(t)

	if err := SetAccount("admin", hashPassword(t, "changed-password")); err != nil {
		t.Fatalf("SetAccount failed: %v", err)
	}

	info, err := os.Stat(accountPath)
	if err != nil {
		t.Fatalf("stat account file: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Fatalf("account file permissions = %v, want %v", got, want)
	}
}

func TestSetAccountRepairsLegacyAccountDirectoryPermissions(t *testing.T) {
	accountPath := setTemporaryAccountFile(t)

	if err := os.MkdirAll(filepath.Dir(accountPath), 0o755); err != nil {
		t.Fatalf("create legacy account directory: %v", err)
	}

	if err := SetAccount("admin", hashPassword(t, "changed-password")); err != nil {
		t.Fatalf("SetAccount failed: %v", err)
	}

	info, err := os.Stat(filepath.Dir(accountPath))
	if err != nil {
		t.Fatalf("stat account directory: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o700); got != want {
		t.Fatalf("legacy account directory permissions = %v, want %v", got, want)
	}
}

func TestSetAccountRepairsLegacyAccountFilePermissions(t *testing.T) {
	accountPath := setTemporaryAccountFile(t)

	if err := os.MkdirAll(filepath.Dir(accountPath), 0o700); err != nil {
		t.Fatalf("create account directory: %v", err)
	}
	if err := os.WriteFile(accountPath, []byte(`{"username":"admin","password":"legacy-hash"}`), 0o644); err != nil {
		t.Fatalf("create legacy account file: %v", err)
	}

	if err := SetAccount("admin", hashPassword(t, "changed-password")); err != nil {
		t.Fatalf("SetAccount failed: %v", err)
	}

	info, err := os.Stat(accountPath)
	if err != nil {
		t.Fatalf("stat account file: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Fatalf("legacy account file permissions = %v, want %v", got, want)
	}
}

func setTemporaryAccountFile(t *testing.T) string {
	t.Helper()

	previousAccountFile := accountFile
	accountPath := filepath.Join(t.TempDir(), "etc", "kvm", "pwd")
	accountFile = accountPath
	t.Cleanup(func() {
		accountFile = previousAccountFile
	})

	return accountPath
}
