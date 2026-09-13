package tui

import "testing"

func TestTerminalTextLayerIgnoresANSIAndControls(t *testing.T) {
	s := "\033[2K\r\033[0;32mhello\033[0m\n"
	if got := stripANSI(s); got != "hello" {
		t.Fatalf("stripANSI = %q, want hello", got)
	}
	if got := cellWidth(s); got != 5 {
		t.Fatalf("cellWidth = %d, want 5", got)
	}
	if got := cellIndex(s, "llo"); got != 2 {
		t.Fatalf("cellIndex = %d, want 2", got)
	}
}

func TestTruncateCellsPreservesWidthAndStylePrefix(t *testing.T) {
	s := green + "abcdef" + reset
	got := truncateCells(s, 4)
	if width := cellWidth(got); width != 4 {
		t.Fatalf("truncated width = %d, want 4: %q", width, got)
	}
	if plain := stripANSI(got); plain != "abc…" {
		t.Fatalf("truncated text = %q, want abc…", plain)
	}
	if got[:len(green)] != green {
		t.Fatalf("leading style was not preserved: %q", got)
	}
}

func TestTruncateCellsDoesNotCountCarriageReturn(t *testing.T) {
	s := "\r123456"
	if got := cellWidth(s); got != 6 {
		t.Fatalf("cellWidth with CR = %d, want 6", got)
	}
	if got := stripANSI(truncateCells(s, 4)); got != "123…" {
		t.Fatalf("truncateCells with CR = %q, want 123…", got)
	}
}
