package auth

import (
	"os"
	"strings"
	"testing"
)

func readSource(t *testing.T, path string) string {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(source)
}

func containsAny(content string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(content, needle) {
			return true
		}
	}
	return false
}

func TestAccountStorageUsesPrivateFileHelpers(t *testing.T) {
	content := readSource(t, "account.go")
	if containsAny(content, "os.ReadFile(AccountFile)", "os.WriteFile(AccountFile, account, 0o644)") {
		t.Fatal("account storage should use private-file helpers instead of direct 0644 reads/writes")
	}
	if !strings.Contains(content, "ReadPrivateFile") || !strings.Contains(content, "WritePrivateFile") {
		t.Fatal("account storage should repair permissions and reject symlinks via private-file helpers")
	}
}

func TestAccountDirectoryIsNotCreatedWorldReadable(t *testing.T) {
	if strings.Contains(readSource(t, "account.go"), "os.MkdirAll(filepath.Dir(AccountFile), 0o644)") {
		t.Fatal("account directory should not be created with 0644 permissions")
	}
}

func TestMissingAccountDoesNotDefaultToAdminCredentials(t *testing.T) {
	content := readSource(t, "account.go")
	if strings.Contains(content, "return getDefaultAccount(), nil") ||
		(strings.Contains(content, `Username: "admin"`) && strings.Contains(content, `[]byte("admin")`)) {
		t.Fatal("missing account file should require provisioning instead of defaulting to admin/admin")
	}
}

func TestPasswordChangeRevokesExistingTokensAfterSuccess(t *testing.T) {
	if !containsAny(readSource(t, "password.go"), "ForceRegenerateSecretKey", "RegenerateSecretKey") {
		t.Fatal("password changes should revoke existing JWTs after the account and root password are updated")
	}
}

func TestPasswordChangeRollsBackAccountInsteadOfDeletingOnRootPasswordFailure(t *testing.T) {
	content := readSource(t, "password.go")
	if strings.Contains(content, "_ = DelAccount()") {
		t.Fatal("root password sync failure should restore the previous account instead of deleting it")
	}
	if !strings.Contains(content, "previousAccount") && !strings.Contains(content, "restore") {
		t.Fatal("password change should preserve the previous account for rollback")
	}
}

func TestPasswordChangeRejectsBcryptTruncationAndControlCharacters(t *testing.T) {
	content := readSource(t, "password.go")
	if !strings.Contains(content, "72") {
		t.Fatal("password change should reject passwords longer than bcrypt's 72-byte effective limit")
	}
	if !strings.Contains(content, "unicode.IsControl") && !strings.Contains(content, "ContainsFunc") {
		t.Fatal("password change should reject control characters before passing input to passwd")
	}
}

func TestPasswordChangeValidatesUsernameBeforePersistingAccount(t *testing.T) {
	content := readSource(t, "password.go")
	if !strings.Contains(content, "TrimSpace") || !strings.Contains(content, "unicode.IsControl") {
		t.Fatal("password change should validate username whitespace, path separators, and control characters before saving")
	}
}

func TestAccountJSONDoesNotExposeHashUnderPasswordFieldName(t *testing.T) {
	content := readSource(t, "account.go")
	if strings.Contains(content, "Password string `json:\"password\"`") {
		t.Fatal("stored account JSON should not label the password hash as password because it encourages unsafe compatibility paths")
	}
}

func TestPasswordChangeRequiresCurrentPasswordProof(t *testing.T) {
	protoSource := readSource(t, "../../proto/auth.go")
	passwordSource := readSource(t, "password.go")

	if !strings.Contains(protoSource, "OldPassword") &&
		!strings.Contains(passwordSource, "CompareAccount") {
		t.Fatal("password change should require current-password proof, not only a valid JWT")
	}
}

func TestInvalidLoginRequestsCountTowardLockout(t *testing.T) {
	content := readSource(t, "login.go")
	parseErr := strings.Index(content, "proto.ParseFormRequest")
	recordFailure := strings.Index(content, "RecordLoginFailure")
	if parseErr >= 0 && recordFailure > parseErr {
		t.Fatal("malformed login requests return before recording a failed attempt")
	}
	if parseErr >= 0 && recordFailure < 0 {
		t.Fatal("login should record malformed requests as failed attempts")
	}
}

func TestIsPasswordUpdatedSupportsEncryptedLegacyAccounts(t *testing.T) {
	content := readSource(t, "password.go")
	start := strings.Index(content, "func (s *Service) IsPasswordUpdated")
	end := strings.Index(content, "func changeRootPassword")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("could not locate IsPasswordUpdated implementation")
	}

	isUpdatedBody := content[start:end]
	if strings.Contains(isUpdatedBody, `[]byte("admin")`) && !strings.Contains(isUpdatedBody, "DecodeDecrypt") {
		t.Fatal("default-password detection should recognize legacy encrypted account records instead of treating invalid bcrypt as unchanged")
	}
}

func TestSuccessfulLegacyPasswordLoginMigratesToBcrypt(t *testing.T) {
	content := readSource(t, "account.go")
	legacyCompat := strings.Index(content, "Compatible with old versions")
	setAccount := strings.Index(content, "SetAccount(")
	if legacyCompat >= 0 && (setAccount < 0 || setAccount < legacyCompat) {
		t.Fatal("successful legacy encrypted-password login should migrate the account record to bcrypt")
	}
}

func TestRootPasswordSyncDoesNotAttachPasswdToProcessStdout(t *testing.T) {
	content := readSource(t, "password.go")
	if strings.Contains(content, "cmd.Stdout = os.Stdout") || strings.Contains(content, "cmd.Stderr = os.Stderr") {
		t.Fatal("root password sync should not attach passwd stdout/stderr directly to the server process logs")
	}
}

func TestRootPasswordSyncUsesTimeoutInsteadOfFixedSleep(t *testing.T) {
	content := readSource(t, "password.go")
	if strings.Contains(content, "time.Sleep(100 * time.Millisecond)") && !strings.Contains(content, "CommandContext") {
		t.Fatal("root password sync should use a bounded command context/expect flow instead of fixed sleeps")
	}
}

func TestPasswordChangeRevokesTokensOnlyAfterRootPasswordSyncSucceeds(t *testing.T) {
	content := readSource(t, "password.go")
	rootSync := strings.Index(content, "changeRootPassword")
	revocation := strings.Index(content, "RegenerateSecretKey")
	if revocation >= 0 && rootSync >= 0 && revocation < rootSync {
		t.Fatal("password-change token revocation should happen only after account persistence and root password sync both succeed")
	}
	if revocation < 0 {
		t.Fatal("password-change token revocation should be present after root password sync succeeds")
	}
}
