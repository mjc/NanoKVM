package vm

import (
	"NanoKVM-Server/proto"
	"NanoKVM-Server/utils"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	AvahiDaemonPid          = "/run/avahi-daemon/pid"
	AvahiDaemonScript       = "/etc/init.d/S50avahi-daemon"
	AvahiDaemonBackupScript = "/kvmapp/system/init.d/S50avahi-daemon"
)

func (s *Service) GetMdnsState(c *gin.Context) {
	var rsp proto.Response

	pid := getAvahiDaemonPid()

	rsp.OkRspWithData(c, &proto.GetMdnsStateRsp{
		Enabled: pid != "",
	})
}

func (s *Service) EnableMdns(c *gin.Context) {
	var rsp proto.Response

	pid := getAvahiDaemonPid()
	if pid != "" {
		rsp.OkRsp(c)
		return
	}

	if err := utils.RestoreAndRunInitScriptAction(AvahiDaemonScript, AvahiDaemonBackupScript, "start"); err != nil {
		log.Errorf("failed to start avahi-daemon: %s", err)
		rsp.ErrRsp(c, -1, "failed to enable mdns")
		return
	}

	rsp.OkRsp(c)
	log.Debugf("avahi-daemon started")
}

func (s *Service) DisableMdns(c *gin.Context) {
	var rsp proto.Response

	pid := getAvahiDaemonPid()
	if pid == "" {
		rsp.OkRsp(c)
		return
	}

	validPID, err := parseAvahiPID(pid)
	if err != nil {
		log.Errorf("invalid mdns pid %q: %s", pid, err)
		rsp.ErrRsp(c, -1, "failed to disable mdns")
		return
	}
	pidValue, err := strconv.Atoi(validPID)
	if err != nil {
		log.Errorf("invalid mdns pid %q: %s", validPID, err)
		rsp.ErrRsp(c, -1, "failed to disable mdns")
		return
	}

	err = syscall.Kill(pidValue, syscall.SIGKILL)
	if err != nil {
		log.Errorf("failed to stop avahi-daemon: %s", err)
		rsp.ErrRsp(c, -1, "failed to disable mdns")
		return
	}

	_ = os.Remove(AvahiDaemonPid)
	_ = os.Remove(AvahiDaemonScript)

	rsp.OkRsp(c)
	log.Debugf("avahi-daemon stopped")
}

func getAvahiDaemonPid() string {
	if _, err := os.Stat(AvahiDaemonPid); err != nil {
		return ""
	}

	content, err := os.ReadFile(AvahiDaemonPid)
	if err != nil {
		log.Errorf("failed to read mdns pid: %s", err)
		return ""
	}

	return strings.ReplaceAll(string(content), "\n", "")
}

func parseAvahiPID(pid string) (string, error) {
	clean := strings.TrimSpace(pid)
	if clean == "" {
		return "", strconv.ErrSyntax
	}
	pidValue, err := strconv.Atoi(clean)
	if err != nil {
		return "", err
	}
	if pidValue <= 0 {
		return "", strconv.ErrSyntax
	}
	return clean, nil
}
