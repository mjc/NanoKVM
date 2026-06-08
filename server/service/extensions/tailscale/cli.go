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

type commandSpec struct {
	name string
	args []string
}

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

	return runCommandSpecs(initScriptCommandSpecs("start"))
}

func (c *Cli) Restart() error {
	return runCommandSpecs(initScriptCommandSpecs("restart"))
}

func (c *Cli) Stop() error {
	if err := runCommandSpecs([]commandSpec{{name: ScriptPath, args: []string{"stop"}}}); err != nil {
		return err
	}

	return os.Remove(ScriptPath)
}

func (c *Cli) Up() error {
	return runCommandSpecs([]commandSpec{tailscaleCommandSpec("up", "--accept-dns=false")})
}

func (c *Cli) Down() error {
	return runCommandSpecs([]commandSpec{tailscaleCommandSpec("down")})
}

func (c *Cli) Status() (*TsStatus, error) {
	cmd := commandFromSpec(tailscaleCommandSpec("status", "--json"))

	output, err := cmd.CombinedOutput()
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
	cmd := commandFromSpec(tailscaleCommandSpec("login", "--accept-dns=false", "--timeout=10m"))

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
	return runCommandSpecs([]commandSpec{tailscaleCommandSpec("logout")})
}

func initScriptCommandSpecs(action string) []commandSpec {
	return []commandSpec{
		{name: "cp", args: []string{"-f", ScriptBackupPath, ScriptPath}},
		{name: ScriptPath, args: []string{action}},
	}
}

func tailscaleCommandSpec(args ...string) commandSpec {
	return commandSpec{name: "tailscale", args: args}
}

func commandFromSpec(spec commandSpec) *exec.Cmd {
	return exec.Command(spec.name, spec.args...)
}

func runCommandSpecs(commands []commandSpec) error {
	for _, command := range commands {
		if err := commandFromSpec(command).Run(); err != nil {
			return err
		}
	}
	return nil
}
