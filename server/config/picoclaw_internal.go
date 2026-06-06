package config

import (
	"NanoKVM-Server/utils"
	"os"
	"strings"
	"sync"
)

const (
	PicoclawInternalTokenHeader = "X-NanoKVM-Internal-Token"
	picoclawInternalTokenFile   = "/etc/kvm/.picoclaw_internal_token"
)

var picoclawInternalToken struct {
	mu    sync.Mutex
	value string
}

func GetPicoclawInternalToken() (string, error) {
	picoclawInternalToken.mu.Lock()
	defer picoclawInternalToken.mu.Unlock()

	if picoclawInternalToken.value != "" {
		return picoclawInternalToken.value, nil
	}

	if token, err := readPicoclawInternalToken(); err == nil {
		if token != "" {
			picoclawInternalToken.value = token
			return token, nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}

	token := generateRandomSecretKey()
	if err := utils.WritePrivateFile(picoclawInternalTokenFile, []byte(token+"\n")); err != nil {
		return "", err
	}

	picoclawInternalToken.value = token
	return token, nil
}

func EnsurePicoclawInternalToken() error {
	_, err := GetPicoclawInternalToken()
	return err
}

func readPicoclawInternalToken() (string, error) {
	data, err := utils.ReadPrivateFile(picoclawInternalTokenFile)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}
