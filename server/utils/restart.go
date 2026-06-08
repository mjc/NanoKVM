package utils

import "context"

func RestartNanoKVM() error {
	return RunInitScriptAction("/etc/init.d/S95nanokvm", "restart")
}

func Reboot() error {
	return Run("reboot")
}

func RunInitScriptAction(scriptPath string, action string) error {
	return Run(scriptPath, action)
}

func RunInitScriptActionOutput(scriptPath string, action string) ([]byte, error) {
	return RunOutput(scriptPath, action)
}

func RunInitScriptActionOutputContext(ctx context.Context, scriptPath string, action string) ([]byte, error) {
	return RunOutputContext(ctx, scriptPath, action)
}

func RestoreAndRunInitScriptAction(scriptPath string, backupPath string, action string) error {
	if err := CopyFile(backupPath, scriptPath); err != nil {
		return err
	}
	return RunInitScriptAction(scriptPath, action)
}
