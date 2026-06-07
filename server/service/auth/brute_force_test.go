package auth

import (
	"strings"
	"sync"
	"testing"

	"NanoKVM-Server/config"
)

func withLoginAttemptState(t *testing.T, lockoutDuration int, maxFailures int) {
	t.Helper()

	conf := config.GetInstance()
	previousLockoutDuration := conf.Security.LoginLockoutDuration
	previousMaxFailures := conf.Security.LoginMaxFailures
	conf.Security.LoginLockoutDuration = lockoutDuration
	conf.Security.LoginMaxFailures = maxFailures
	t.Cleanup(func() {
		conf.Security.LoginLockoutDuration = previousLockoutDuration
		conf.Security.LoginMaxFailures = previousMaxFailures
	})

	previousAttempts := loginAttempts
	previousCleanupOnce := cleanupOnce
	loginAttempts = make(map[string]*loginAttempt)
	cleanupOnce = sync.Once{}
	t.Cleanup(func() {
		loginAttempts = previousAttempts
		cleanupOnce = previousCleanupOnce
	})
}

func TestRecordLoginFailureReportsLockoutAtThreshold(t *testing.T) {
	withLoginAttemptState(t, 60, 3)
	conf := config.GetInstance()

	clientIP := "192.0.2.10"
	for i := 1; i < conf.Security.LoginMaxFailures; i++ {
		if locked, _, _ := RecordLoginFailure(clientIP); locked {
			t.Fatalf("failure %d locked too early", i)
		}
	}

	if locked, code, msg := RecordLoginFailure(clientIP); !locked {
		t.Fatalf("threshold failure did not report lockout, code=%d msg=%q", code, msg)
	}
}

func TestRecordLoginFailureDoesNotClearExistingAttemptsAtCapacity(t *testing.T) {
	withLoginAttemptState(t, 60, 5)
	loginAttempts = make(map[string]*loginAttempt, maxLoginAttemptsRecords)
	for i := 0; i < maxLoginAttemptsRecords; i++ {
		loginAttempts[string(rune(i+1))] = &loginAttempt{failures: 1}
	}

	RecordLoginFailure("198.51.100.200")

	if len(loginAttempts) < maxLoginAttemptsRecords {
		t.Fatalf("login attempt capacity handling cleared existing protections; remaining records = %d", len(loginAttempts))
	}
}

func TestLoginAttemptKeyIncludesUsernameDimension(t *testing.T) {
	content := readSource(t, "login.go")
	if strings.Contains(content, "CheckLoginAttempt(clientIP)") ||
		strings.Contains(content, "RecordLoginFailure(clientIP)") ||
		strings.Contains(content, "ClearLoginAttempt(clientIP)") {
		t.Fatal("login lockout accounting should include the attempted username, not only the client IP")
	}
}

func TestGetClientIPDoesNotTrustProxyHeadersWithoutConfiguration(t *testing.T) {
	content := readSource(t, "brute_force.go")
	if strings.Contains(content, "c.ClientIP()") && !strings.Contains(content, "TrustedProxies") {
		t.Fatal("login rate limiting should not trust forwarded client IP headers unless trusted proxies are configured")
	}
}

func TestLoginAttemptCleanupTickerCanStop(t *testing.T) {
	content := readSource(t, "brute_force.go")
	if strings.Contains(content, "time.NewTicker(cleanupInterval)") &&
		!strings.Contains(content, "Stop()") {
		t.Fatal("login-attempt cleanup ticker should be stoppable so tests/reloads do not leak background goroutines")
	}
}

func TestLoginAttemptCleanupWindowIsConfigured(t *testing.T) {
	if strings.Contains(readSource(t, "brute_force.go"), "30*time.Minute") {
		t.Fatal("login-attempt cleanup window should be configurable instead of hard-coded independently from lockout policy")
	}
}
