package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nasmon/internal/app"
	"nasmon/internal/statefile"
)

func publishUpdates(ctx context.Context, interval time.Duration, updates <-chan struct{}, publish func() error, report func(error)) {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	dirty := false

	flush := func() {
		if !dirty {
			return
		}
		if err := publish(); err != nil {
			report(err)
			return
		}
		dirty = false
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case <-updates:
			dirty = true
		case <-ticker.C:
			flush()
		}
	}
}

func main() {
	cfg, err := app.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nasmond: cannot load config: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	lock, err := statefile.AcquireWriterLock(cfg.StateFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nasmond: cannot acquire writer lock: %v\n", err)
		os.Exit(1)
	}
	defer lock.Close()
	defer os.Remove(cfg.StateFile)

	a := app.New(cfg)
	if err := statefile.WriteAtomic(cfg.StateFile, a.Store.Snapshot()); err != nil {
		fmt.Fprintf(os.Stderr, "nasmond: cannot write startup state %s: %v\n", cfg.StateFile, err)
		os.Exit(1)
	}

	a.Bootstrap()
	if err := statefile.WriteAtomic(cfg.StateFile, a.Store.Snapshot()); err != nil {
		fmt.Fprintf(os.Stderr, "nasmond: cannot write initial state %s: %v\n", cfg.StateFile, err)
		os.Exit(1)
	}
	a.Start(ctx)

	publishUpdates(ctx, cfg.StateInterval, a.Updates, func() error {
		return statefile.WriteAtomic(cfg.StateFile, a.Store.Snapshot())
	}, func(err error) {
		fmt.Fprintf(os.Stderr, "nasmond: cannot update state %s: %v\n", cfg.StateFile, err)
	})
}
