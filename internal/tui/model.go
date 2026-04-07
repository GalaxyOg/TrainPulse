package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/trainpulse/trainpulse/internal/config"
)

func newModel(opts Options) model {
	interval := opts.RefreshInterval
	if interval <= 0 {
		interval = 3 * time.Second
	}
	m := model{
		opts:         opts,
		styles:       defaultStyles(),
		focus:        focusList,
		modal:        modalNone,
		counts:       map[string]int{},
		selected:     0,
		statusChips:  []string{"all", "running", "failed", "succeeded", "interrupted", "stopped"},
		chipIndex:    0,
		autoRefresh:  true,
		lastRefresh:  time.Time{},
		notice:       "ready",
		noticeIsErr:  false,
		logTailLines: 120,
		logFollow:    false,
		errorSummary: map[string]string{},
	}
	m.opts.RefreshInterval = interval
	if strings.TrimSpace(m.opts.ConfigPath) == "" {
		m.opts.ConfigPath = "~/.config/trainpulse/config.toml"
	}
	if strings.TrimSpace(m.opts.ErrorLogPath) == "" {
		m.opts.ErrorLogPath = config.DefaultErrorLogPath
	}
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.refreshCmd(), tickEverySecond())
}

func tickEverySecond() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) refreshCmd() tea.Cmd {
	statuses := append([]string{}, m.filterStatuses...)
	since := ""
	if m.since24h {
		since = time.Now().Add(-24 * time.Hour).In(time.FixedZone("UTC+8", 8*3600)).Format(time.RFC3339)
	}
	project := m.projectQuery
	job := m.jobQuery
	st := m.opts.Store
	return func() tea.Msg {
		if st == nil {
			return refreshMsg{err: fmt.Errorf("store is nil")}
		}
		runs, err := st.ListByFilters(statuses, since, project, job, 500)
		if err != nil {
			return refreshMsg{err: err}
		}
		all, err := st.ListByFilters(nil, "", "", "", 4000)
		if err != nil {
			all = runs
		}
		counts := map[string]int{
			"RUNNING":     0,
			"FAILED":      0,
			"SUCCEEDED":   0,
			"INTERRUPTED": 0,
			"STOPPED":     0,
		}
		lastFailed := ""
		lastActive := ""
		for _, r := range all {
			counts[strings.ToUpper(strings.TrimSpace(r.Status))]++
			if moreRecent(r.UpdatedAt, lastActive) {
				lastActive = r.UpdatedAt
			}
			if strings.EqualFold(strings.TrimSpace(r.Status), "FAILED") {
				if moreRecent(r.UpdatedAt, lastFailed) {
					lastFailed = r.UpdatedAt
				}
			}
		}
		return refreshMsg{runs: runs, counts: counts, lastFailed: lastFailed, lastActive: lastActive}
	}
}

func (m *model) setNotice(msg string, isErr bool) {
	m.notice = strings.TrimSpace(msg)
	if m.notice == "" {
		m.notice = "ok"
	}
	m.noticeIsErr = isErr
}

func (m *model) applyChipFilter() {
	if m.chipIndex < 0 {
		m.chipIndex = 0
	}
	if m.chipIndex >= len(m.statusChips) {
		m.chipIndex = len(m.statusChips) - 1
	}
	chip := m.statusChips[m.chipIndex]
	switch chip {
	case "all":
		m.filterStatuses = nil
	case "running":
		m.filterStatuses = []string{"RUNNING"}
	case "failed":
		m.filterStatuses = []string{"FAILED"}
	case "succeeded":
		m.filterStatuses = []string{"SUCCEEDED"}
	case "interrupted":
		m.filterStatuses = []string{"INTERRUPTED"}
	case "stopped":
		m.filterStatuses = []string{"STOPPED"}
	default:
		m.filterStatuses = nil
	}
}

func (m *model) preserveSelection() {
	if len(m.runs) == 0 {
		m.selected = 0
		m.selectedRunID = ""
		return
	}
	if m.selectedRunID == "" {
		if m.selected < 0 {
			m.selected = 0
		}
		if m.selected >= len(m.runs) {
			m.selected = len(m.runs) - 1
		}
		m.selectedRunID = m.runs[m.selected].RunID
		return
	}
	for idx, r := range m.runs {
		if r.RunID == m.selectedRunID {
			m.selected = idx
			return
		}
	}
	m.selected = 0
	m.selectedRunID = m.runs[0].RunID
}

func (m *model) clearFilters() {
	m.chipIndex = 0
	m.filterStatuses = nil
	m.since24h = false
	m.projectQuery = ""
	m.jobQuery = ""
}

func moreRecent(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return false
	}
	if b == "" {
		return true
	}
	ta, errA := time.Parse(time.RFC3339, a)
	tb, errB := time.Parse(time.RFC3339, b)
	if errA == nil && errB == nil {
		return ta.After(tb)
	}
	return a > b
}
