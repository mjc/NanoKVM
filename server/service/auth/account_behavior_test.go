package auth

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAccountJSONStoresHashOutsidePasswordField(t *testing.T) {
	account := Account{Username: "admin", HashedPassword: "$2a$10$hash"}

	data, err := json.Marshal(account)
	if err != nil {
		t.Fatalf("marshal account: %v", err)
	}

	encoded := string(data)
	if strings.Contains(encoded, `"password"`) {
		t.Fatalf("account JSON exposed a password field: %s", encoded)
	}
	if !strings.Contains(encoded, `"hashedPassword"`) {
		t.Fatalf("account JSON did not expose hashedPassword: %s", encoded)
	}
}

func TestLoginAttemptKeySeparatesUsersAtSameAddress(t *testing.T) {
	adminKey := LoginAttemptKey("192.0.2.10", "admin")
	userKey := LoginAttemptKey("192.0.2.10", "user")

	if adminKey == userKey {
		t.Fatal("login attempt keys should include username as well as client IP")
	}
	if adminKey == "192.0.2.10" || userKey == "192.0.2.10" {
		t.Fatal("login attempt keys should not be the raw client IP")
	}
}
