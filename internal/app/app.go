package app

import (
	"context"
	"time"

	"nasmon/internal/collect"
	"nasmon/internal/model"
)

type App struct {
	Config  Config
	Store   *model.Store
	CPU     *collect.CPUCollector
	Net     *collect.NetworkCollector
	Disk    *collect.DiskCollector
	Updates chan struct{}
}

func New(cfg Config) *App {
	return &App{
		Config:  cfg,
		Store:   model.NewStore(time.Now(), cfg.StoragePath),
		CPU:     &collect.CPUCollector{},
		Net:     collect.NewNetworkCollector(cfg.Interface),
		Disk:    collect.NewDiskCollector(cfg.DiskPaths, cfg.StoragePath),
		Updates: make(chan struct{}, 1),
	}
}

func (a *App) ping() {
	select {
	case a.Updates <- struct{}{}:
	default:
	}
}

func periodic(ctx context.Context, d time.Duration, fn func()) {
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fn()
		}
	}
}

// Bootstrap mirrors the old Bash behavior: collect a coherent first snapshot
// before the first frame is drawn. After that every collector runs independently.
func (a *App) Bootstrap() {
	a.CPU.Collect(a.Store)
	collect.CollectMemory(a.Store)
	collect.CollectLoadUptime(a.Store)
	collect.CollectCPUTemp(a.Store)
	a.Net.CollectIP(a.Store)
	a.Net.CollectTraffic(a.Store)
	a.Disk.CollectUsage(a.Store)
	a.Disk.CollectIO(a.Store)
	collect.CollectGPU(a.Config.GPUHelper, a.Store)
	collect.CollectDiskTemps(a.Disk.Devices(), a.Store)
	collect.CollectSMART(a.Disk.Devices(), a.Store)
	collect.CollectDocker(a.Store)
	collect.CollectSystemd(a.Store)
}

func (a *App) Start(ctx context.Context) {
	main := func() {
		a.CPU.Collect(a.Store)
		collect.CollectMemory(a.Store)
		collect.CollectLoadUptime(a.Store)
		collect.CollectCPUTemp(a.Store)
		a.Net.CollectTraffic(a.Store)
		a.Disk.CollectIO(a.Store)
		a.ping()
	}
	go periodic(ctx, a.Config.MainInterval, main)
	go periodic(ctx, a.Config.IPInterval, func() { a.Net.CollectIP(a.Store); a.ping() })
	go periodic(ctx, a.Config.GPUInterval, func() { collect.CollectGPU(a.Config.GPUHelper, a.Store); a.ping() })
	go periodic(ctx, a.Config.DiskInterval, func() { a.Disk.CollectUsage(a.Store); a.ping() })
	go periodic(ctx, a.Config.DiskTempInterval, func() { collect.CollectDiskTemps(a.Disk.Devices(), a.Store); a.ping() })
	go periodic(ctx, a.Config.SMARTInterval, func() { collect.CollectSMART(a.Disk.Devices(), a.Store); a.ping() })
	go periodic(ctx, a.Config.DockerInterval, func() { collect.CollectDocker(a.Store); a.ping() })
	go periodic(ctx, a.Config.SystemdInterval, func() { collect.CollectSystemd(a.Store); a.ping() })
}
