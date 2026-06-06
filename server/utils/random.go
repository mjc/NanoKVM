package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

func RandomBase64URLString(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(buf), nil
}

func RandomHexString(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf), nil
}
