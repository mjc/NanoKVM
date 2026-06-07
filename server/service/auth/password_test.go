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

	// These tests replace package-level hooks and must not run with t.Parallel().
	oldPasswordHash := hashPassword(t, "old-password")
	previousGetAccount := getAccount
	getAccount = func() (*Account, error) {
		return &Account{Username: "admin", Password: oldPasswordHash}, nil
	}
	t.Cleanup(func() {
		getAccount = previousGetAccount
	})

	var accountUsername string
	var accountPassword string
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

func TestChangePasswordRestoresPreviousAccountWhenRootPasswordChangeFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// This test replaces package-level hooks and must not run with t.Parallel().
	oldPasswordHash := hashPassword(t, "old-password")
	previousGetAccount := getAccount
	getAccount = func() (*Account, error) {
		return &Account{Username: "admin", Password: oldPasswordHash}, nil
	}
	t.Cleanup(func() {
		getAccount = previousGetAccount
	})

	var savedAccounts []Account
	previousSetAccount := setAccount
	setAccount = func(username string, hashedPassword string) error {
		savedAccounts = append(savedAccounts, Account{Username: username, Password: hashedPassword})
		return nil
	}
	t.Cleanup(func() {
		setAccount = previousSetAccount
	})

	previousChangeRootPassword := changeRootPassword
	changeRootPassword = func(password string) error {
		return errors.New("root password update failed")
	}
	t.Cleanup(func() {
		changeRootPassword = previousChangeRootPassword
	})

	previousDelAccount := delAccount
	delAccount = func() error {
		t.Fatal("failed password changes must not delete the stored account")
		return nil
	}
	t.Cleanup(func() {
		delAccount = previousDelAccount
	})

	previousRevokeTokens := revokeTokensAfterPasswordChange
	revokeTokensAfterPasswordChange = func() {
		t.Fatal("tokens should not be revoked when root password change fails")
	}
	t.Cleanup(func() {
		revokeTokensAfterPasswordChange = previousRevokeTokens
	})

	w := changePasswordRequest(t, proto.ChangePasswordReq{
		Username: "admin",
		Password: url.QueryEscape(aes256.Encrypt("changed-password", utils.SecretKey)),
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

func TestChangePasswordReportsRestoreFailureWhenRootPasswordChangeFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// This test replaces package-level hooks and must not run with t.Parallel().
	oldPasswordHash := hashPassword(t, "old-password")
	previousGetAccount := getAccount
	getAccount = func() (*Account, error) {
		return &Account{Username: "admin", Password: oldPasswordHash}, nil
	}
	t.Cleanup(func() {
		getAccount = previousGetAccount
	})

	var saves int
	previousSetAccount := setAccount
	setAccount = func(username string, hashedPassword string) error {
		saves++
		if saves == 2 {
			return errors.New("restore failed")
		}
		return nil
	}
	t.Cleanup(func() {
		setAccount = previousSetAccount
	})

	previousChangeRootPassword := changeRootPassword
	changeRootPassword = func(password string) error {
		return errors.New("root password update failed")
	}
	t.Cleanup(func() {
		changeRootPassword = previousChangeRootPassword
	})

	previousRevokeTokens := revokeTokensAfterPasswordChange
	revokeTokensAfterPasswordChange = func() {
		t.Fatal("tokens should not be revoked when root password change fails")
	}
	t.Cleanup(func() {
		revokeTokensAfterPasswordChange = previousRevokeTokens
	})

	w := changePasswordRequest(t, proto.ChangePasswordReq{
		Username: "admin",
		Password: url.QueryEscape(aes256.Encrypt("changed-password", utils.SecretKey)),
	})

	assertAuthResponse(t, w, -6, "failed to restore password")
}

func TestIsDefaultPasswordChanged(t *testing.T) {
	bcryptAdmin := hashPassword(t, "admin")
	bcryptChanged := hashPassword(t, "changed-password")
	legacyAdmin := url.QueryEscape(aes256.Encrypt("admin", utils.SecretKey))
	legacyChanged := url.QueryEscape(aes256.Encrypt("changed-password", utils.SecretKey))

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
		{name: "malformed bcrypt", account: &Account{Username: "admin", Password: "$2a$not-a-valid-bcrypt-hash"}, want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDefaultPasswordChanged(tt.account); got != tt.want {
				t.Fatalf("changed = %t, want %t", got, tt.want)
			}
		})
	}
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
