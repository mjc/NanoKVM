package tailscale

import "testing"

func TestTailscaleCommandsUseFixedArgv(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cmd  []string
		want []string
	}{
		{"up", tailscaleCommand("up", "--accept-dns=false").Args, []string{TailscalePath, "up", "--accept-dns=false"}},
		{"down", tailscaleCommand("down").Args, []string{TailscalePath, "down"}},
		{"status", tailscaleCommand("status", "--json").Args, []string{TailscalePath, "status", "--json"}},
		{"login", tailscaleCommand("login", "--accept-dns=false", "--timeout=2m").Args, []string{TailscalePath, "login", "--accept-dns=false", "--timeout=2m"}},
		{"logout", tailscaleCommand("logout").Args, []string{TailscalePath, "logout"}},
		{"script", serviceScriptCommand("restart").Args, []string{ScriptPath, "restart"}},
		{"copy", copyInitScriptCommand().Args, []string{"cp", "-f", ScriptBackupPath, ScriptPath}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.cmd) != len(tc.want) {
				t.Fatalf("args = %#v, want %#v", tc.cmd, tc.want)
			}
			for i := range tc.want {
				if tc.cmd[i] != tc.want[i] {
					t.Fatalf("args = %#v, want %#v", tc.cmd, tc.want)
				}
			}
		})
	}
}

func TestParseStatusJSONRejectsPrefixedOutput(t *testing.T) {
	t.Parallel()

	if _, err := parseStatusJSON([]byte("warning: noisy output\n{\"BackendState\":\"Running\"}")); err == nil {
		t.Fatal("parseStatusJSON accepted prefixed output")
	}
}

func TestParseStatusJSONAcceptsObjectOutput(t *testing.T) {
	t.Parallel()

	status, err := parseStatusJSON([]byte(`{"BackendState":"Running"}`))
	if err != nil {
		t.Fatalf("parseStatusJSON returned error: %v", err)
	}
	if status.BackendState != "Running" {
		t.Fatalf("BackendState = %q, want Running", status.BackendState)
	}
}

func TestExtractLoginURLValidatesTailscaleAuthURL(t *testing.T) {
	t.Parallel()

	got, ok := extractLoginURL("To authenticate, visit: https://login.tailscale.com/a/abcdef")
	if !ok {
		t.Fatal("extractLoginURL did not find auth URL")
	}
	if got != "https://login.tailscale.com/a/abcdef" {
		t.Fatalf("login URL = %q", got)
	}
}

func TestExtractLoginURLRejectsGenericHTTPSURL(t *testing.T) {
	t.Parallel()

	if got, ok := extractLoginURL("debug https://example.com/a/abcdef"); ok {
		t.Fatalf("extractLoginURL accepted %q", got)
	}
	if got, ok := extractLoginURL("debug https://login.tailscale.com/admin"); ok {
		t.Fatalf("extractLoginURL accepted %q", got)
	}
}
