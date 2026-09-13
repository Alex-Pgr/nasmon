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
	"nasmon/internal/doctor"
	"nasmon/internal/model"
	"nasmon/internal/statefile"
	"nasmon/internal/tui"
)

func printUsage() {
	fmt.Println("usage: nasmon [--standalone] [refresh-seconds]")
	fmt.Println("       nasmon doctor")
	fmt.Println("       nasmon reads shared state from nasmond by default")
}

func parseArgs(cfg *app.Config) (standalone bool, err error) {
	intervalSet := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--standalone":
			standalone = true
		case "-h", "--help":
			printUsage()
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
	cfg, err := app.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nasmon: cannot load config: %v\n", err)
		os.Exit(1)
	}
	if len(os.Args) == 2 && os.Args[1] == "doctor" {
		report := doctor.Run(cfg)
		report.Write(os.Stdout)
		if report.Failed() {
			os.Exit(1)
		}
		return
	}
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

	sortMode := tui.DockerSortDefault
	lastFrame := ""
	draw := func() {
		rows, cols := tui.Size()
		snapshot := a.Store.Snapshot()
		lastFrame = renderer.RenderInteractive(snapshot, rows, cols, sortMode)
		tui.Draw(lastFrame)
	}
	draw()
	if cfg.OneShot {
		return
	}

	mouseEvents, stopMouse := tui.StartMouseInput()
	defer stopMouse()
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
		case ev, ok := <-mouseEvents:
			if !ok {
				mouseEvents = nil
				continue
			}
			if next, changed := tui.DockerSortForClick(lastFrame, ev.X, ev.Y, sortMode); changed {
				sortMode = next
				draw()
			}
		}
	}
}

func readInitialState(ctx context.Context, path string) (statefile.State, error) {
	const (
		startupWait = 3 * time.Second
		retryEvery  = 100 * time.Millisecond
	)

	state, err := statefile.ReadState(path)
	if err == nil {
		return state, nil
	}
	lastErr := err

	timer := time.NewTimer(startupWait)
	defer timer.Stop()
	ticker := time.NewTicker(retryEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return statefile.State{}, ctx.Err()
		case <-timer.C:
			return statefile.State{}, lastErr
		case <-ticker.C:
			state, err = statefile.ReadState(path)
			if err == nil {
				return state, nil
			}
			lastErr = err
		}
	}
}

func applyStateFreshness(snapshot *model.Snapshot, writtenAt time.Time, interval time.Duration) {
	age := time.Duration(0)
	if !writtenAt.IsZero() {
		age = time.Since(writtenAt)
		if age < 0 {
			age = 0
		}
	}
	snapshot.StateAge = age
	snapshot.StateStale = writtenAt.IsZero() || age > 4*interval
}

func runClient(ctx context.Context, cfg app.Config) {
	state, err := readInitialState(ctx, cfg.StateFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nasmon: не удалось прочитать %s: %v\n", cfg.StateFile, err)
		fmt.Fprintln(os.Stderr, "запусти nasmond или используй nasmon --standalone")
		os.Exit(1)
	}

	snapshot := state.Snapshot
	lastWrittenAt := state.WrittenAt
	renderer := tui.Renderer{Config: cfg}
	clientStartedAt := time.Now()
	freshnessInterval := cfg.MainInterval
	if cfg.StateInterval > freshnessInterval {
		freshnessInterval = cfg.StateInterval
	}
	sortMode := tui.DockerSortDefault
	lastFrame := ""
	draw := func(s model.Snapshot) {
		s.StartedAt = clientStartedAt
		applyStateFreshness(&s, lastWrittenAt, freshnessInterval)
		rows, cols := tui.Size()
		lastFrame = renderer.RenderInteractive(s, rows, cols, sortMode)
		tui.Draw(lastFrame)
	}

	tui.Enter()
	defer tui.Leave()
	draw(snapshot)
	if cfg.OneShot {
		return
	}

	mouseEvents, stopMouse := tui.StartMouseInput()
	defer stopMouse()
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
			if next, readErr := statefile.ReadState(cfg.StateFile); readErr == nil {
				snapshot = next.Snapshot
				lastWrittenAt = next.WrittenAt
			}
			draw(snapshot)
		case <-winch:
			draw(snapshot)
		case ev, ok := <-mouseEvents:
			if !ok {
				mouseEvents = nil
				continue
			}
			if next, changed := tui.DockerSortForClick(lastFrame, ev.X, ev.Y, sortMode); changed {
				sortMode = next
				draw(snapshot)
			}
		}
	}
}
