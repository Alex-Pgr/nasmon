package collect

import (
	"testing"
	"time"
)

func fakeExists(names ...string) func(string) bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return func(name string) bool { return set[name] }
}

func TestChooseNetworkInterfaceFallsBackToDefaultRoute(t *testing.T) {
	got := chooseNetworkInterface("enp1s0f1", "", fakeExists("enp2s0"), func() string { return "enp2s0" })
	if got != "enp2s0" {
		t.Fatalf("chooseNetworkInterface() = %q, want enp2s0", got)
	}
}

func TestChooseNetworkInterfaceKeepsWorkingFallback(t *testing.T) {
	routeCalls := 0
	got := chooseNetworkInterface("enp1s0f1", "enp2s0", fakeExists("enp2s0"), func() string {
		routeCalls++
		return "wlan0"
	})
	if got != "enp2s0" {
		t.Fatalf("chooseNetworkInterface() = %q, want enp2s0", got)
	}
	if routeCalls != 0 {
		t.Fatalf("default route consulted %d times, want 0 while current interface is valid", routeCalls)
	}
}

func TestChooseNetworkInterfaceSwitchesBackToPreferred(t *testing.T) {
	got := chooseNetworkInterface("enp1s0f1", "enp2s0", fakeExists("enp1s0f1", "enp2s0"), func() string { return "enp2s0" })
	if got != "enp1s0f1" {
		t.Fatalf("chooseNetworkInterface() = %q, want preferred enp1s0f1", got)
	}
}

func TestSetInterfaceResetsTrafficBaseline(t *testing.T) {
	n := &NetworkCollector{
		iface:  "enp1s0f1",
		prevRX: 123,
		prevTX: 456,
		prevAt: time.Now(),
	}
	if !n.setInterfaceLocked("enp2s0") {
		t.Fatal("setInterfaceLocked() reported no change")
	}
	if n.iface != "enp2s0" || n.prevRX != 0 || n.prevTX != 0 || !n.prevAt.IsZero() {
		t.Fatalf("interface switch did not reset baseline: %+v", n)
	}
}
