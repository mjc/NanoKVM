package config

import "testing"

func TestForceRegenerateSecretKeyIgnoresLogoutRevocationFlag(t *testing.T) {
	instance.JWT.SecretKey = "old-secret"
	instance.JWT.RevokeTokensOnLogout = false

	ForceRegenerateSecretKey()

	if instance.JWT.SecretKey == "old-secret" || instance.JWT.SecretKey == "" {
		t.Fatalf("secret key was not force-regenerated: %q", instance.JWT.SecretKey)
	}
}

func TestRegenerateSecretKeyHonorsLogoutRevocationFlag(t *testing.T) {
	instance.JWT.SecretKey = "old-secret"
	instance.JWT.RevokeTokensOnLogout = false

	RegenerateSecretKey()

	if instance.JWT.SecretKey != "old-secret" {
		t.Fatalf("logout regeneration ignored RevokeTokensOnLogout=false: %q", instance.JWT.SecretKey)
	}
}
