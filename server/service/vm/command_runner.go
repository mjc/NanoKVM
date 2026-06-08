package vm

import (
	"time"

	"NanoKVM-Server/utils"
)

type commandSpec = utils.CommandSpec

func runCommandSpecs(commands []commandSpec, delay time.Duration) error {
	return utils.RunSequenceWithDelay(commands, delay)
}
