package collect

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"nasmon/internal/model"
)

type NetworkCollector struct {
	mu             sync.Mutex
	iface          string
	prevRX, prevTX uint64
	prevAt         time.Time
}

func defaultRouteInterface() string {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if sc.Scan() {
	} // header
	for sc.Scan() {
		p := strings.Fields(sc.Text())
		if len(p) >= 2 && p[1] == "00000000" {
			return p[0]
		}
	}
	return ""
}

func NewNetworkCollector(preferred string) *NetworkCollector {
	if _, err := os.Stat(filepath.Join("/sys/class/net", preferred)); err != nil {
		preferred = defaultRouteInterface()
	}
	return &NetworkCollector{iface: preferred}
}

func readUint(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
}

func (n *NetworkCollector) CollectTraffic(store *model.Store) {
	if n.iface == "" {
		return
	}
	rx, e1 := readUint(filepath.Join("/sys/class/net", n.iface, "statistics/rx_bytes"))
	tx, e2 := readUint(filepath.Join("/sys/class/net", n.iface, "statistics/tx_bytes"))
	if e1 != nil || e2 != nil {
		return
	}
	now := time.Now()
	n.mu.Lock()
	ready := !n.prevAt.IsZero()
	var rb, tb int64
	if ready {
		sec := now.Sub(n.prevAt).Seconds()
		if sec < 0.001 {
			sec = 0.001
		}
		rb = int64(float64(int64(rx)-int64(n.prevRX)) / sec)
		tb = int64(float64(int64(tx)-int64(n.prevTX)) / sec)
		if rb < 0 {
			rb = 0
		}
		if tb < 0 {
			tb = 0
		}
	}
	n.prevRX, n.prevTX, n.prevAt = rx, tx, now
	iface := n.iface
	n.mu.Unlock()
	store.Update(func(s *model.Snapshot) { s.Interface = iface; s.RXBps = rb; s.TXBps = tb; s.NetReady = ready })
}

func (n *NetworkCollector) CollectIP(store *model.Store) {
	if n.iface == "" {
		return
	}
	ni, err := net.InterfaceByName(n.iface)
	if err != nil {
		return
	}
	addrs, err := ni.Addrs()
	if err != nil {
		return
	}
	ip := ""
	for _, a := range addrs {
		var parsed net.IP
		switch v := a.(type) {
		case *net.IPNet:
			parsed = v.IP
		case *net.IPAddr:
			parsed = v.IP
		}
		if parsed != nil && parsed.To4() != nil {
			ip = parsed.String()
			break
		}
	}
	store.Update(func(s *model.Snapshot) { s.Interface = n.iface; s.IP = ip })
}
