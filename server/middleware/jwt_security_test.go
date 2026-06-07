package middleware

import (
	"testing"
	"time"

	"NanoKVM-Server/config"
	"github.com/golang-jwt/jwt/v5"
)

func withJWTConfig(t *testing.T, secret string, duration uint64) {
	t.Helper()

	conf := config.GetInstance()
	previousSecret := conf.JWT.SecretKey
	previousDuration := conf.JWT.RefreshTokenDuration
	conf.JWT.SecretKey = secret
	conf.JWT.RefreshTokenDuration = duration
	t.Cleanup(func() {
		conf.JWT.SecretKey = previousSecret
		conf.JWT.RefreshTokenDuration = previousDuration
	})
}

func signedToken(t *testing.T, method jwt.SigningMethod, claims Token) string {
	t.Helper()

	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(config.GetInstance().JWT.SecretKey))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func expectGenerateJWTError(t *testing.T, username string, reason string) {
	t.Helper()

	if token, err := GenerateJWT(username); err == nil {
		t.Fatalf("expected GenerateJWT to reject %s, got token %q", reason, token)
	}
}

func expectParseJWTError(t *testing.T, token string, reason string) {
	t.Helper()

	if parsed, err := ParseJWT(token); err == nil {
		t.Fatalf("expected ParseJWT to reject %s, got %+v", reason, parsed)
	}
}

func expiringClaims(username string) Token {
	return Token{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
}

func TestParseJWTRejectsUnexpectedHMACSigningMethod(t *testing.T) {
	withJWTConfig(t, "test-secret", 3600)

	token := signedToken(t, jwt.SigningMethodHS384, expiringClaims("admin"))

	expectParseJWTError(t, token, "tokens not signed with HS256")
}

func TestGenerateJWTIncludesIssuedAtAndNotBeforeClaims(t *testing.T) {
	withJWTConfig(t, "test-secret", 3600)

	token, err := GenerateJWT("admin")
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}
	claims, err := ParseJWT(token)
	if err != nil {
		t.Fatalf("ParseJWT failed: %v", err)
	}
	if claims.IssuedAt == nil {
		t.Fatal("JWT should include an issued-at claim")
	}
	if claims.NotBefore == nil {
		t.Fatal("JWT should include a not-before claim")
	}
}

func TestGenerateJWTRejectsEmptySigningSecret(t *testing.T) {
	withJWTConfig(t, "", 3600)

	expectGenerateJWTError(t, "admin", "an empty signing secret")
}

func TestParseJWTRejectsEmptyUsernameClaim(t *testing.T) {
	withJWTConfig(t, "test-secret", 3600)
	token := signedToken(t, jwt.SigningMethodHS256, expiringClaims(""))

	expectParseJWTError(t, token, "empty username claim")
}

func TestGenerateJWTRejectsEmptyUsername(t *testing.T) {
	withJWTConfig(t, "test-secret", 3600)

	expectGenerateJWTError(t, "", "empty username")
}

func TestGenerateJWTUsesRegisteredSubjectClaim(t *testing.T) {
	withJWTConfig(t, "test-secret", 3600)

	token, err := GenerateJWT("admin")
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}
	claims, err := ParseJWT(token)
	if err != nil {
		t.Fatalf("ParseJWT failed: %v", err)
	}
	if claims.Subject != "admin" {
		t.Fatalf("JWT subject should identify the authenticated account, got %q", claims.Subject)
	}
}

func TestParseJWTRejectsMissingExpiration(t *testing.T) {
	withJWTConfig(t, "test-secret", 3600)

	claims := Token{
		Username: "admin",
	}
	token := signedToken(t, jwt.SigningMethodHS256, claims)

	expectParseJWTError(t, token, "token without expiration")
}

func TestGenerateJWTRejectsZeroDuration(t *testing.T) {
	withJWTConfig(t, "test-secret", 0)

	expectGenerateJWTError(t, "admin", "zero refresh duration")
}

func TestGenerateJWTRejectsExcessiveDuration(t *testing.T) {
	withJWTConfig(t, "test-secret", 10*365*24*60*60)

	expectGenerateJWTError(t, "admin", "excessive refresh duration")
}

func TestParseJWTRejectsFutureIssuedAt(t *testing.T) {
	withJWTConfig(t, "test-secret", 3600)

	claims := Token{
		Username: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := signedToken(t, jwt.SigningMethodHS256, claims)

	expectParseJWTError(t, token, "token with future issued-at")
}

func TestParseJWTRejectsMissingIssuedAt(t *testing.T) {
	withJWTConfig(t, "test-secret", 3600)

	claims := Token{
		Username: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	}
	token := signedToken(t, jwt.SigningMethodHS256, claims)

	expectParseJWTError(t, token, "token without issued-at")
}

func TestParseJWTRejectsMissingNotBefore(t *testing.T) {
	withJWTConfig(t, "test-secret", 3600)

	claims := Token{
		Username: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := signedToken(t, jwt.SigningMethodHS256, claims)

	expectParseJWTError(t, token, "token without not-before")
}

func TestGenerateJWTRejectsShortSigningSecret(t *testing.T) {
	withJWTConfig(t, "short", 3600)

	expectGenerateJWTError(t, "admin", "short signing secret")
}
