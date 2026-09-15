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

	"github.com/Alex-Pgr/nasmon/internal/model"
)

type NetworkCollector struct {
	mu             sync.Mutex
	preferred      string
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

func interfaceExists(name string) bool {
	if name == "" {
		return false
	}
	_, err := os.Stat(filepath.Join("/sys/class/net", name))
	return err == nil
}

func chooseNetworkInterface(preferred, current string, exists func(string) bool, route func() string) string {
	if preferred != "" && exists(preferred) {
		return preferred
	}
	if current != "" && exists(current) {
		return current
	}
	if fallback := route(); fallback != "" && exists(fallback) {
		return fallback
	}
	return ""
}

func NewNetworkCollector(preferred string) *NetworkCollector {
	n := &NetworkCollector{preferred: preferred}
	n.iface = chooseNetworkInterface(preferred, "", interfaceExists, defaultRouteInterface)
	return n
}

func (n *NetworkCollector) setInterfaceLocked(iface string) bool {
	if iface == n.iface {
		return false
	}
	n.iface = iface
	n.prevRX = 0
	n.prevTX = 0
	n.prevAt = time.Time{}
	return true
}

func (n *NetworkCollector) resolveInterface() (string, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	next := chooseNetworkInterface(n.preferred, n.iface, interfaceExists, defaultRouteInterface)
	changed := n.setInterfaceLocked(next)
	return n.iface, changed
}

func (n *NetworkCollector) invalidateInterface(iface string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.iface == iface {
		n.setInterfaceLocked("")
	}
}

func readUint(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
}

func ipv4ForInterface(iface string) (string, error) {
	ni, err := net.InterfaceByName(iface)
	if err != nil {
		return "", err
	}
	addrs, err := ni.Addrs()
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		var parsed net.IP
		switch v := a.(type) {
		case *net.IPNet:
			parsed = v.IP
		case *net.IPAddr:
			parsed = v.IP
		}
		if parsed != nil && parsed.To4() != nil {
			return parsed.String(), nil
		}
	}
	return "", nil
}

func clearNetworkState(store *model.Store) {
	store.Update(func(s *model.Snapshot) {
		s.Interface = ""
		s.IP = ""
		s.RXBps = 0
		s.TXBps = 0
		s.NetReady = false
	})
}

func (n *NetworkCollector) collectIPResolved(iface string, store *model.Store) {
	ip, err := ipv4ForInterface(iface)
	if err != nil {
		n.invalidateInterface(iface)
		clearNetworkState(store)
		return
	}
	store.Update(func(s *model.Snapshot) {
		s.Interface = iface
		s.IP = ip
	})
}

func (n *NetworkCollector) CollectTraffic(store *model.Store) {
	iface, changed := n.resolveInterface()
	if iface == "" {
		clearNetworkState(store)
		return
	}
	rx, e1 := readUint(filepath.Join("/sys/class/net", iface, "statistics/rx_bytes"))
	tx, e2 := readUint(filepath.Join("/sys/class/net", iface, "statistics/tx_bytes"))
	if e1 != nil || e2 != nil {
		n.invalidateInterface(iface)
		clearNetworkState(store)
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
	n.mu.Unlock()
	store.Update(func(s *model.Snapshot) {
		s.Interface = iface
		s.RXBps = rb
		s.TXBps = tb
		s.NetReady = ready
	})

	// During early boot the daemon may start before the interface has an IPv4
	// address. Once traffic collection discovers or switches to an interface,
	// refresh IP immediately instead of waiting for the slower IP interval.
	if changed || store.Snapshot().IP == "" {
		n.collectIPResolved(iface, store)
	}
}

func (n *NetworkCollector) CollectIP(store *model.Store) {
	iface, _ := n.resolveInterface()
	if iface == "" {
		clearNetworkState(store)
		return
	}
	n.collectIPResolved(iface, store)
}
