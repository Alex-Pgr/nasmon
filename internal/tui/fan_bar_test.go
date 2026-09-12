package tui

import (
	"strings"
	"testing"
)

func TestShortenCPURAMBarsByTwoCells(t *testing.T) {
	frame := "│ CPU 10% [████░░░░]\n│ RAM 20% [██░░░░░░] 4.0/16.0G\n│ ZRAM 5% [█░░░░░░░] 0.2/4.0G\n"
	out := shortenCPURAMBars(frame)
	plain := plainTerminalLine(out)
	if !strings.Contains(plain, "CPU 10% [████░░]  ") {
		t.Fatalf("CPU bar was not shortened by two cells: %q", plain)
	}
	if !strings.Contains(plain, "RAM 20% [██░░░░]   4.0/16.0G") {
		t.Fatalf("RAM bar was not shortened by two cells: %q", plain)
	}
	if !strings.Contains(plain, "ZRAM 5% [█░░░░░░░] 0.2/4.0G") {
		t.Fatalf("ZRAM bar must keep its original width: %q", plain)
	}
}

func TestFanRPMUsesGrayStyle(t *testing.T) {
	rpm := 2200
	frame := "│ CPU 10% [████░░░░]\n"
	out := decorateCPUFan(frame, 80, &rpm)
	if !strings.Contains(out, gray+"Fan 2200 rpm"+reset) {
		t.Fatalf("fan RPM is not rendered with gray style")
	}
}
