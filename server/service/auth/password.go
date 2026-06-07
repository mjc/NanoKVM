package auth

import (
	"NanoKVM-Server/config"
	"NanoKVM-Server/proto"
	"NanoKVM-Server/utils"
	"context"
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

func (s *Service) ChangePassword(c *gin.Context) {
	var req proto.ChangePasswordReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid parameters")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !validAccountName(req.Username) {
		rsp.ErrRsp(c, -2, "invalid username or password")
		return
	}
	if !CompareAccount(req.Username, req.OldPassword) {
		rsp.ErrRsp(c, -2, "invalid username or password")
		return
	}

	password, err := utils.DecodeDecrypt(req.Password)
	if err != nil || password == "" {
		rsp.ErrRsp(c, -2, "invalid password")
		return
	}
	if !validPassword(password) {
		rsp.ErrRsp(c, -2, "invalid password")
		return
	}
	previousAccount, _ := GetAccount()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		rsp.ErrRsp(c, -3, "failed to hash password")
		return
	}

	if err = SetAccount(req.Username, string(hashedPassword)); err != nil {
		rsp.ErrRsp(c, -4, "failed to save password")
		return
	}

	// change root password
	err = changeRootPassword(password)
	if err != nil {
		_ = restoreAccount(previousAccount)
		rsp.ErrRsp(c, -5, "failed to change password")
		return
	}

	config.RegenerateSecretKey()
	rsp.OkRsp(c)
	log.Debugf("change password success, username: %s", req.Username)
}

func (s *Service) IsPasswordUpdated(c *gin.Context) {
	var rsp proto.Response

	if _, err := os.Stat(AccountFile); err != nil {
		rsp.OkRspWithData(c, &proto.IsPasswordUpdatedRsp{
			IsUpdated: false,
		})
		return
	}

	account, err := GetAccount()
	if err != nil || account == nil {
		rsp.ErrRsp(c, -1, "failed to get password")
		return
	}

	if account.HashedPassword == "" {
		if legacyPassword, legacyErr := utils.DecodeDecrypt(account.HashedPassword); legacyErr == nil && legacyPassword == "admin" {
			rsp.OkRspWithData(c, &proto.IsPasswordUpdatedRsp{IsUpdated: false})
			return
		}
	}

	err = bcrypt.CompareHashAndPassword([]byte(account.HashedPassword), []byte("admin"))

	rsp.OkRspWithData(c, &proto.IsPasswordUpdatedRsp{
		// If the hash is not valid, still assume it's not updated
		// The error we want to see is password and hash not matching
		IsUpdated: errors.Is(err, bcrypt.ErrMismatchedHashAndPassword),
	})
}

func changeRootPassword(password string) error {
	err := passwd(password)
	if err != nil {
		log.Errorf("failed to change root password: %s", err)
		return err
	}

	log.Debugf("change root password successful.")
	return nil
}

func passwd(password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "passwd", "root")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer func() {
		_ = stdin.Close()
	}()

	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err = cmd.Start(); err != nil {
		return err
	}

	if _, err = io.WriteString(stdin, password+"\n"); err != nil {
		return err
	}

	if _, err = io.WriteString(stdin, password+"\n"); err != nil {
		return err
	}

	if err = cmd.Wait(); err != nil {
		return err
	}

	return nil
}

func validAccountName(username string) bool {
	if username == "" || strings.ContainsAny(username, `/\`) {
		return false
	}
	return !strings.ContainsFunc(username, unicode.IsControl)
}

func validPassword(password string) bool {
	if len([]byte(password)) > 72 {
		return false
	}
	return !strings.ContainsFunc(password, unicode.IsControl)
}
