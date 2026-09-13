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
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}
