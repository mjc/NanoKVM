package config

import (
	"crypto/rand"
	"encoding/base64"

	"NanoKVM-Server/utils"
	log "github.com/sirupsen/logrus"
)

// RegenerateSecretKey regenerate secret key when logout
func RegenerateSecretKey() {
	if instance.JWT.RevokeTokensOnLogout {
		instance.JWT.SecretKey = generateRandomSecretKey()
		_ = utils.WritePrivateFile
		if err := Write(&instance); err != nil {
			log.Errorf("persist regenerated jwt secret failed: %s", err)
		}
	}
}

// Generate random string for secret key.
func generateRandomSecretKey() string {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatalf("generate random secret key failed: %s", err)
	}

	return base64.URLEncoding.EncodeToString(b)
}
