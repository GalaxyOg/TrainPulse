package tui

import (
	"time"

	"github.com/trainpulse/trainpulse/internal/store"
)

type model struct {
	opts           Options
	styles         styles
	width          int
	height         int
	focus          focusArea
	modal          modalType
	runs           []store.Run
	counts         map[string]int
	selected       int
	statusChips    []string
	chipIndex      int
	filterStatuses []string
	since24h       bool
	projectQuery   string
	jobQuery       string
	searchInput    string
	selectedRunID  string
	notice         string
	noticeIsErr    bool
	lastRefresh    time.Time
	autoRefresh    bool
	lastFailed     string
	lastActive     string
	modalTitle     string
	modalBody      string
	confirmRunID   string
	setup          setupState
	logRunID       string
	logPath        string
	logTailLines   int
	logFollow      bool
	logLines       []string
	logOffset      int
	cleanupIndex   int
	errorSummary   map[string]string
}

func (m model) selectedRun() *store.Run {
	if len(m.runs) == 0 {
		return nil
	}
	if m.selected < 0 {
		return nil
	}
	if m.selected >= len(m.runs) {
		return nil
	}
	return &m.runs[m.selected]
}
