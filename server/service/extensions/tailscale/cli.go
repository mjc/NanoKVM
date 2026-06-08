package tailscale

import (
	"NanoKVM-Server/utils"
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"regexp"
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

	return runInitScriptAction("start")
}

func (c *Cli) Restart() error {
	return runInitScriptAction("restart")
}

func (c *Cli) Stop() error {
	if err := exec.Command(ScriptPath, "stop").Run(); err != nil {
		return err
	}

	return os.Remove(ScriptPath)
}

func (c *Cli) Up() error {
	return runTailscale("up", "--accept-dns=false")
}

func (c *Cli) Down() error {
	return runTailscale("down")
}

func (c *Cli) Status() (*TsStatus, error) {
	output, err := runTailscaleOutput("status", "--json")
	if err != nil {
		return nil, err
	}

	// output is not in standard json format
	if outputStr := string(output); !strings.HasPrefix(outputStr, "{") {
		index := strings.Index(outputStr, "{")
		if index == -1 {
			return nil, errors.New("unknown output")
		}

		output = []byte(outputStr[index:])
	}

	var status TsStatus
	err = json.Unmarshal(output, &status)
	if err != nil {
		return nil, err
	}

	return &status, nil
}

func (c *Cli) Login() (string, error) {
	cmd := tailscaleCommand("login", "--accept-dns=false", "--timeout=10m")

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

		if strings.Contains(line, "https") {
			reg := regexp.MustCompile(`\s+`)
			url := reg.ReplaceAllString(line, "")
			return url, nil
		}
	}
}

func (c *Cli) Logout() error {
	return runTailscale("logout")
}

func runInitScriptAction(action string) error {
	for _, command := range initScriptCommands(action) {
		if err := exec.Command(command.name, command.args...).Run(); err != nil {
			return err
		}
	}

	return nil
}

func initScriptCommands(action string) []struct {
	name string
	args []string
} {
	return []struct {
		name string
		args []string
	}{
		{name: "cp", args: []string{"-f", ScriptBackupPath, ScriptPath}},
		{name: ScriptPath, args: []string{action}},
	}
}

func runTailscale(args ...string) error {
	return tailscaleCommand(args...).Run()
}

func runTailscaleOutput(args ...string) ([]byte, error) {
	return tailscaleCommand(args...).CombinedOutput()
}

func tailscaleCommand(args ...string) *exec.Cmd {
	return exec.Command("tailscale", args...)
}
