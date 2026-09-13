package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublishUpdatesCoalescesBursts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan struct{}, 8)
	var publishes atomic.Int32
	started := make(chan struct{})

	go func() {
		close(started)
		publishUpdates(ctx, 25*time.Millisecond, updates, func() error {
			publishes.Add(1)
			return nil
		}, func(error) {})
	}()
	<-started

	for i := 0; i < 5; i++ {
		updates <- struct{}{}
	}
	time.Sleep(40 * time.Millisecond)
	if got := publishes.Load(); got != 1 {
		t.Fatalf("publishes after burst = %d, want 1", got)
	}

	updates <- struct{}{}
	cancel()
	time.Sleep(10 * time.Millisecond)
	if got := publishes.Load(); got != 2 {
		t.Fatalf("publishes after shutdown flush = %d, want 2", got)
	}
}
