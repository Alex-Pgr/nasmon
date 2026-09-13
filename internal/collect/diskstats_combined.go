package collect

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"

	"nasmon/internal/model"
)

type diskSectorCounters struct {
	read  uint64
	write uint64
}

func readDiskSectorCounters(devs []string) (map[string]diskSectorCounters, bool) {
	if len(devs) == 0 {
		return map[string]diskSectorCounters{}, true
	}
	wanted := make(map[string]bool, len(devs))
	for _, dev := range devs {
		wanted[dev] = true
	}

	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, false
	}
	defer f.Close()

	current := make(map[string]diskSectorCounters, len(devs))
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 10 || !wanted[fields[2]] {
			continue
		}
		read, errRead := strconv.ParseUint(fields[5], 10, 64)
		write, errWrite := strconv.ParseUint(fields[9], 10, 64)
		if errRead != nil || errWrite != nil {
			continue
		}
		current[fields[2]] = diskSectorCounters{read: read, write: write}
	}
	if err := sc.Err(); err != nil {
		return nil, false
	}
	return current, true
}

func (d *DiskCollector) updateIOFromCounters(store *model.Store, current map[string]diskSectorCounters, now time.Time) {
	var readSectors, writeSectors uint64
	for _, counters := range current {
		readSectors += counters.read
		writeSectors += counters.write
	}

	d.mu.Lock()
	ready := !d.prevAt.IsZero()
	var readBps, writeBps int64
	if ready {
		seconds := now.Sub(d.prevAt).Seconds()
		if seconds < 0.001 {
			seconds = 0.001
		}
		readBps = int64(float64(int64(readSectors)-int64(d.prevRead)) * 512 / seconds)
		writeBps = int64(float64(int64(writeSectors)-int64(d.prevWrite)) * 512 / seconds)
		if readBps < 0 {
			readBps = 0
		}
		if writeBps < 0 {
			writeBps = 0
		}
	}
	d.prevRead, d.prevWrite, d.prevAt = readSectors, writeSectors, now
	d.mu.Unlock()

	store.Update(func(s *model.Snapshot) {
		s.DiskReadBps = readBps
		s.DiskWriteBps = writeBps
		s.DiskIOReady = ready
	})
}

func updateDiskActivityFromCounters(devs []string, current map[string]diskSectorCounters, now time.Time) {
	diskActivityMu.Lock()
	defer diskActivityMu.Unlock()

	wanted := make(map[string]bool, len(devs))
	for _, dev := range devs {
		wanted[dev] = true
		counters, ok := current[dev]
		if !ok {
			continue
		}
		state := diskActivity[dev]
		if !state.initialized {
			state.initialized = true
			state.lastActivity = now
		} else if counters.read != state.readSectors || counters.write != state.writeSectors {
			state.lastActivity = now
		}
		state.readSectors = counters.read
		state.writeSectors = counters.write
		diskActivity[dev] = state
	}
	for dev := range diskActivity {
		if !wanted[dev] {
			delete(diskActivity, dev)
		}
	}
}

// CollectIOAndActivity samples /proc/diskstats once and feeds both throughput
// and quiet-disk activity tracking from the same cumulative counters.
func (d *DiskCollector) CollectIOAndActivity(store *model.Store) {
	devs := d.Devices()
	if len(devs) == 0 {
		return
	}
	current, ok := readDiskSectorCounters(devs)
	if !ok {
		return
	}
	now := time.Now()
	d.updateIOFromCounters(store, current, now)
	updateDiskActivityFromCounters(devs, current, now)
}
