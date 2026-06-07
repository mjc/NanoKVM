package config

import (
	"strings"
	"testing"
)

func TestServerConfigWritesOwnerOnlyBecauseItContainsSecrets(t *testing.T) {
	content := readSource(t, "file.go")
	if strings.Contains(content, "os.WriteFile(ConfigurationFile, data, 0644)") {
		t.Fatal("config.Write stores server.yaml as 0644 even though it may contain JWT and TURN secrets")
	}
	if !containsAny(content, "0600", "WritePrivateFile") {
		t.Fatal("config.Write should persist server.yaml as an owner-only file")
	}
}
func TestServerConfigReadRepairsSecretFilePermissions(t *testing.T) {
	content := readSource(t, "file.go")
	if strings.Contains(content, "os.ReadFile(ConfigurationFile)") && !strings.Contains(content, "ReadPrivateFile") {
		t.Fatal("config.Read should repair permissions and reject symlinks before reading server.yaml secrets")
	}
}
func TestServerConfigCreateUsesOwnerOnlyPermissions(t *testing.T) {
	content := readSource(t, "config.go")
	if strings.Contains(content, `os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644`) {
		t.Fatal("config.create creates server.yaml as 0644 even though it may contain secrets")
	}
	if !containsAny(content, "0o600", "WritePrivateFile") {
		t.Fatal("config.create should create server.yaml as an owner-only file")
	}
}
func TestConfigValidationDoesNotDeleteSecretBearingConfig(t *testing.T) {
	content := readSource(t, "config.go")
	if strings.Contains(content, `os.Remove("/etc/kvm/server.yaml")`) {
		t.Fatal("config validation should not delete server.yaml because it may contain JWT and TURN secrets")
	}
}
func TestDefaultConfigEnablesLoginLockout(t *testing.T) {
	if defaultConfig.Security.LoginLockoutDuration <= 0 {
		t.Fatal("default config should enable login lockout protection")
	}
	if defaultConfig.Security.LoginMaxFailures <= 0 {
		t.Fatal("default config should set a positive login failure threshold")
	}
}
func TestDefaultConfigUsesHTTPSControlPlane(t *testing.T) {
	if defaultConfig.Proto != "https" {
		t.Fatalf("default config should serve the authenticated control plane over HTTPS, got %q", defaultConfig.Proto)
	}
}
func TestDefaultJWTDurationIsShortLived(t *testing.T) {
	const maxSessionSeconds = 24 * 60 * 60
	if defaultConfig.JWT.RefreshTokenDuration > maxSessionSeconds {
		t.Fatalf("default JWT lifetime should be at most %d seconds, got %d", maxSessionSeconds, defaultConfig.JWT.RefreshTokenDuration)
	}
}
func TestSecretKeyGenerationDoesNotFallBackToPredictableTime(t *testing.T) {
	if containsAny(readSource(t, "jwt.go"), "time.Now", "UnixNano") {
		t.Fatal("JWT/internal-token secret generation should fail closed instead of falling back to predictable time-derived material")
	}
}
func TestJWTSecretRotationPersistsAcrossRestart(t *testing.T) {
	content := readSource(t, "jwt.go")
	if strings.Contains(content, "func RegenerateSecretKey") &&
		strings.Contains(content, "instance.JWT.SecretKey = generateRandomSecretKey()") &&
		!strings.Contains(content, "Write(instance)") &&
		!strings.Contains(content, "WritePrivateFile") {
		t.Fatal("JWT secret rotation should persist the new signing key so revoked sessions do not revive after restart")
	}
}
func TestGeneratedDefaultJWTSecretPersistsAcrossRestart(t *testing.T) {
	content := readSource(t, "default.go")
	if strings.Contains(content, "instance.JWT.SecretKey = generateRandomSecretKey()") &&
		!strings.Contains(content, "Write(&instance)") &&
		!strings.Contains(content, "WritePrivateFile") {
		t.Fatal("generated default JWT secret should be persisted so sessions and revocation semantics survive restart predictably")
	}
}
func TestConfigYAMLDecodeRejectsUnknownSecretFields(t *testing.T) {
	content := readSource(t, "file.go")
	if strings.Contains(content, "yaml.Unmarshal(data, &conf)") && !strings.Contains(content, "KnownFields(true)") {
		t.Fatal("config YAML decoding should reject unknown fields so misspelled security settings do not silently disable protections")
	}
}
