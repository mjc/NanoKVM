package auth

import (
	"NanoKVM-Server/utils"
	"encoding/json"
	"errors"
	"os"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

const AccountFile = "/etc/kvm/pwd"

var accountFile = AccountFile

const (
	defaultUsername = "admin"
	defaultPassword = "admin"
)

type Account struct {
	Username string `json:"username"`
	Password string `json:"password"` // should be named HashedPassword for clarity
}

func GetAccount() (*Account, error) {
	content, err := utils.ReadPrivateFile(accountFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return getDefaultAccount(), nil
		}
		return nil, err
	}

	var account Account
	if err = json.Unmarshal(content, &account); err != nil {
		log.Errorf("unmarshal account failed: %s", err)
		return nil, err
	}

	return &account, nil
}

func SetAccount(username string, hashedPassword string) error {
	account, err := json.Marshal(&Account{
		Username: username,
		Password: hashedPassword,
	})
	if err != nil {
		log.Errorf("failed to marshal account information to json: %s", err)
		return err
	}

	if err = utils.WritePrivateFile(accountFile, account); err != nil {
		log.Errorf("write password failed: %s", err)
		return err
	}

	return nil
}

func CompareAccount(username string, plainPassword string) bool {
	account, err := GetAccount()
	if err != nil {
		return false
	}

	if username != account.Username {
		return false
	}

	hashedPassword, err := utils.DecodeDecrypt(plainPassword)
	if err != nil || hashedPassword == "" {
		return false
	}

	return matchStoredPassword(account.Password, hashedPassword)
}

func DelAccount() error {
	if err := utils.RemoveFileIfExists(accountFile); err != nil {
		log.Errorf("failed to delete password: %s", err)
		return err
	}

	return nil
}

func getDefaultAccount() *Account {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)

	return &Account{
		Username: defaultUsername,
		Password: string(hashedPassword),
	}
}

func matchStoredPassword(storedPassword string, plainPassword string) bool {
	if isBcryptHash(storedPassword) {
		return bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(plainPassword)) == nil
	}

	legacyPassword, err := decodeLegacyPassword(storedPassword)
	return err == nil && legacyPassword == plainPassword
}

func isBcryptHash(storedPassword string) bool {
	return len(storedPassword) >= 4 && storedPassword[0] == '$' && storedPassword[1] == '2'
}
