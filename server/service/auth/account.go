package auth

import (
	"NanoKVM-Server/utils"
	"encoding/json"
	"os"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

const AccountFile = "/etc/kvm/pwd"

type Account struct {
	Username       string `json:"username"`
	HashedPassword string `json:"hashedPassword"`
}

func GetAccount() (*Account, error) {
	if _, err := os.Stat(AccountFile); err != nil {
		return nil, err
	}

	content, err := utils.ReadPrivateFile(AccountFile)
	if err != nil {
		return nil, err
	}

	var account Account
	if err = json.Unmarshal(content, &account); err != nil {
		log.Errorf("unmarshal account failed: %s", err)
		return nil, err
	}
	if account.HashedPassword == "" {
		var legacy map[string]string
		if legacyErr := json.Unmarshal(content, &legacy); legacyErr != nil {
			return nil, legacyErr
		}
		account.Username = legacy["username"]
		account.HashedPassword = legacy["password"]
	}

	return &account, nil
}

func SetAccount(username string, hashedPassword string) error {
	account, err := json.Marshal(&Account{
		Username:       username,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		log.Errorf("failed to marshal account information to json: %s", err)
		return err
	}

	err = utils.WritePrivateFile(AccountFile, account)
	if err != nil {
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

	err = bcrypt.CompareHashAndPassword([]byte(account.HashedPassword), []byte(hashedPassword))
	if err != nil {
		// Migrate legacy encrypted account records after successful login.
		accountHashedPassword, _ := utils.DecodeDecrypt(account.HashedPassword)
		if accountHashedPassword == hashedPassword {
			newHash, hashErr := bcrypt.GenerateFromPassword([]byte(hashedPassword), bcrypt.DefaultCost)
			if hashErr == nil {
				_ = SetAccount(account.Username, string(newHash))
			}
			return true
		}

		return false
	}

	return true
}

func DelAccount() error {
	if err := os.Remove(AccountFile); err != nil {
		log.Errorf("failed to delete password: %s", err)
		return err
	}

	return nil
}

func getDefaultAccount() *Account {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)

	return &Account{
		Username:       "admin",
		HashedPassword: string(hashedPassword),
	}
}

func restoreAccount(account *Account) error {
	if account == nil {
		return DelAccount()
	}
	return SetAccount(account.Username, account.HashedPassword)
}
