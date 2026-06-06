package config

import (
	"NanoKVM-Server/utils"
	"fmt"
	"time"
)

// RegenerateSecretKey regenerate secret key when logout
func RegenerateSecretKey() {
	if instance.JWT.RevokeTokensOnLogout {
		ForceRegenerateSecretKey()
	}
}

func ForceRegenerateSecretKey() {
	instance.JWT.SecretKey = generateRandomSecretKey()
}

// Generate random string for secret key.
func generateRandomSecretKey() string {
	secret, err := utils.RandomBase64URLString(64)
	if err != nil {
		currentTime := time.Now().UnixNano()
		timeString := fmt.Sprintf("%d", currentTime)
		return fmt.Sprintf("%064s", timeString)
	}

	return secret
}
