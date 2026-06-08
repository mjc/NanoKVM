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
		{Name: "fallocate", Args: []string{"-l", "128M", SwapFile}},
		{Name: "chmod", Args: []string{"600", SwapFile}},
		{Name: "mkswap", Args: []string{SwapFile}},
		{Name: "swapon", Args: []string{SwapFile}},
	}

	for i := range want {
		if got[i].Name != want[i].Name {
			t.Fatalf("command %d name: expected %q, got %q", i, want[i].Name, got[i].Name)
		}
		if len(got[i].Args) != len(want[i].Args) {
			t.Fatalf("command %d args length: expected %d, got %d", i, len(want[i].Args), len(got[i].Args))
		}
		for j := range want[i].Args {
			if got[i].Args[j] != want[i].Args[j] {
				t.Fatalf("command %d arg %d: expected %q, got %q", i, j, want[i].Args[j], got[i].Args[j])
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
			if cmd.Name == "sh" {
				t.Fatalf("set %d command %d unexpectedly uses sh", setIdx, cmdIdx)
			}
			for _, arg := range cmd.Args {
				if arg == "-c" {
					t.Fatalf("set %d command %d unexpectedly uses -c", setIdx, cmdIdx)
				}
			}
		}
	}
}
