package tailscale

import (
	"NanoKVM-Server/utils"
	"bufio"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"regexp"
	"strings"
)

const (
	ScriptPath       = "/etc/init.d/S98tailscaled"
	ScriptBackupPath = "/kvmapp/system/init.d/S98tailscaled"
)

var loginURLPattern = regexp.MustCompile(`https://[^\s]+`)

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

	return runInitScriptAction("start")
}

func (c *Cli) Restart() error {
	return runInitScriptAction("restart")
}

func (c *Cli) Stop() error {
	if err := utils.Run(ScriptPath, "stop"); err != nil {
		return err
	}

	return os.Remove(ScriptPath)
}

func (c *Cli) Up() error {
	return utils.Run("tailscale", "up", "--accept-dns=false")
}

func (c *Cli) Down() error {
	return utils.Run("tailscale", "down")
}

func (c *Cli) Status() (*TsStatus, error) {
	output, err := utils.RunOutput("tailscale", "status", "--json")
	if err != nil {
		return nil, err
	}

	return parseStatusOutput(output)
}

func (c *Cli) Login() (string, error) {
	cmd := utils.Command("tailscale", "login", "--accept-dns=false", "--timeout=10m")

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

		if url := extractLoginURL(line); url != "" {
			return url, nil
		}
	}
}

func (c *Cli) Logout() error {
	return utils.Run("tailscale", "logout")
}

func runInitScriptAction(action string) error {
	return utils.RestoreAndRunInitScriptAction(ScriptPath, ScriptBackupPath, action)
}

func parseStatusOutput(output []byte) (*TsStatus, error) {
	outputStr := string(output)
	if !strings.HasPrefix(outputStr, "{") {
		index := strings.Index(outputStr, "{")
		if index == -1 {
			return nil, errors.New("unknown output")
		}

		output = []byte(outputStr[index:])
	}

	var status TsStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

func extractLoginURL(line string) string {
	candidate := loginURLPattern.FindString(line)
	if candidate == "" {
		return ""
	}

	parsed, err := url.ParseRequestURI(candidate)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ""
	}

	return candidate
}
