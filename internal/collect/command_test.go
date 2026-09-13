package collect

import (
	"testing"
	"time"
)

func TestCommandCombinedOutputTimeout(t *testing.T) {
	start := time.Now()
	_, err := commandCombinedOutputTimeout(50*time.Millisecond, "sh", "-c", "exec sleep 5")
	if err == nil {
		t.Fatalf("timed command unexpectedly succeeded")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("timed command took %s, expected cancellation well under 1s", elapsed)
	}
}
