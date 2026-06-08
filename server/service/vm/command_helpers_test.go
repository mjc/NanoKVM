package vm

import "testing"

func TestParseAvahiPID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "plain", input: "123", want: "123"},
		{name: "trimmed", input: " 456\n", want: "456"},
		{name: "empty", input: "", wantErr: true},
		{name: "non-numeric", input: "12a", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseAvahiPID(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestBuildSwapCommands(t *testing.T) {
	got := buildSwapCommands(128)
	if len(got) != 4 {
		t.Fatalf("expected 4 commands, got %d", len(got))
	}

	want := []commandSpec{
		{name: "fallocate", args: []string{"-l", "128M", SwapFile}},
		{name: "chmod", args: []string{"600", SwapFile}},
		{name: "mkswap", args: []string{SwapFile}},
		{name: "swapon", args: []string{SwapFile}},
	}

	for i := range want {
		if got[i].name != want[i].name {
			t.Fatalf("command %d name: expected %q, got %q", i, want[i].name, got[i].name)
		}
		if len(got[i].args) != len(want[i].args) {
			t.Fatalf("command %d args length: expected %d, got %d", i, len(want[i].args), len(got[i].args))
		}
		for j := range want[i].args {
			if got[i].args[j] != want[i].args[j] {
				t.Fatalf("command %d arg %d: expected %q, got %q", i, j, want[i].args[j], got[i].args[j])
			}
		}
	}
}

func TestVirtualDeviceCommandSpecsUseArgv(t *testing.T) {
	commandSets := [][]commandSpec{
		mountNetworkCommands,
		unmountNetworkCommands,
		mountDiskCommands,
		unmountDiskCommands,
	}

	for setIdx, set := range commandSets {
		for cmdIdx, cmd := range set {
			if cmd.name == "sh" {
				t.Fatalf("set %d command %d unexpectedly uses sh", setIdx, cmdIdx)
			}
			for _, arg := range cmd.args {
				if arg == "-c" {
					t.Fatalf("set %d command %d unexpectedly uses -c", setIdx, cmdIdx)
				}
			}
		}
	}
}
