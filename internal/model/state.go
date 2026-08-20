package model

import (
	"sync"
	"time"
)

type DiskUsage struct {
	Path       string
	Filesystem string
	UsedBytes  uint64
	TotalBytes uint64
	Percent    int
}

type DiskHealth struct {
	Device      string
	Temperature string
	Health      string
	Reallocated int64
	Pending     int64
	Uncorrect   int64
	Sleeping    bool
}

type Container struct {
	ID       string
	Name     string
	Status   string
	State    string
	Health   string
	Restarts int
}

type Snapshot struct {
	UpdatedAt time.Time
	StartedAt time.Time

	Uptime time.Duration

	CPUUsage   int
	CPUPercent int
	CPUTempC   *float64
	IOWait     int

	GPUTempC *float64
	GPUVCN   string

	MemUsedBytes   uint64
	MemTotalBytes  uint64
	MemPercent     int
	ZRAMUsedBytes  uint64
	ZRAMTotalBytes uint64
	ZRAMPercent    int
	SwapUsedBytes  uint64
	SwapTotalBytes uint64
	SwapPercent    int

	Load1  string
	Load5  string
	Load15 string

	Interface string
	IP        string
	RXBps     int64
	TXBps     int64
	NetReady  bool

	DiskReadBps  int64
	DiskWriteBps int64
	DiskIOReady  bool

	DiskUsage  []DiskUsage
	DiskHealth []DiskHealth
	Containers []Container

	StoragePath       string
	StorageUsedBytes  uint64
	StorageTotalBytes uint64
	StoragePercent    int

	FailedUnits int
}

type Store struct {
	mu sync.RWMutex
	s  Snapshot
}

func NewStore(startedAt time.Time, storagePath string) *Store {
	return &Store{s: Snapshot{StartedAt: startedAt, StoragePath: storagePath, GPUVCN: "N/A"}}
}

func (s *Store) Update(fn func(*Snapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.s)
	s.s.UpdatedAt = time.Now()
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.s
	out.DiskUsage = append([]DiskUsage(nil), s.s.DiskUsage...)
	out.DiskHealth = append([]DiskHealth(nil), s.s.DiskHealth...)
	out.Containers = append([]Container(nil), s.s.Containers...)
	return out
}
