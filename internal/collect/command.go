package collect

import (
	"context"
	"os/exec"
	"time"
)

const externalCommandTimeout = 8 * time.Second

func commandOutput(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), externalCommandTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

func commandCombinedOutput(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), externalCommandTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}
