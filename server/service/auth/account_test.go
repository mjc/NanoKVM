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

func TestGetAccountRepairsLegacyAccountDirectoryPermissions(t *testing.T) {
	accountPath := setTemporaryAccountFile(t)

	if err := os.MkdirAll(filepath.Dir(accountPath), 0o755); err != nil {
		t.Fatalf("create legacy account directory: %v", err)
	}
	if err := os.WriteFile(accountPath, []byte(`{"username":"admin","password":"legacy-hash"}`), 0o600); err != nil {
		t.Fatalf("create account file: %v", err)
	}

	if _, err := GetAccount(); err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}

	info, err := os.Stat(filepath.Dir(accountPath))
	if err != nil {
		t.Fatalf("stat account directory: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o700); got != want {
		t.Fatalf("legacy account directory permissions after read = %v, want %v", got, want)
	}
}

func TestGetAccountRepairsLegacyAccountFilePermissions(t *testing.T) {
	accountPath := setTemporaryAccountFile(t)

	if err := os.MkdirAll(filepath.Dir(accountPath), 0o700); err != nil {
		t.Fatalf("create account directory: %v", err)
	}
	if err := os.WriteFile(accountPath, []byte(`{"username":"admin","password":"legacy-hash"}`), 0o644); err != nil {
		t.Fatalf("create legacy account file: %v", err)
	}

	if _, err := GetAccount(); err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}

	info, err := os.Stat(accountPath)
	if err != nil {
		t.Fatalf("stat account file: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Fatalf("legacy account file permissions after read = %v, want %v", got, want)
	}
}

func TestGetAccountRejectsSymlinkAccountFile(t *testing.T) {
	accountPath := setTemporaryAccountFile(t)
	targetPath := filepath.Join(t.TempDir(), "target-account")

	if err := os.MkdirAll(filepath.Dir(accountPath), 0o700); err != nil {
		t.Fatalf("create account directory: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte(`{"username":"admin","password":"legacy-hash"}`), 0o600); err != nil {
		t.Fatalf("create target file: %v", err)
	}
	if err := os.Symlink(targetPath, accountPath); err != nil {
		t.Fatalf("create account symlink: %v", err)
	}

	if _, err := GetAccount(); err == nil {
		t.Fatal("expected GetAccount to reject symlink account file")
	}
}

func TestSetAccountRejectsSymlinkAccountFile(t *testing.T) {
	accountPath := setTemporaryAccountFile(t)
	targetPath := filepath.Join(t.TempDir(), "target-account")
	targetContent := []byte("do not overwrite")

	if err := os.MkdirAll(filepath.Dir(accountPath), 0o700); err != nil {
		t.Fatalf("create account directory: %v", err)
	}
	if err := os.WriteFile(targetPath, targetContent, 0o600); err != nil {
		t.Fatalf("create target file: %v", err)
	}
	if err := os.Symlink(targetPath, accountPath); err != nil {
		t.Fatalf("create account symlink: %v", err)
	}

	if err := SetAccount("admin", hashPassword(t, "changed-password")); err == nil {
		t.Fatal("expected SetAccount to reject symlink account file")
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target file: %v", err)
	}
	if string(content) != string(targetContent) {
		t.Fatalf("symlink target was overwritten: %q", content)
	}

	info, err := os.Lstat(accountPath)
	if err != nil {
		t.Fatalf("lstat account path: %v", err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Fatal("account symlink should not be replaced")
	}
}

func TestDelAccountIgnoresMissingAccountFile(t *testing.T) {
	setTemporaryAccountFile(t)

	if err := DelAccount(); err != nil {
		t.Fatalf("DelAccount should ignore missing account file: %v", err)
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
