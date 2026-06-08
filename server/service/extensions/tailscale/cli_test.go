package tailscale

import (
	"path/filepath"
	"testing"
)

func TestInitScriptCommandSpecs(t *testing.T) {
	got := initScriptCommands("restart")
	if len(got) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(got))
	}

	if got[0].Name != "cp" {
		t.Fatalf("expected first command to be cp, got %q", got[0].Name)
	}
	if len(got[0].Args) != 3 || got[0].Args[0] != "-f" || got[0].Args[1] != ScriptBackupPath || got[0].Args[2] != ScriptPath {
		t.Fatalf("unexpected copy args: %#v", got[0].Args)
	}

	if got[1].Name != ScriptPath {
		t.Fatalf("expected second command %q, got %q", ScriptPath, got[1].Name)
	}
	if len(got[1].Args) != 1 || got[1].Args[0] != "restart" {
		t.Fatalf("unexpected action args: %#v", got[1].Args)
	}
}

func TestTailscaleCommand(t *testing.T) {
	cmd := tailscaleCommand("status", "--json")
	if filepath.Base(cmd.Path) != "tailscale" {
		t.Fatalf("expected command basename tailscale, got %q", cmd.Path)
	}
	if len(cmd.Args) != 3 || cmd.Args[0] != "tailscale" || cmd.Args[1] != "status" || cmd.Args[2] != "--json" {
		t.Fatalf("unexpected command args: %#v", cmd.Args)
	}
}
