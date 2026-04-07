package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestCalcViewLayoutModes(t *testing.T) {
	tiny := calcViewLayout(40, 7)
	if tiny.mode != layoutModeTiny {
		t.Fatalf("expected tiny mode, got %v", tiny.mode)
	}

	single := calcViewLayout(90, 26)
	if single.mode != layoutModeSingle {
		t.Fatalf("expected single mode, got %v", single.mode)
	}
	if single.listH <= 0 {
		t.Fatalf("expected positive listH in single mode")
	}

	dual := calcViewLayout(140, 36)
	if dual.mode != layoutModeDual {
		t.Fatalf("expected dual mode, got %v", dual.mode)
	}
	if dual.leftW <= 0 || dual.rightW <= 0 || dual.leftW+dual.rightW+1 != dual.totalW {
		t.Fatalf("invalid dual widths: left=%d right=%d total=%d", dual.leftW, dual.rightW, dual.totalW)
	}
}

func TestViewFitsTerminalCanvas(t *testing.T) {
	m := newModel(Options{
		Version:    "0.2.3",
		StorePath:  "/tmp/trainpulse.db",
		ConfigPath: "/tmp/config.toml",
	})

	for _, tc := range []struct {
		w int
		h int
	}{
		{w: 60, h: 14},
		{w: 96, h: 26},
		{w: 180, h: 50},
	} {
		m.width = tc.w
		m.height = tc.h
		out := m.View()
		if got := lipgloss.Width(out); got > tc.w {
			t.Fatalf("view width overflow: got=%d limit=%d", got, tc.w)
		}
		if got := lipgloss.Height(out); got > tc.h {
			t.Fatalf("view height overflow: got=%d limit=%d", got, tc.h)
		}
	}
}

func TestViewWithModalFitsTerminalCanvas(t *testing.T) {
	m := newModel(Options{
		Version:    "0.2.3",
		StorePath:  "/tmp/trainpulse.db",
		ConfigPath: "/tmp/config.toml",
	})
	m.modal = modalInfo
	m.modalTitle = "Doctor"
	m.modalBody = "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"

	m.width = 72
	m.height = 16
	out := m.View()
	if got := lipgloss.Width(out); got > m.width {
		t.Fatalf("view width overflow in modal: got=%d limit=%d", got, m.width)
	}
	if got := lipgloss.Height(out); got > m.height {
		t.Fatalf("view height overflow in modal: got=%d limit=%d", got, m.height)
	}
}
