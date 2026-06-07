package auth

import (
	"time"

	"NanoKVM-Server/config"
	"NanoKVM-Server/middleware"
	"NanoKVM-Server/proto"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func (s *Service) Login(c *gin.Context) {
	var req proto.LoginReq
	var rsp proto.Response
	const loginFailureMessage = "authentication failed"

	clientIP := GetClientIP(c)
	recordFailure := RecordLoginFailure
	if err := proto.ParseFormRequest(c, &req); err != nil {
		time.Sleep(3 * time.Second)
		recordFailure(LoginAttemptKey(clientIP, ""))
		rsp.ErrRsp(c, -1, loginFailureMessage)
		return
	}

	attemptKey := LoginAttemptKey(clientIP, req.Username)
	if locked, code, msg := CheckLoginAttempt(attemptKey); locked {
		time.Sleep(3 * time.Second)
		rsp.ErrRsp(c, code, msg)
		return
	}

	// authentication disabled
	conf := config.GetInstance()
	if conf.Authentication == "disable" {
		rsp.OkRsp(c)
		return
	}

	if ok := CompareAccount(req.Username, req.Password); !ok {
		time.Sleep(3 * time.Second)

		if locked, code, msg := RecordLoginFailure(attemptKey); locked {
			rsp.ErrRsp(c, code, msg)
			return
		}

		rsp.ErrRsp(c, -2, loginFailureMessage)
		return
	}

	ClearLoginAttempt(attemptKey)

	token, err := middleware.GenerateJWT(req.Username)
	if err != nil {
		time.Sleep(3 * time.Second)
		rsp.ErrRsp(c, -3, "generate token failed")
		return
	}

	c.SetCookie("nano-kvm-token", token, int(conf.JWT.RefreshTokenDuration), "/", "", conf.Proto == "https", true)
	rsp.OkRspWithData(c, &proto.LoginRsp{})

	log.Debugf("login success, username: %s", req.Username)
}

func (s *Service) Logout(c *gin.Context) {
	config.RegenerateSecretKey()
	c.SetCookie("nano-kvm-token", "", -1, "/", "", true, true)

	var rsp proto.Response
	rsp.OkRsp(c)
}

func (s *Service) GetAccount(c *gin.Context) {
	var rsp proto.Response

	account, err := GetAccount()
	if err != nil {
		rsp.ErrRsp(c, -1, "get account failed")
		return
	}

	rsp.OkRspWithData(c, &proto.GetAccountRsp{
		Username: account.Username,
	})
	log.Debugf("get account successful")
}
