package collect

import (
	"context"
	"os/exec"
	"time"
)

const externalCommandTimeout = 8 * time.Second

func commandOutput(name string, args ...string) ([]byte, error) {
	return commandOutputTimeout(externalCommandTimeout, name, args...)
}

func commandOutputTimeout(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

func commandCombinedOutput(name string, args ...string) ([]byte, error) {
	return commandCombinedOutputTimeout(externalCommandTimeout, name, args...)
}

func commandCombinedOutputTimeout(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil && smartctlInvocation(name, args) && smartctlUsableOutput(string(out)) {
		// smartctl uses non-zero exit-status bits to report disk health findings
		// as well as execution failures. If it produced a recognizable SMART
		// report, let the SMART parser consume it instead of treating the
		// collector itself as unavailable.
		err = nil
	}
	return out, err
}
