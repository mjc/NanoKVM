package tailscale

import (
	"NanoKVM-Server/utils"
	"bufio"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

const (
	ScriptPath       = "/etc/init.d/S98tailscaled"
	ScriptBackupPath = "/kvmapp/system/init.d/S98tailscaled"
)

type Cli struct{}

type TsStatus struct {
	BackendState string `json:"BackendState"`

	Self struct {
		HostName     string   `json:"HostName"`
		TailscaleIPs []string `json:"TailscaleIPs"`
	} `json:"Self"`

	CurrentTailnet struct {
		Name string `json:"Name"`
	} `json:"CurrentTailnet"`
}

func NewCli() *Cli {
	return &Cli{}
}

func (c *Cli) Start() error {
	for _, filePath := range []string{TailscalePath, TailscaledPath} {
		if err := utils.EnsurePermission(filePath, 0o100); err != nil {
			return err
		}
	}

	if err := copyInitScriptCommand().Run(); err != nil {
		return err
	}
	return serviceScriptCommand("start").Run()
}

func (c *Cli) Restart() error {
	if err := copyInitScriptCommand().Run(); err != nil {
		return err
	}
	return serviceScriptCommand("restart").Run()
}

func (c *Cli) Stop() error {
	if err := serviceScriptCommand("stop").Run(); err != nil {
		return err
	}

	return os.Remove(ScriptPath)
}

func (c *Cli) Up() error {
	return tailscaleCommand("up", "--accept-dns=false").Run()
}

func (c *Cli) Down() error {
	return tailscaleCommand("down").Run()
}

func (c *Cli) Status() (*TsStatus, error) {
	cmd := tailscaleCommand("status", "--json")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return parseStatusJSON(output)
}

func (c *Cli) Login() (string, error) {
	cmd := tailscaleCommand("login", "--accept-dns=false", "--timeout=2m")

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = stderr.Close()
	}()

	go func() {
		_ = cmd.Run()
	}()

	reader := bufio.NewReader(stderr)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		loginURL, ok := extractLoginURL(line)
		if ok {
			return loginURL, nil
		}
	}
}

func (c *Cli) Logout() error {
	return tailscaleCommand("logout").Run()
}

func copyInitScriptCommand() *exec.Cmd {
	return exec.Command("cp", "-f", ScriptBackupPath, ScriptPath)
}

func serviceScriptCommand(action string) *exec.Cmd {
	switch action {
	case "start", "restart", "stop":
		return exec.Command(ScriptPath, action)
	default:
		return exec.Command("false")
	}
}

func tailscaleCommand(args ...string) *exec.Cmd {
	return exec.Command(TailscalePath, args...)
}

func parseStatusJSON(output []byte) (*TsStatus, error) {
	if !strings.HasPrefix(strings.TrimSpace(string(output)), "{") {
		return nil, errors.New("unknown output")
	}

	var status TsStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func extractLoginURL(line string) (string, bool) {
	fields := strings.Fields(line)
	for _, field := range fields {
		parsed, err := url.Parse(field)
		if err != nil {
			continue
		}
		if parsed.Scheme == "https" && parsed.Hostname() == "login.tailscale.com" && strings.HasPrefix(parsed.Path, "/a/") {
			return parsed.String(), true
		}
	}
	return "", false
}
