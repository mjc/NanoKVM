package auth

import (
	"NanoKVM-Server/config"
	"NanoKVM-Server/proto"
	"NanoKVM-Server/utils"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

var (
	setAccount                      = SetAccount
	delAccount                      = DelAccount
	getAccount                      = GetAccount
	changeRootPassword              = changeRootPasswordImpl
	revokeTokensAfterPasswordChange = config.RegenerateSecretKey
)

func (s *Service) ChangePassword(c *gin.Context) {
	var req proto.ChangePasswordReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid parameters")
		return
	}

	password, err := utils.DecodeDecrypt(req.Password)
	if err != nil || password == "" {
		rsp.ErrRsp(c, -2, "invalid password")
		return
	}

	if err := validatePasswordChange(req.Username, password); err != nil {
		rsp.ErrRsp(c, -2, "invalid password")
		return
	}

	previousAccount, previousAccountErr := getAccount()
	if previousAccountErr != nil {
		rsp.ErrRsp(c, -1, "failed to get password")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		rsp.ErrRsp(c, -3, "failed to hash password")
		return
	}

	if err = setAccount(req.Username, string(hashedPassword)); err != nil {
		rsp.ErrRsp(c, -4, "failed to save password")
		return
	}

	// change root password
	err = changeRootPassword(password)
	if err != nil {
		if previousAccount != nil {
			if restoreErr := setAccount(previousAccount.Username, previousAccount.Password); restoreErr != nil {
				log.Errorf("failed to restore password after root password change failure: %s", restoreErr)
			}
		}
		rsp.ErrRsp(c, -5, "failed to change password")
		return
	}

	revokeTokensAfterPasswordChange()

	rsp.OkRsp(c)
	log.Debugf("change password success, username: %s", req.Username)
}

func validatePasswordChange(username string, password string) error {
	if !isSafeAccountUsername(username) {
		return errors.New("invalid username")
	}
	if !isSafePassword(username, password) {
		return errors.New("invalid password")
	}

	return nil
}

func isSafeAccountUsername(username string) bool {
	if username != strings.TrimSpace(username) {
		return false
	}
	if strings.ContainsAny(username, `/\<>`) {
		return false
	}
	if strings.ContainsFunc(username, func(r rune) bool {
		return unicode.IsControl(r)
	}) {
		return false
	}
	if username == "" {
		return false
	}

	return true
}

func isSafePassword(_ string, password string) bool {
	if len([]byte(password)) > 72 {
		return false
	}
	if strings.ContainsFunc(password, func(r rune) bool {
		return unicode.IsControl(r)
	}) {
		return false
	}

	return true
}

func (s *Service) IsPasswordUpdated(c *gin.Context) {
	var rsp proto.Response

	isUpdated, err := IsDefaultPasswordChanged()
	if err != nil {
		rsp.ErrRsp(c, -1, "failed to get password")
		return
	}

	rsp.OkRspWithData(c, &proto.IsPasswordUpdatedRsp{
		IsUpdated: isUpdated,
	})
}

func IsDefaultPasswordChanged() (bool, error) {
	account, err := GetAccount()
	if err != nil {
		return false, err
	}

	return isDefaultPasswordChanged(account), nil
}

func isDefaultPasswordChanged(account *Account) bool {
	if account == nil {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte("admin"))
	if err == nil {
		return false
	}
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return true
	}

	legacyPassword, err := decodeLegacyPassword(account.Password)
	return err == nil && legacyPassword != "" && legacyPassword != "admin"
}

func decodeLegacyPassword(password string) (value string, err error) {
	defer func() {
		if r := recover(); r != nil {
			value = ""
			err = errors.New("decode legacy password failed")
		}
	}()

	return utils.DecodeDecrypt(password)
}

func changeRootPasswordImpl(password string) error {
	err := passwd(password)
	if err != nil {
		log.Errorf("failed to change root password: %s", err)
		return err
	}

	log.Debugf("change root password successful.")
	return nil
}

func passwd(password string) error {
	cmd := exec.Command("passwd", "root")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer func() {
		_ = stdin.Close()
	}()

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Start(); err != nil {
		return err
	}

	if _, err = io.WriteString(stdin, password+"\n"); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	if _, err = io.WriteString(stdin, password+"\n"); err != nil {
		return err
	}

	if err = cmd.Wait(); err != nil {
		return err
	}

	return nil
}
