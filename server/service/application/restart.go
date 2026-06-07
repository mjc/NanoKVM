package application

import "os/exec"

func restartNanoKVM() error {
	return exec.Command("/etc/init.d/S95nanokvm", "restart").Run()
}
