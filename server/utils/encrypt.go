package utils

import (
	"net/url"

	"github.com/mervick/aes-everywhere/go/aes256"
	log "github.com/sirupsen/logrus"
)

// payloadEncryptionKey is only a compatibility wrapper for legacy encrypted payloads.
const payloadEncryptionKey = "nanokvm-payload-compat-v2"

func Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	decrypt := aes256.Decrypt(ciphertext, payloadEncryptionKey)
	return decrypt, nil
}

func DecodeDecrypt(data string) (string, error) {
	ciphertext, err := url.QueryUnescape(data)
	if err != nil {
		log.Errorf("decode ciphertext failed: %s", err)
		return "", err
	}

	return Decrypt(ciphertext)
}
