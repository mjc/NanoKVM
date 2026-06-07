package middleware

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"

	"NanoKVM-Server/config"
)

type Token struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

const minJWTSecretBytes = 10
const maxJWTRefreshDuration = 24 * 60 * 60

func CheckToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if allowByToken(c) {
			c.Next()
			return
		}

		abortUnauthorized(c)
	}
}

func CheckLoopbackInternalToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if allowByLoopbackInternalToken(c.Request) {
			c.Next()
			return
		}

		abortUnauthorized(c)
	}
}

func CheckTokenOrLoopbackInternalToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if allowByToken(c) || allowByLoopbackInternalToken(c.Request) {
			c.Next()
			return
		}

		abortUnauthorized(c)
	}
}

func allowByToken(c *gin.Context) bool {
	cookie, err := c.Cookie("nano-kvm-token")
	if err != nil {
		return false
	}
	if !originAllowed(c.Request) {
		return false
	}

	_, err = ParseJWT(cookie)
	return err == nil
}

func originAllowed(req *http.Request) bool {
	origin := req.Header.Get("Origin")
	if origin == "" {
		return true
	}
	return origin == "https://"+req.Host || origin == "http://"+req.Host
}

func abortUnauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, "unauthorized")
	c.Abort()
}

func GenerateJWT(username string) (string, error) {
	conf := config.GetInstance()
	if username == "" {
		return "", errors.New("empty username")
	}
	if len(conf.JWT.SecretKey) < minJWTSecretBytes {
		return "", errors.New("jwt signing secret is too short")
	}
	if conf.JWT.RefreshTokenDuration == 0 || conf.JWT.RefreshTokenDuration > maxJWTRefreshDuration {
		return "", errors.New("invalid jwt refresh duration")
	}

	expireDuration := time.Duration(conf.JWT.RefreshTokenDuration) * time.Second
	now := time.Now()

	claims := Token{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(now.Add(expireDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return t.SignedString([]byte(conf.JWT.SecretKey))
}

func ParseJWT(jwtToken string) (*Token, error) {
	conf := config.GetInstance()
	if len(conf.JWT.SecretKey) < minJWTSecretBytes {
		return nil, errors.New("jwt signing secret is too short")
	}

	t, err := jwt.ParseWithClaims(jwtToken, &Token{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected jwt signing method")
		}
		return []byte(conf.JWT.SecretKey), nil
	})
	if err != nil {
		log.Debugf("parse jwt error: %s", err)
		return nil, err
	}

	if claims, ok := t.Claims.(*Token); ok && t.Valid {
		if claims.Username == "" {
			return nil, errors.New("empty username claim")
		}
		if claims.ExpiresAt == nil {
			return nil, errors.New("missing expiration claim")
		}
		if claims.IssuedAt == nil {
			return nil, errors.New("missing issued-at claim")
		}
		if claims.NotBefore == nil {
			return nil, errors.New("missing not-before claim")
		}
		if claims.IssuedAt.After(time.Now()) {
			return nil, errors.New("issued-at claim is in the future")
		}
		return claims, nil
	} else {
		return nil, err
	}
}
