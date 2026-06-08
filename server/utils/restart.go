package utils

func RestartNanoKVM() error {
	name, args := restartNanoKVMCommand()
	return Run(name, args...)
}

func restartNanoKVMCommand() (string, []string) {
	return "/etc/init.d/S95nanokvm", []string{"restart"}
}
