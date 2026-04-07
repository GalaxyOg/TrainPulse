package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModalLogsManualScrollPausesFollow(t *testing.T) {
	m := newModel(Options{})
	m.modal = modalLogs
	m.height = 36
	m.logFollow = true
	m.logLines = makeTestLines(120)
	m.logOffset = 80

	next, _ := m.handleModalKeys(tea.KeyMsg{Type: tea.KeyPgUp})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("unexpected model type: %T", next)
	}
	if got.logFollow {
		t.Fatalf("expected follow=false after manual scroll")
	}
	if got.logOffset >= 80 {
		t.Fatalf("expected offset moved up, got %d", got.logOffset)
	}
}

func TestLogMsgOffsetClampAndFollowTail(t *testing.T) {
	m := newModel(Options{})
	m.height = 36
	m.logFollow = false
	m.logOffset = 50

	msg := logMsg{
		runID: "r1",
		path:  "/tmp/a.log",
		tail:  120,
		lines: makeTestLines(20),
	}
	next, _ := m.Update(msg)
	got, ok := next.(model)
	if !ok {
		t.Fatalf("unexpected model type: %T", next)
	}
	if got.logOffset != 8 {
		t.Fatalf("expected clamped offset=8, got %d", got.logOffset)
	}

	got.logFollow = true
	got.logOffset = 0
	next2, _ := got.Update(logMsg{
		runID: "r1",
		path:  "/tmp/a.log",
		tail:  120,
		lines: makeTestLines(40),
	})
	got2, ok := next2.(model)
	if !ok {
		t.Fatalf("unexpected model type: %T", next2)
	}
	if got2.logOffset != 28 {
		t.Fatalf("expected tail offset=28 in follow mode, got %d", got2.logOffset)
	}
}

func makeTestLines(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("line-%d", i))
	}
	return out
}
