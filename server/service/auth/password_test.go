package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"NanoKVM-Server/proto"
	"NanoKVM-Server/utils"
	"github.com/gin-gonic/gin"
	"github.com/mervick/aes-everywhere/go/aes256"
	"golang.org/x/crypto/bcrypt"
)

func TestChangePasswordRevokesTokensAfterSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var accountUsername string
	var accountPassword string
	previousGetAccount := getAccount
	getAccount = func() (*Account, error) {
		return &Account{Username: "admin", Password: hashPassword(t, "old-password")}, nil
	}
	t.Cleanup(func() {
		getAccount = previousGetAccount
	})

	previousSetAccount := setAccount
	setAccount = func(username string, hashedPassword string) error {
		accountUsername = username
		accountPassword = hashedPassword
		return nil
	}
	t.Cleanup(func() {
		setAccount = previousSetAccount
	})

	var rootPassword string
	previousChangeRootPassword := changeRootPassword
	changeRootPassword = func(password string) error {
		rootPassword = password
		return nil
	}
	t.Cleanup(func() {
		changeRootPassword = previousChangeRootPassword
	})

	var revoked bool
	previousRevokeTokens := revokeTokensAfterPasswordChange
	revokeTokensAfterPasswordChange = func() {
		revoked = true
	}
	t.Cleanup(func() {
		revokeTokensAfterPasswordChange = previousRevokeTokens
	})

	w := changePasswordRequest(t, proto.ChangePasswordReq{
		Username: "admin",
		Password: url.QueryEscape(aes256.Encrypt("changed-password", utils.SecretKey)),
	})

	assertAuthResponse(t, w, 0, "success")
	if accountUsername != "admin" {
		t.Fatalf("username = %q, want admin", accountUsername)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(accountPassword), []byte("changed-password")); err != nil {
		t.Fatalf("stored password does not match changed password: %v", err)
	}
	if rootPassword != "changed-password" {
		t.Fatalf("root password = %q, want changed-password", rootPassword)
	}
	if !revoked {
		t.Fatal("expected successful password change to revoke existing tokens")
	}
}

func TestChangePasswordDoesNotRevokeTokensForInvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	setPasswordChangeHooks(t, passwordChangeHooks{
		setAccount: func(username string, hashedPassword string) error {
			t.Fatal("setAccount should not be called for invalid password changes")
			return nil
		},
		changeRootPassword: func(password string) error {
			t.Fatal("changeRootPassword should not be called for invalid password changes")
			return nil
		},
		delAccount: func() error {
			t.Fatal("delAccount should not be called for invalid password changes")
			return nil
		},
		revokeTokens: func() {
			t.Fatal("tokens should not be revoked for invalid password changes")
		},
	})

	w := changePasswordRequest(t, proto.ChangePasswordReq{
		Username: "admin",
		Password: "",
	})

	assertAuthResponse(t, w, -1, "invalid parameters")
}

func TestChangePasswordDoesNotRevokeTokensForInvalidPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)

	setPasswordChangeHooks(t, passwordChangeHooks{
		setAccount: func(username string, hashedPassword string) error {
			t.Fatal("setAccount should not be called for invalid password changes")
			return nil
		},
		changeRootPassword: func(password string) error {
			t.Fatal("changeRootPassword should not be called for invalid password changes")
			return nil
		},
		delAccount: func() error {
			t.Fatal("delAccount should not be called for invalid password changes")
			return nil
		},
		revokeTokens: func() {
			t.Fatal("tokens should not be revoked for invalid password changes")
		},
	})

	w := changePasswordRequest(t, proto.ChangePasswordReq{
		Username: "admin",
		Password: "not-a-valid-encrypted-password",
	})

	assertAuthResponse(t, w, -2, "invalid password")
}

func TestChangePasswordDoesNotRevokeTokensWhenSavingPasswordFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var setAccountCalled bool
	setPasswordChangeHooks(t, passwordChangeHooks{
		getAccount: func() (*Account, error) {
			return &Account{Username: "admin", Password: hashPassword(t, "old-password")}, nil
		},
		setAccount: func(username string, hashedPassword string) error {
			setAccountCalled = true
			return errors.New("save failed")
		},
		changeRootPassword: func(password string) error {
			t.Fatal("changeRootPassword should not be called when saving the password fails")
			return nil
		},
		delAccount: func() error {
			t.Fatal("delAccount should not be called when saving the password fails")
			return nil
		},
		revokeTokens: func() {
			t.Fatal("tokens should not be revoked when saving the password fails")
		},
	})

	w := changePasswordRequest(t, proto.ChangePasswordReq{
		Username: "admin",
		Password: encryptedPassword("changed-password"),
	})

	assertAuthResponse(t, w, -4, "failed to save password")
	if !setAccountCalled {
		t.Fatal("expected setAccount to be called")
	}
}

func TestChangePasswordDoesNotRevokeTokensWhenRootPasswordChangeFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldPasswordHash := hashPassword(t, "old-password")
	var savedAccounts []Account
	setPasswordChangeHooks(t, passwordChangeHooks{
		getAccount: func() (*Account, error) {
			return &Account{Username: "admin", Password: oldPasswordHash}, nil
		},
		setAccount: func(username string, hashedPassword string) error {
			savedAccounts = append(savedAccounts, Account{Username: username, Password: hashedPassword})
			return nil
		},
		changeRootPassword: func(password string) error {
			return errors.New("root password update failed")
		},
		delAccount: func() error {
			t.Fatal("failed password changes must not remove the stored account")
			return nil
		},
		revokeTokens: func() {
			t.Fatal("tokens should not be revoked when the root password change fails")
		},
	})

	w := changePasswordRequest(t, proto.ChangePasswordReq{
		Username: "admin",
		Password: encryptedPassword("changed-password"),
	})

	assertAuthResponse(t, w, -5, "failed to change password")
	if len(savedAccounts) != 2 {
		t.Fatalf("saved accounts = %d, want new account save and previous account restore", len(savedAccounts))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(savedAccounts[0].Password), []byte("changed-password")); err != nil {
		t.Fatalf("first saved account does not contain changed password: %v", err)
	}
	if savedAccounts[1].Username != "admin" || savedAccounts[1].Password != oldPasswordHash {
		t.Fatalf("restored account = %+v, want previous admin account", savedAccounts[1])
	}
}

func TestChangePasswordRejectsUnsafePasswordsBeforeMutatingAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tt := range []struct {
		name     string
		password string
	}{
		{name: "leading newline", password: "\npassword-with-newline"},
		{name: "embedded newline", password: "password\nwith-newline"},
		{name: "tab character", password: "password\twith-tab"},
		{name: "null byte", password: "password\x00with-null"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertPasswordChangeRejectedBeforeMutation(t, proto.ChangePasswordReq{
				Username: "mjc",
				Password: encryptedPassword(tt.password),
			})
		})
	}
}

func TestChangePasswordRejectsUnsafeUsernamesBeforeMutatingAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, username := range []string{
		"admin ",
		" admin",
		"admin\n",
		"../../etc/passwd",
		"<script>alert(1)</script>",
	} {
		t.Run(username, func(t *testing.T) {
			assertPasswordChangeRejectedBeforeMutation(t, proto.ChangePasswordReq{
				Username: username,
				Password: encryptedPassword("long-random-passphrase-2026!"),
			})
		})
	}
}

func TestChangePasswordRejectsBcryptTruncationBoundaryBeforeHashing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tt := range []struct {
		name     string
		password string
	}{
		{name: "seventy three ascii bytes", password: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{name: "shared first seventy two bytes", password: "same-prefix-same-prefix-same-prefix-same-prefix-same-prefix-same-prefix-1"},
		{name: "same first seventy two bytes but different suffix", password: "same-prefix-same-prefix-same-prefix-same-prefix-same-prefix-same-prefix-2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := changePasswordRequest(t, proto.ChangePasswordReq{
				Username: "admin",
				Password: encryptedPassword(tt.password),
			})

			assertAuthResponse(t, w, -2, "invalid password")
		})
	}
}

func TestIsDefaultPasswordChanged(t *testing.T) {
	bcryptAdmin := hashPassword(t, "admin")
	bcryptChanged := hashPassword(t, "changed-password")
	legacyAdmin := encryptedPassword("admin")
	legacyChanged := encryptedPassword("changed-password")

	for _, tt := range []struct {
		name    string
		account *Account
		want    bool
	}{
		{name: "missing account", account: nil, want: false},
		{name: "bcrypt default", account: &Account{Username: "admin", Password: bcryptAdmin}, want: false},
		{name: "bcrypt changed", account: &Account{Username: "admin", Password: bcryptChanged}, want: true},
		{name: "legacy default", account: &Account{Username: "admin", Password: legacyAdmin}, want: false},
		{name: "legacy changed", account: &Account{Username: "admin", Password: legacyChanged}, want: true},
		{name: "malformed legacy", account: &Account{Username: "admin", Password: "not-a-valid-legacy-value"}, want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDefaultPasswordChanged(tt.account); got != tt.want {
				t.Fatalf("changed = %t, want %t", got, tt.want)
			}
		})
	}
}

type passwordChangeHooks struct {
	getAccount         func() (*Account, error)
	setAccount         func(username string, hashedPassword string) error
	changeRootPassword func(password string) error
	delAccount         func() error
	revokeTokens       func()
}

func setPasswordChangeHooks(t *testing.T, hooks passwordChangeHooks) {
	t.Helper()

	previousSetAccount := setAccount
	previousGetAccount := getAccount
	previousChangeRootPassword := changeRootPassword
	previousDelAccount := delAccount
	previousRevokeTokens := revokeTokensAfterPasswordChange

	if hooks.getAccount != nil {
		getAccount = hooks.getAccount
	}
	if hooks.setAccount != nil {
		setAccount = hooks.setAccount
	}
	if hooks.changeRootPassword != nil {
		changeRootPassword = hooks.changeRootPassword
	}
	if hooks.delAccount != nil {
		delAccount = hooks.delAccount
	}
	if hooks.revokeTokens != nil {
		revokeTokensAfterPasswordChange = hooks.revokeTokens
	}

	t.Cleanup(func() {
		setAccount = previousSetAccount
		getAccount = previousGetAccount
		changeRootPassword = previousChangeRootPassword
		delAccount = previousDelAccount
		revokeTokensAfterPasswordChange = previousRevokeTokens
	})
}

func changePasswordRequest(t *testing.T, payload proto.ChangePasswordReq) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/auth/password", NewService().ChangePassword)
	router.ServeHTTP(w, req)

	return w
}

func assertPasswordChangeRejectedBeforeMutation(t *testing.T, payload proto.ChangePasswordReq) {
	t.Helper()

	setPasswordChangeHooks(t, passwordChangeHooks{
		setAccount: func(username string, hashedPassword string) error {
			t.Fatalf("setAccount should not be called for unsafe password changes, username=%q", username)
			return nil
		},
		changeRootPassword: func(password string) error {
			t.Fatal("changeRootPassword should not be called for unsafe password changes")
			return nil
		},
		delAccount: func() error {
			t.Fatal("delAccount should not be called for unsafe password changes")
			return nil
		},
		revokeTokens: func() {
			t.Fatal("tokens should not be revoked for unsafe password changes")
		},
	})

	w := changePasswordRequest(t, payload)

	assertAuthResponse(t, w, -2, "invalid password")
}

func encryptedPassword(password string) string {
	return url.QueryEscape(aes256.Encrypt(password, utils.SecretKey))
}

func assertAuthResponse(t *testing.T, w *httptest.ResponseRecorder, wantCode int, wantMsg string) {
	t.Helper()

	if w.Code != http.StatusOK {
		t.Fatalf("http status = %d, want %d", w.Code, http.StatusOK)
	}

	var response proto.Response
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Code != wantCode || response.Msg != wantMsg {
		t.Fatalf("response = (%d, %q), want (%d, %q); body=%s", response.Code, response.Msg, wantCode, wantMsg, w.Body.String())
	}
}

func hashPassword(t *testing.T, password string) string {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	return string(hash)
}
