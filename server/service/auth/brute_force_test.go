package auth

import (
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
