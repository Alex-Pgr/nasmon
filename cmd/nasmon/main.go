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
	"nasmon/internal/tui"
)

func main() {
	cfg := app.DefaultConfig()
	if len(os.Args) > 1 {
		n, err := strconv.Atoi(os.Args[1])
		if err != nil || n < 1 {
			fmt.Fprintln(os.Stderr, "интервал должен быть числом >= 1")
			os.Exit(2)
		}
		if n > 3600 {
			n = 3600
		}
		cfg.MainInterval = time.Duration(n) * time.Second
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	a := app.New(cfg)
	renderer := tui.Renderer{Config: cfg}
	a.Bootstrap()
	a.Start(ctx)
	tui.Enter()
	defer tui.Leave()
	draw := func() { rows, cols := tui.Size(); tui.Draw(renderer.Render(a.Store.Snapshot(), rows, cols)) }
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
