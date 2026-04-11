package tui

import (
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		cmds := []tea.Cmd{tickEverySecond()}
		if m.modal == modalNone && m.autoRefresh {
			if m.lastRefresh.IsZero() || time.Since(m.lastRefresh) >= m.opts.RefreshInterval {
				cmds = append(cmds, m.refreshCmd())
			}
		}
		if m.modal == modalLogs && m.logFollow && strings.TrimSpace(m.logPath) != "" {
			cmds = append(cmds, loadLogCmd(m.logRunID, m.logPath, m.logTailLines))
		}
		return m, tea.Batch(cmds...)
	case refreshMsg:
		if msg.err != nil {
			m.setNotice("refresh failed: "+msg.err.Error(), true)
			return m, nil
		}
		m.runs = msg.runs
		m.counts = msg.counts
		m.lastFailed = msg.lastFailed
		m.lastActive = msg.lastActive
		m.lastRefresh = time.Now()
		m.preserveSelection()
		if len(m.runs) == 0 {
			m.setNotice("no runs matched current filters", false)
		}
		return m, nil
	case logMsg:
		if msg.err != nil {
			m.setNotice("log load failed: "+msg.err.Error(), true)
			return m, nil
		}
		prevRunID := m.logRunID
		m.logRunID = msg.runID
		m.logPath = msg.path
		m.logTailLines = msg.tail
		m.logLines = append([]string{}, msg.lines...)
		m.errorSummary[msg.runID] = msg.summary
		page := m.logPageSize()
		maxOffset := maxInt(0, len(m.logLines)-page)
		if m.logFollow || prevRunID != msg.runID {
			m.logOffset = maxOffset
		} else {
			if m.logOffset > maxOffset {
				m.logOffset = maxOffset
			}
			if m.logOffset < 0 {
				m.logOffset = 0
			}
		}
		if strings.TrimSpace(msg.summary) != "" && msg.summary != "-" {
			m.setNotice("log summary: "+msg.summary, false)
		}
		return m, nil
	case actionMsg:
		if msg.err != nil {
			m.setNotice(msg.kind+" failed: "+msg.err.Error(), true)
			return m, nil
		}
		if strings.TrimSpace(msg.message) != "" {
			m.setNotice(msg.message, false)
		}
		switch msg.kind {
		case "stop":
			return m, m.refreshCmd()
		case "setup":
			m.modal = modalNone
			return m, m.refreshCmd()
		case "doctor":
			m.openInfo("Doctor Result", msg.message)
		case "cleanup":
			return m, m.refreshCmd()
		}
		return m, nil
	case tea.KeyMsg:
		if m.modal != modalNone {
			return m.handleModalKeys(msg)
		}
		return m.handleMainKeys(msg)
	}
	return m, nil
}

func (m model) handleMainKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab":
		if m.focus == focusList {
			m.focus = focusFilter
		} else {
			m.focus = focusList
		}
		return m, nil
	case "left":
		if m.focus == focusFilter {
			m.focus = focusList
		}
		return m, nil
	case "right":
		if m.focus == focusList {
			m.focus = focusFilter
		}
		return m, nil
	case "[":
		if len(m.statusChips) == 0 {
			return m, nil
		}
		if m.chipIndex > 0 {
			m.chipIndex--
		} else {
			m.chipIndex = len(m.statusChips) - 1
		}
		m.applyChipFilter()
		m.setNotice("status filter: "+m.statusChips[m.chipIndex], false)
		return m, m.refreshCmd()
	case "]":
		if len(m.statusChips) == 0 {
			return m, nil
		}
		if m.chipIndex < len(m.statusChips)-1 {
			m.chipIndex++
		} else {
			m.chipIndex = 0
		}
		m.applyChipFilter()
		m.setNotice("status filter: "+m.statusChips[m.chipIndex], false)
		return m, m.refreshCmd()
	case "up", "k":
		if m.focus == focusList && m.selected > 0 {
			m.selected--
			m.selectedRunID = m.runs[m.selected].RunID
		}
		return m, nil
	case "down", "j":
		if m.focus == focusList && m.selected < len(m.runs)-1 {
			m.selected++
			m.selectedRunID = m.runs[m.selected].RunID
		}
		return m, nil
	case "enter":
		if m.focus == focusFilter {
			m.applyChipFilter()
			return m, m.refreshCmd()
		}
		return m, nil
	case "r":
		return m, m.refreshCmd()
	case "p":
		m.autoRefresh = !m.autoRefresh
		if m.autoRefresh {
			m.setNotice("auto refresh enabled", false)
		} else {
			m.setNotice("auto refresh paused", false)
		}
		return m, nil
	case "t":
		m.since24h = !m.since24h
		if m.since24h {
			m.setNotice("filter enabled: last 24h", false)
		} else {
			m.setNotice("filter disabled: last 24h", false)
		}
		return m, m.refreshCmd()
	case "c":
		m.clearFilters()
		m.setNotice("filters cleared", false)
		return m, m.refreshCmd()
	case "/":
		m.modal = modalSearch
		m.searchInput = strings.TrimSpace("p:" + m.projectQuery + " j:" + m.jobQuery)
		return m, nil
	case "s":
		r := m.selectedRun()
		if r == nil {
			m.setNotice("no run selected", true)
			return m, nil
		}
		m.modal = modalConfirmStop
		m.confirmRunID = r.RunID
		return m, nil
	case "a":
		r := m.selectedRun()
		if r == nil {
			m.setNotice("no run selected", true)
			return m, nil
		}
		if strings.TrimSpace(r.TmuxSession) == "" {
			m.openInfo("Attach", "current run has no tmux session")
		} else {
			m.openInfo("Attach", "tmux attach -t "+r.TmuxSession)
		}
		return m, nil
	case "l":
		r := m.selectedRun()
		if r == nil {
			m.setNotice("no run selected", true)
			return m, nil
		}
		if strings.TrimSpace(r.LogPath) == "" {
			m.openInfo("Log", "current run has no log path")
			return m, nil
		}
		m.modal = modalLogs
		m.logRunID = r.RunID
		m.logPath = r.LogPath
		m.logOffset = 0
		if m.logTailLines <= 0 {
			m.logTailLines = 120
		}
		return m, loadLogCmd(m.logRunID, m.logPath, m.logTailLines)
	case "d":
		return m, doctorCmd(m.opts.Doctor, m.opts.ConfigPath, m.opts.StorePath)
	case "u":
		m.openSetupModal()
		return m, nil
	case "x":
		m.openCleanupModal()
		return m, nil
	}
	return m, nil
}

func (m model) handleModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.modal {
	case modalConfirmStop:
		switch msg.String() {
		case "esc", "n":
			m.modal = modalNone
			m.confirmRunID = ""
			return m, nil
		case "y", "enter":
			runID := m.confirmRunID
			m.modal = modalNone
			m.confirmRunID = ""
			return m, stopCmd(runID, m.opts.Stop)
		}
		return m, nil
	case modalSearch:
		switch msg.String() {
		case "esc":
			m.modal = modalNone
			return m, nil
		case "enter":
			project, job := parseSearchInput(m.searchInput)
			m.projectQuery = project
			m.jobQuery = job
			m.modal = modalNone
			m.setNotice("search applied", false)
			return m, m.refreshCmd()
		case "backspace":
			m.searchInput = removeLastRune(m.searchInput)
			return m, nil
		case "ctrl+u":
			m.searchInput = ""
			return m, nil
		default:
			if msg.Type == tea.KeyRunes {
				m.searchInput += msg.String()
			}
			return m, nil
		}
	case modalSetup:
		switch msg.String() {
		case "esc":
			m.modal = modalNone
			return m, nil
		case "tab", "down":
			if m.setup.index < len(m.setup.fields)-1 {
				m.setup.index++
			}
			return m, nil
		case "shift+tab", "up":
			if m.setup.index > 0 {
				m.setup.index--
			}
			return m, nil
		case "enter":
			if m.setup.index < len(m.setup.fields)-1 {
				m.setup.index++
				return m, nil
			}
			return m, m.saveSetupCmd()
		case "backspace":
			m.setup.fields[m.setup.index].value = removeLastRune(m.setup.fields[m.setup.index].value)
			return m, nil
		case "ctrl+u":
			m.setup.fields[m.setup.index].value = ""
			return m, nil
		default:
			if msg.Type == tea.KeyRunes {
				m.setup.fields[m.setup.index].value += msg.String()
			}
			return m, nil
		}
	case modalLogs:
		switch msg.String() {
		case "esc", "q":
			m.modal = modalNone
			return m, nil
		case "r":
			return m, loadLogCmd(m.logRunID, m.logPath, m.logTailLines)
		case "f":
			m.logFollow = !m.logFollow
			if m.logFollow {
				m.setNotice("log follow enabled", false)
			} else {
				m.setNotice("log follow paused", false)
			}
			return m, nil
		case "+":
			m.logTailLines += 40
			if m.logTailLines > 2000 {
				m.logTailLines = 2000
			}
			return m, loadLogCmd(m.logRunID, m.logPath, m.logTailLines)
		case "-":
			m.logTailLines -= 40
			if m.logTailLines < 40 {
				m.logTailLines = 40
			}
			return m, loadLogCmd(m.logRunID, m.logPath, m.logTailLines)
		case "pgdown":
			m.pauseLogFollowOnManualScroll()
			m.logOffset = m.nextLogOffset(1 * m.logPageSize())
			return m, nil
		case "pgup":
			m.pauseLogFollowOnManualScroll()
			m.logOffset = m.nextLogOffset(-1 * m.logPageSize())
			return m, nil
		case "down", "j":
			m.pauseLogFollowOnManualScroll()
			m.logOffset = m.nextLogOffset(1)
			return m, nil
		case "up", "k":
			m.pauseLogFollowOnManualScroll()
			m.logOffset = m.nextLogOffset(-1)
			return m, nil
		case "home":
			m.pauseLogFollowOnManualScroll()
			m.logOffset = 0
			return m, nil
		case "end":
			m.pauseLogFollowOnManualScroll()
			m.logOffset = maxInt(0, len(m.logLines)-m.logPageSize())
			return m, nil
		}
		if m.logFollow {
			return m, loadLogCmd(m.logRunID, m.logPath, m.logTailLines)
		}
		return m, nil
	case modalCleanup:
		switch msg.String() {
		case "esc":
			m.modal = modalNone
			return m, nil
		case "up", "k":
			if m.cleanupIndex > 0 {
				m.cleanupIndex--
			}
			return m, nil
		case "down", "j":
			if m.cleanupIndex < len(cleanupOptions)-1 {
				m.cleanupIndex++
			}
			return m, nil
		case "enter":
			switch m.cleanupIndex {
			case 0:
				m.clearFilters()
				m.modal = modalNone
				m.setNotice("filters cleared", false)
				return m, m.refreshCmd()
			case 1:
				m.modal = modalNone
				return m, clearErrorLogCmd(m.opts.ErrorLogPath)
			case 2:
				m.modal = modalNone
				return m, reconcileCmd(m.opts.Store)
			default:
				m.modal = modalNone
				return m, nil
			}
		}
		return m, nil
	case modalInfo:
		switch msg.String() {
		case "esc", "enter", "q":
			m.modal = modalNone
			m.modalTitle = ""
			m.modalBody = ""
			return m, nil
		}
		return m, nil
	}
	return m, nil
}

func removeLastRune(s string) string {
	if s == "" {
		return s
	}
	_, size := utf8.DecodeLastRuneInString(s)
	if size <= 0 || size > len(s) {
		return ""
	}
	return s[:len(s)-size]
}

func (m model) logPageSize() int {
	if m.height <= 0 {
		return 16
	}
	v := m.height / 3
	if v < 8 {
		v = 8
	}
	if v > 30 {
		v = 30
	}
	return v
}

func (m model) nextLogOffset(delta int) int {
	maxOffset := maxInt(0, len(m.logLines)-m.logPageSize())
	next := m.logOffset + delta
	if next < 0 {
		next = 0
	}
	if next > maxOffset {
		next = maxOffset
	}
	return next
}

func (m *model) pauseLogFollowOnManualScroll() {
	if !m.logFollow {
		return
	}
	m.logFollow = false
	m.setNotice("log follow paused (manual scroll)", false)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
