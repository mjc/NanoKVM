package tailscale

import "testing"

func TestInitScriptCommandSpecs(t *testing.T) {
	got := initScriptCommandSpecs("restart")
	if len(got) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(got))
	}

	if got[0].name != "cp" {
		t.Fatalf("expected first command to be cp, got %q", got[0].name)
	}
	if len(got[0].args) != 3 || got[0].args[0] != "-f" || got[0].args[1] != ScriptBackupPath || got[0].args[2] != ScriptPath {
		t.Fatalf("unexpected copy args: %#v", got[0].args)
	}

	if got[1].name != ScriptPath {
		t.Fatalf("expected second command %q, got %q", ScriptPath, got[1].name)
	}
	if len(got[1].args) != 1 || got[1].args[0] != "restart" {
		t.Fatalf("unexpected action args: %#v", got[1].args)
	}
}

func TestTailscaleCommandSpec(t *testing.T) {
	got := tailscaleCommandSpec("status", "--json")
	if got.name != "tailscale" {
		t.Fatalf("expected command name tailscale, got %q", got.name)
	}
	if len(got.args) != 2 || got.args[0] != "status" || got.args[1] != "--json" {
		t.Fatalf("unexpected command args: %#v", got.args)
	}
}
