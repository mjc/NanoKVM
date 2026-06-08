package utils

import "os/exec"

func RestartNanoKVM() error {
	name, args := restartNanoKVMCommand()
	return exec.Command(name, args...).Run()
}

func restartNanoKVMCommand() (string, []string) {
	return "/etc/init.d/S95nanokvm", []string{"restart"}
}
