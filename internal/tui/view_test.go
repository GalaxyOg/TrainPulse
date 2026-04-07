package tui

import "testing"

func TestLogWindowBounds(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}

	start, end, shown := logWindow(lines, -10, 2)
	if start != 0 || end != 2 || len(shown) != 2 || shown[0] != "a" || shown[1] != "b" {
		t.Fatalf("unexpected window for negative offset: start=%d end=%d shown=%v", start, end, shown)
	}

	start, end, shown = logWindow(lines, 100, 2)
	if start != 3 || end != 5 || len(shown) != 2 || shown[0] != "d" || shown[1] != "e" {
		t.Fatalf("unexpected window for high offset: start=%d end=%d shown=%v", start, end, shown)
	}

	start, end, shown = logWindow(nil, 0, 10)
	if start != 0 || end != 0 || len(shown) != 0 {
		t.Fatalf("unexpected window for empty lines: start=%d end=%d shown=%v", start, end, shown)
	}
}
