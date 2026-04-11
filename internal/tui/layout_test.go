package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/trainpulse/trainpulse/internal/store"
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
		Version:    "0.2.6",
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
		Version:    "0.2.6",
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

func TestPaneWidthRespected(t *testing.T) {
	m := newModel(Options{Version: "0.2.6"})
	layout := calcViewLayout(140, 36)
	if layout.mode != layoutModeDual {
		t.Fatalf("expected dual mode for test")
	}
	listPane := m.renderListPane(layout.leftW, layout.bodyH)
	if got := lipgloss.Width(listPane); got != layout.leftW {
		t.Fatalf("list pane width mismatch: got=%d want=%d", got, layout.leftW)
	}
	detailPane := m.renderDetailPane(layout.rightW, layout.bodyH)
	if got := lipgloss.Width(detailPane); got != layout.rightW {
		t.Fatalf("detail pane width mismatch: got=%d want=%d", got, layout.rightW)
	}
}

func TestJoinedWidthRespected(t *testing.T) {
	m := newModel(Options{Version: "0.2.6"})
	for _, size := range []struct {
		w int
		h int
	}{
		{w: 90, h: 24},
		{w: 120, h: 30},
		{w: 160, h: 40},
	} {
		layout := calcViewLayout(size.w, size.h)
		body := m.renderMainBody(layout)
		if got := lipgloss.Width(body); got > size.w {
			t.Fatalf("joined body overflow: got=%d limit=%d", got, size.w)
		}
	}
}

func TestLongFieldsNoOverflow(t *testing.T) {
	exit := 137
	pid := 12345
	m := newModel(Options{Version: "0.2.6"})
	m.runs = []store.Run{
		{
			RunID:         "run_very_long_identifier_abcdefghijklmnopqrstuvwxyz_0123456789",
			Status:        "FAILED",
			Event:         "FAILED",
			Project:       "project_with_extremely_long_name_that_should_not_break_layout",
			JobName:       "job_name_with_long_suffix_xxxxxxxxxxxxxxxxx",
			Host:          "my-host-with-a-very-very-long-name.example.internal",
			CWD:           "/mnt/share/path/to/a/very/long/workspace/with/many/segments/that/can/overflow",
			GitBranch:     "feature/super-long-branch-name-with-many-parts",
			GitCommit:     "abcdef0123456789abcdef0123456789abcdef01",
			LogPath:       "/var/log/trainpulse/some/really/long/path/to/log/file/output.log",
			LastHeartbeat: "2026-04-09T12:00:00+08:00",
			Cmd:           "python train.py --config configs/very_long_config_name.yaml --data /very/long/path --notes this_is_a_very_long_argument_that_should_wrap",
			ExitCode:      &exit,
			PID:           &pid,
			UpdatedAt:     "2026-04-09T12:00:00+08:00",
		},
	}
	m.selected = 0
	m.selectedRunID = m.runs[0].RunID

	pane := m.renderDetailPane(58, 18)
	for _, line := range strings.Split(pane, "\n") {
		if got := lipgloss.Width(line); got > 58 {
			t.Fatalf("detail line overflow: got=%d limit=%d line=%q", got, 58, line)
		}
	}
}

func TestResizeStability(t *testing.T) {
	m := newModel(Options{
		Version:    "0.2.6",
		StorePath:  "/tmp/trainpulse.db",
		ConfigPath: "/tmp/config.toml",
	})
	sequence := []struct {
		w int
		h int
	}{
		{w: 80, h: 18},
		{w: 140, h: 40},
		{w: 92, h: 22},
		{w: 180, h: 50},
		{w: 70, h: 14},
	}
	for _, s := range sequence {
		m.width = s.w
		m.height = s.h
		out := m.View()
		if got := lipgloss.Width(out); got > s.w {
			t.Fatalf("resize overflow width: got=%d limit=%d", got, s.w)
		}
		if got := lipgloss.Height(out); got > s.h {
			t.Fatalf("resize overflow height: got=%d limit=%d", got, s.h)
		}
	}
}
