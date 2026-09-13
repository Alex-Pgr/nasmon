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
	store := model.NewStore(time.Now(), cfg.StoragePath)
	store.Update(func(s *model.Snapshot) {
		s.DockerCollector.Enabled = true
		s.GPUCollector.Enabled = cfg.GPUHelper != ""
		s.SystemdCollector.Enabled = true
	})
	return &App{
		Config:  cfg,
		Store:   store,
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

func periodic(ctx context.Context, d time.Duration, immediate bool, fn func()) {
	if immediate {
		fn()
	}
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

func recordCollector(store *model.Store, enabled, success bool, pick func(*model.Snapshot) *model.CollectorStatus) {
	now := time.Now()
	store.Update(func(s *model.Snapshot) {
		status := pick(s)
		status.Enabled = enabled
		if !enabled {
			status.LastAttempt = time.Time{}
			status.LastSuccess = time.Time{}
			return
		}
		status.LastAttempt = now
		if success {
			status.LastSuccess = now
		}
	})
}

func (a *App) collectGPU() {
	enabled := a.Config.GPUHelper != ""
	success := false
	if enabled {
		success = collect.CollectGPU(a.Config.GPUHelper, a.Store)
	}
	recordCollector(a.Store, enabled, success, func(s *model.Snapshot) *model.CollectorStatus { return &s.GPUCollector })
}

func (a *App) collectDocker() {
	success := collect.CollectDocker(a.Store)
	recordCollector(a.Store, true, success, func(s *model.Snapshot) *model.CollectorStatus { return &s.DockerCollector })
}

func (a *App) collectSystemd() {
	success := collect.CollectSystemd(a.Store)
	recordCollector(a.Store, true, success, func(s *model.Snapshot) *model.CollectorStatus { return &s.SystemdCollector })
}

func (a *App) collectDiskTemps() {
	devs := a.Disk.SMARTDevices()
	enabled := len(devs) > 0
	success := false
	if enabled {
		success = collect.CollectDiskTemps(devs, a.Config.DiskQuietWindow, a.Store)
	}
	recordCollector(a.Store, enabled, success, func(s *model.Snapshot) *model.CollectorStatus { return &s.DiskTempCollector })
}

func (a *App) collectSMART() {
	devs := a.Disk.SMARTDevices()
	enabled := len(devs) > 0
	success := false
	if enabled {
		success = collect.CollectSMART(devs, a.Config.DiskQuietWindow, a.Store)
	}
	recordCollector(a.Store, enabled, success, func(s *model.Snapshot) *model.CollectorStatus { return &s.SMARTCollector })
}

// Bootstrap collects only cheap local metrics. External commands and Docker
// API calls start asynchronously in Start so a slow or unavailable optional
// integration cannot delay daemon startup or the first standalone TUI frame.
func (a *App) Bootstrap() {
	a.CPU.Collect(a.Store)
	collect.CollectMemory(a.Store)
	collect.CollectLoadUptime(a.Store)
	collect.CollectCPUTemp(a.Store)
	collect.CollectFanRPM(a.Store)
	a.Net.CollectIP(a.Store)
	a.Net.CollectTraffic(a.Store)
	a.Disk.CollectUsage(a.Store)
	collect.CollectDiskIdentities(a.Disk.HealthDevices(), a.Store)
	a.Disk.CollectIOAndActivity(a.Store)
	collect.CollectGPUUsage(a.Store)
}

func (a *App) Start(ctx context.Context) {
	main := func() {
		a.CPU.Collect(a.Store)
		collect.CollectMemory(a.Store)
		collect.CollectLoadUptime(a.Store)
		a.Net.CollectTraffic(a.Store)
		a.Disk.CollectIOAndActivity(a.Store)
		a.ping()
	}
	thermal := func() {
		collect.CollectCPUTemp(a.Store)
		collect.CollectFanRPM(a.Store)
		collect.CollectGPUUsage(a.Store)
		a.ping()
	}
	go periodic(ctx, a.Config.MainInterval, false, main)
	go periodic(ctx, a.Config.ThermalInterval, false, thermal)
	go periodic(ctx, a.Config.IPInterval, false, func() { a.Net.CollectIP(a.Store); a.ping() })
	if a.Config.GPUHelper != "" {
		go periodic(ctx, a.Config.GPUInterval, true, func() { a.collectGPU(); a.ping() })
	}
	go periodic(ctx, a.Config.DiskInterval, false, func() {
		a.Disk.CollectUsage(a.Store)
		collect.CollectDiskIdentities(a.Disk.HealthDevices(), a.Store)
		a.ping()
	})
	go periodic(ctx, a.Config.DiskPowerInterval, true, func() {
		collect.CollectDiskPowerState(a.Disk.HealthDevices(), a.Store)
		a.ping()
	})
	go periodic(ctx, a.Config.DiskTempInterval, true, func() { a.collectDiskTemps(); a.ping() })
	go periodic(ctx, a.Config.SMARTInterval, true, func() { a.collectSMART(); a.ping() })
	go periodic(ctx, a.Config.DockerInterval, true, func() { a.collectDocker(); a.ping() })
	go periodic(ctx, a.Config.SystemdInterval, true, func() { a.collectSystemd(); a.ping() })
}
