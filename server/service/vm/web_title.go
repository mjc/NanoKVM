package vm

import (
	"os"
	"strings"
	"unicode"

	"NanoKVM-Server/proto"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	WebTitleFile = "/etc/kvm/web-title"
)

func (s *Service) SetWebTitle(c *gin.Context) {
	var req proto.SetWebTitleReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	title, ok := normalizeWebTitle(req.Title)
	if !ok {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	if title == "" || title == "NanoKVM" {
		err := os.Remove(WebTitleFile)
		if err != nil && !os.IsNotExist(err) {
			rsp.ErrRsp(c, -2, "reset failed")
			return
		}
	} else {
		err := os.WriteFile(WebTitleFile, []byte(title), 0o600)
		if err != nil {
			rsp.ErrRsp(c, -3, "write failed")
			return
		}
	}

	rsp.OkRsp(c)
	log.Debugf("set web title")
}

func (s *Service) GetWebTitle(c *gin.Context) {
	var rsp proto.Response

	data, err := os.ReadFile(WebTitleFile)
	if err != nil {
		if os.IsNotExist(err) {
			rsp.OkRspWithData(c, &proto.GetWebTitleRsp{Title: "NanoKVM"})
			return
		}
		rsp.ErrRsp(c, -1, "read web title failed")
		return
	}

	rsp.OkRspWithData(c, &proto.GetWebTitleRsp{
		Title: strings.Replace(string(data), "\n", "", -1),
	})

	log.Debugf("get web title successful")
}

func normalizeWebTitle(title string) (string, bool) {
	title = strings.TrimSpace(title)
	if len(title) > 64 {
		return "", false
	}
	for _, r := range title {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return title, true
}
