package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"nasmon/internal/app"
	"nasmon/internal/model"
	"nasmon/internal/statefile"
	"nasmon/internal/tui"
)

func parseArgs(cfg *app.Config) (standalone bool, err error) {
	intervalSet := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--standalone":
			standalone = true
		case "-h", "--help":
			fmt.Println("usage: nasmon [--standalone] [refresh-seconds]")
			fmt.Println("       nasmon reads shared state from nasmond by default")
			os.Exit(0)
		default:
			if intervalSet {
				return false, fmt.Errorf("лишний аргумент %q", arg)
			}
			n, convErr := strconv.Atoi(arg)
			if convErr != nil || n < 1 {
				return false, fmt.Errorf("интервал должен быть числом >= 1")
			}
			if n > 3600 {
				n = 3600
			}
			cfg.MainInterval = time.Duration(n) * time.Second
			intervalSet = true
		}
	}
	return standalone, nil
}

func main() {
	cfg := app.DefaultConfig()
	standalone, err := parseArgs(&cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if standalone {
		runStandalone(ctx, cfg)
		return
	}
	runClient(ctx, cfg)
}

func runStandalone(ctx context.Context, cfg app.Config) {
	a := app.New(cfg)
	renderer := tui.Renderer{Config: cfg}
	a.Bootstrap()
	a.Start(ctx)
	tui.Enter()
	defer tui.Leave()
	draw := func() { rows, cols := tui.Size(); tui.Draw(renderer.RenderAdaptive(a.Store.Snapshot(), rows, cols)) }
	draw()
	if cfg.OneShot {
		return
	}

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.Updates:
			draw()
		case <-winch:
			draw()
		}
	}
}

func readInitialState(ctx context.Context, path string) (model.Snapshot, error) {
	const (
		startupWait = 3 * time.Second
		retryEvery  = 100 * time.Millisecond
	)

	snapshot, err := statefile.Read(path)
	if err == nil {
		return snapshot, nil
	}
	lastErr := err

	timer := time.NewTimer(startupWait)
	defer timer.Stop()
	ticker := time.NewTicker(retryEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return model.Snapshot{}, ctx.Err()
		case <-timer.C:
			return model.Snapshot{}, lastErr
		case <-ticker.C:
			snapshot, err = statefile.Read(path)
			if err == nil {
				return snapshot, nil
			}
			lastErr = err
		}
	}
}

func runClient(ctx context.Context, cfg app.Config) {
	snapshot, err := readInitialState(ctx, cfg.StateFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nasmon: не удалось прочитать %s: %v\n", cfg.StateFile, err)
		fmt.Fprintln(os.Stderr, "запусти nasmond или используй nasmon --standalone")
		os.Exit(1)
	}

	renderer := tui.Renderer{Config: cfg}
	clientStartedAt := time.Now()
	draw := func(s model.Snapshot) {
		s.StartedAt = clientStartedAt
		rows, cols := tui.Size()
		tui.Draw(renderer.RenderAdaptive(s, rows, cols))
	}

	tui.Enter()
	defer tui.Leave()
	draw(snapshot)
	if cfg.OneShot {
		return
	}

	ticker := time.NewTicker(cfg.MainInterval)
	defer ticker.Stop()
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if next, readErr := statefile.Read(cfg.StateFile); readErr == nil {
				snapshot = next
			}
			draw(snapshot)
		case <-winch:
			draw(snapshot)
		}
	}
}
