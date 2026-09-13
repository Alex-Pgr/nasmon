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

func TestTerminalCellWidthHandlesUnicode(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"ASCII", 5},
		{"磁盘", 4},
		{"e\u0301", 1},
		{"🙂", 2},
		{"👩‍💻", 2},
	}
	for _, tc := range cases {
		if got := cellWidth(tc.text); got != tc.want {
			t.Fatalf("cellWidth(%q) = %d, want %d", tc.text, got, tc.want)
		}
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

func TestTruncateCellsHandlesWideAndCombiningText(t *testing.T) {
	if got := stripANSI(truncateCells("磁盘data", 5)); got != "磁盘…" {
		t.Fatalf("wide truncate = %q, want 磁盘…", got)
	}
	if got := stripANSI(truncateCells("e\u0301abcd", 4)); got != "e\u0301ab…" {
		t.Fatalf("combining truncate = %q, want éab…", got)
	}
}

func TestCellIndexUsesDisplayCells(t *testing.T) {
	if got := cellIndex("磁盘 name", "name"); got != 5 {
		t.Fatalf("cellIndex = %d, want 5", got)
	}
}

func TestCellPaddingUsesDisplayWidth(t *testing.T) {
	if got := cellWidth(padRightCells("磁盘", 6)); got != 6 {
		t.Fatalf("right padded width = %d, want 6", got)
	}
	if got := cellWidth(padLeftCells("🙂", 4)); got != 4 {
		t.Fatalf("left padded width = %d, want 4", got)
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
