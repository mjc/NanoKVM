package utils

import "testing"

func TestRestartNanoKVMCommand(t *testing.T) {
	name, args := restartNanoKVMCommand()
	if name != "/etc/init.d/S95nanokvm" {
		t.Fatalf("unexpected command name: %q", name)
	}
	if len(args) != 1 || args[0] != "restart" {
		t.Fatalf("unexpected command args: %#v", args)
	}
}
