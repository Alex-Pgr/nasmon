package collect

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"nasmon/internal/model"
)

type mountInfo struct{ MountPoint, Source, FSType string }

func unescapeMount(s string) string {
	r := strings.NewReplacer("\\040", " ", "\\011", "\t", "\\012", "\n", "\\134", "\\")
	return r.Replace(s)
}

func readMounts() []mountInfo {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []mountInfo
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		p := strings.Fields(sc.Text())
		sep := -1
		for i, x := range p {
			if x == "-" {
				sep = i
				break
			}
		}
		if sep < 0 || sep+2 >= len(p) || len(p) < 5 {
			continue
		}
		out = append(out, mountInfo{MountPoint: unescapeMount(p[4]), FSType: p[sep+1], Source: unescapeMount(p[sep+2])})
	}
	return out
}

func mountForPath(path string, mounts []mountInfo) (mountInfo, bool) {
	clean := filepath.Clean(path)
	best := -1
	var bm mountInfo
	for _, m := range mounts {
		mp := filepath.Clean(m.MountPoint)
		if clean == mp || (strings.HasPrefix(clean, mp+string(os.PathSeparator))) || mp == "/" {
			if len(mp) > best {
				best = len(mp)
				bm = m
			}
		}
	}
	return bm, best >= 0
}

func statFS(path string) (used, total uint64, pct int, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return
	}
	total = st.Blocks * uint64(st.Bsize)
	free := st.Bfree * uint64(st.Bsize)
	avail := st.Bavail * uint64(st.Bsize)
	if total >= free {
		used = total - free
	}
	// Match df semantics: percentage is based on used + user-available blocks,
	// excluding filesystem-reserved space from the denominator.
	denom := used + avail
	if denom > 0 {
		pct = int((used*100 + denom - 1) / denom)
	}
	ok = true
	return
}

func physicalBlock(source string) string {
	if !strings.HasPrefix(source, "/dev/") {
		return ""
	}
	dev := filepath.Base(source)
	link, err := filepath.EvalSymlinks(filepath.Join("/sys/class/block", dev))
	if err != nil {
		return dev
	}
	if _, err := os.Stat(filepath.Join("/sys/class/block", dev, "partition")); err == nil {
		return filepath.Base(filepath.Dir(link))
	}
	return dev
}

func discoverNVMeBlockDevices() []string {
	entries, err := filepath.Glob("/sys/class/block/nvme*n*")
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range entries {
		dev := filepath.Base(entry)
		if _, err := os.Stat(filepath.Join(entry, "partition")); err == nil {
			continue
		}
		if _, err := os.Stat(filepath.Join("/dev", dev)); err != nil {
			continue
		}
		out = append(out, dev)
	}
	sort.Strings(out)
	return out
}

// autoDiskMount selects real block-device mounts while excluding pseudo and
// loop-backed filesystems such as snap squashfs images. Explicit configured
// paths are still collected separately, preserving the previous behavior.
func autoDiskMount(m mountInfo) bool {
	if filepath.Clean(m.MountPoint) == "/boot/efi" {
		return false
	}
	source := filepath.Clean(m.Source)
	if !strings.HasPrefix(source, "/dev/") {
		return false
	}
	dev := filepath.Base(source)
	if strings.HasPrefix(dev, "loop") || strings.HasPrefix(dev, "zram") {
		return false
	}
	return m.FSType != "squashfs"
}

func diskUsagePriority(path string) int {
	switch filepath.Clean(path) {
	case "/":
		return 0
	case "/mnt/ssd":
		return 1
	case "/mnt/fast":
		return 2
	case "/mnt/hdd":
		return 3
	default:
		return 4
	}
}

func sortDiskUsage(usage []model.DiskUsage) {
	sort.SliceStable(usage, func(i, j int) bool {
		pi := diskUsagePriority(usage[i].Path)
		pj := diskUsagePriority(usage[j].Path)
		if pi != pj {
			return pi < pj
		}
		return strings.ToLower(usage[i].Path) < strings.ToLower(usage[j].Path)
	})
}

type DiskCollector struct {
	mu                  sync.Mutex
	paths               []string
	storagePath         string
	devices             []string
	healthDevices       []string
	prevRead, prevWrite uint64
	prevAt              time.Time
}

func NewDiskCollector(paths []string, storage string) *DiskCollector {
	return &DiskCollector{paths: paths, storagePath: storage}
}
func (d *DiskCollector) Devices() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.devices...)
}
func (d *DiskCollector) HealthDevices() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.healthDevices...)
}

func (d *DiskCollector) CollectUsage(store *model.Store) {
	mounts := readMounts()
	seen := map[string]bool{}
	devSeen := map[string]bool{}
	var usage []model.DiskUsage
	var devices []string

	addMount := func(m mountInfo, statPath string) {
		mp := filepath.Clean(m.MountPoint)
		if seen[mp] {
			return
		}
		u, t, pct, ok := statFS(statPath)
		if !ok {
			return
		}
		seen[mp] = true
		usage = append(usage, model.DiskUsage{Path: mp, Filesystem: m.Source, UsedBytes: u, TotalBytes: t, Percent: pct})
		if dev := physicalBlock(m.Source); dev != "" && !devSeen[dev] {
			devSeen[dev] = true
			devices = append(devices, dev)
		}
	}

	// Preserve the old explicitly configured paths first.
	for _, p := range d.paths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		m, ok := mountForPath(p, mounts)
		if !ok {
			continue
		}
		addMount(m, p)
	}

	// Then add other local block-device mounts automatically.
	for _, m := range mounts {
		if !autoDiskMount(m) {
			continue
		}
		if _, err := os.Stat(m.MountPoint); err != nil {
			continue
		}
		addMount(m, m.MountPoint)
	}
	sortDiskUsage(usage)

	var su, st uint64
	sp := 0
	if u, t, pct, ok := statFS(d.storagePath); ok {
		su, st, sp = u, t, pct
	}

	healthDevices := append([]string(nil), devices...)
	for _, dev := range discoverNVMeBlockDevices() {
		if !devSeen[dev] {
			devSeen[dev] = true
			healthDevices = append(healthDevices, dev)
		}
	}
	sort.Strings(healthDevices)
	d.mu.Lock()
	d.devices = devices
	d.healthDevices = healthDevices
	d.mu.Unlock()
	store.Update(func(s *model.Snapshot) {
		s.DiskUsage = usage
		s.StorageUsedBytes = su
		s.StorageTotalBytes = st
		s.StoragePercent = sp
	})
}

func (d *DiskCollector) CollectIO(store *model.Store) {
	devs := d.Devices()
	if len(devs) == 0 {
		return
	}
	wanted := map[string]bool{}
	for _, x := range devs {
		wanted[x] = true
	}
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return
	}
	defer f.Close()
	var rsec, wsec uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		p := strings.Fields(sc.Text())
		if len(p) < 10 || !wanted[p[2]] {
			continue
		}
		r, _ := strconv.ParseUint(p[5], 10, 64)
		w, _ := strconv.ParseUint(p[9], 10, 64)
		rsec += r
		wsec += w
	}
	now := time.Now()
	d.mu.Lock()
	ready := !d.prevAt.IsZero()
	var rb, wb int64
	if ready {
		sec := now.Sub(d.prevAt).Seconds()
		if sec < 0.001 {
			sec = 0.001
		}
		rb = int64(float64(int64(rsec)-int64(d.prevRead)) * 512 / sec)
		wb = int64(float64(int64(wsec)-int64(d.prevWrite)) * 512 / sec)
		if rb < 0 {
			rb = 0
		}
		if wb < 0 {
			wb = 0
		}
	}
	d.prevRead, d.prevWrite, d.prevAt = rsec, wsec, now
	d.mu.Unlock()
	store.Update(func(s *model.Snapshot) { s.DiskReadBps = rb; s.DiskWriteBps = wb; s.DiskIOReady = ready })
}
