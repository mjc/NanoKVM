package auth

import (
	"crypto/sha256"
	"strings"
	"sync"
	"time"

	"NanoKVM-Server/config"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type loginAttempt struct {
	failures   int
	lastFailed time.Time
	lockoutEnd time.Time
}

const (
	maxLoginAttemptsRecords = 3000
	cleanupInterval         = 6 * time.Hour
)

var (
	loginAttempts = make(map[string]*loginAttempt)
	loginMutex    sync.Mutex
	cleanupOnce   sync.Once
)

// startCleanupRoutine starts a background routine to clean up memory
func startCleanupRoutine() {
	conf := config.GetInstance()
	if conf.Security.LoginLockoutDuration <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			loginMutex.Lock()
			now := time.Now()
			for ip, attempt := range loginAttempts {
				// Cleanup records after the configured lockout window elapses.
				if (!attempt.lockoutEnd.IsZero() && now.After(attempt.lockoutEnd)) ||
					(attempt.lockoutEnd.IsZero() && now.Sub(attempt.lastFailed) > time.Duration(conf.Security.LoginLockoutDuration)*time.Second) {
					delete(loginAttempts, ip)
				}
			}
			loginMutex.Unlock()
		}
	}()
}

// GetClientIP gets a reliable real IP
func GetClientIP(c *gin.Context) string {
	ip := c.RemoteIP()
	return ip
}

func LoginAttemptKey(clientIP string, username string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(username)))
	return clientIP + ":" + string(sum[:])
}

// CheckLoginAttempt checks if a login attempt is allowed based on brute-force protection rules.
// Returning true means the IP/System is locked out, and an error string and error code are returned.
func CheckLoginAttempt(clientIP string) (bool, int, string) {
	conf := config.GetInstance()
	if conf.Security.LoginLockoutDuration <= 0 {
		return false, 0, ""
	}

	cleanupOnce.Do(startCleanupRoutine)

	loginMutex.Lock()
	defer loginMutex.Unlock()

	if attempt, exists := loginAttempts[clientIP]; exists {
		if time.Now().Before(attempt.lockoutEnd) {
			log.Debugf("login blocked for IP %s: account locked due to too many failed attempts (until %s)", clientIP, attempt.lockoutEnd)
			return true, -5, "Account locked due to too many failed attempts, please try again later"
		}

		// If lockout has elapsed, then we reset the failures and lockoutEnd.
		if !attempt.lockoutEnd.IsZero() {
			attempt.failures = 0
			attempt.lockoutEnd = time.Time{}
		}
	}

	return false, 0, ""
}

// RecordLoginFailure records a failed login attempt for the given IP address.
func RecordLoginFailure(clientIP string) (bool, int, string) {
	conf := config.GetInstance()
	if conf.Security.LoginLockoutDuration <= 0 {
		return false, 0, ""
	}

	cleanupOnce.Do(startCleanupRoutine)

	loginMutex.Lock()
	defer loginMutex.Unlock()

	attempt, exists := loginAttempts[clientIP]
	if !exists {
		// When the record pool is full, clear the records instead of global lockout to prevent DDoS
		if len(loginAttempts) >= maxLoginAttemptsRecords {
			log.Warn("Login attempt records reached maximum limit, refusing to add new record")
			return true, -5, "Account locked due to too many failed attempts, please try again later"
		}
		attempt = &loginAttempt{}
		loginAttempts[clientIP] = attempt
	}

	now := time.Now()
	// Failure time window: if it has been a long time since the last failure
	// (e.g., beyond the lockoutDuration window), reset the failure count
	if !attempt.lastFailed.IsZero() && now.Sub(attempt.lastFailed) > time.Duration(conf.Security.LoginLockoutDuration)*time.Second {
		attempt.failures = 0
	}

	attempt.failures++
	attempt.lastFailed = now

	// Reach the failure limit, lock out
	if attempt.failures >= conf.Security.LoginMaxFailures {
		attempt.lockoutEnd = now.Add(time.Duration(conf.Security.LoginLockoutDuration) * time.Second)
		log.Debugf("login failures reached threshold for IP %s, locking out until %s", clientIP, attempt.lockoutEnd)
		return true, -5, "Account locked due to too many failed attempts, please try again later"
	}

	return false, 0, ""
}

// ClearLoginAttempt clears the failed login attempt record for an IP upon successful login.
func ClearLoginAttempt(clientIP string) {
	conf := config.GetInstance()
	if conf.Security.LoginLockoutDuration <= 0 {
		return
	}

	loginMutex.Lock()
	defer loginMutex.Unlock()

	delete(loginAttempts, clientIP)
}
