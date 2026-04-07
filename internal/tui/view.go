package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	w := m.width
	if w <= 0 {
		w = 140
	}
	h := m.height
	if h <= 0 {
		h = 36
	}

	header := m.renderHeader(w)
	statusLine := m.renderStatusLine(w)
	help := m.renderHelpBar(w)

	contentHeight := h - 5
	if contentHeight < 12 {
		contentHeight = 12
	}
	leftW := int(float64(w) * 0.54)
	if leftW < 60 {
		leftW = 60
	}
	rightW := w - leftW - 1
	if rightW < 40 {
		rightW = 40
	}

	listPane := m.renderListPane(leftW, contentHeight)
	detailPane := m.renderDetailPane(rightW, contentHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)

	ui := lipgloss.JoinVertical(lipgloss.Left, header, body, statusLine, help)
	if m.modal != modalNone {
		ui = lipgloss.JoinVertical(lipgloss.Left, ui, m.renderModal(w))
	}
	return m.styles.root.Render(ui)
}

func (m model) renderHeader(width int) string {
	now := time.Now().In(time.FixedZone("UTC+8", 8*3600)).Format("2006-01-02 15:04:05")
	filterSummary := fmt.Sprintf("status=%s 24h=%v project=%q job=%q", m.currentStatusLabel(), m.since24h, m.projectQuery, m.jobQuery)
	refreshSummary := "auto=off"
	if m.autoRefresh {
		refreshSummary = "auto=" + m.opts.RefreshInterval.String()
	}
	left := fmt.Sprintf("TrainPulse TUI v%s", m.opts.Version)
	meta := fmt.Sprintf("now=%s  store=%s  config=%s", now, shortenPath(m.opts.StorePath, 42), shortenPath(m.opts.ConfigPath, 42))
	counts := fmt.Sprintf("running=%d failed=%d succeeded=%d interrupted=%d stopped=%d",
		m.counts["RUNNING"], m.counts["FAILED"], m.counts["SUCCEEDED"], m.counts["INTERRUPTED"], m.counts["STOPPED"])
	timeLine := fmt.Sprintf("last_failed=%s  last_active=%s", shortTime(dash(m.lastFailed)), shortTime(dash(m.lastActive)))
	line1 := m.styles.header.Render(left) + "  " + m.styles.headerMuted.Render(meta)
	line2 := m.styles.headerMuted.Render(filterSummary + "  " + refreshSummary)
	line3 := m.styles.headerMuted.Render(counts + "  " + timeLine)
	return fitWidth(lipgloss.JoinVertical(lipgloss.Left, line1, line2, line3), width)
}

func (m model) renderListPane(width, height int) string {
	title := "Runs List"
	if m.focus == focusList {
		title = "Runs List [focus]"
	}
	chips := m.renderStatusChips(width - 4)
	lines := []string{
		m.styles.panelTitle.Render(title),
		chips,
		m.styles.panelTitleDim.Render("st project           job            updated             dur   exit   run_id"),
	}

	visible := height - 6
	if visible < 4 {
		visible = 4
	}
	start := 0
	if m.selected >= visible {
		start = m.selected - visible + 1
	}
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(m.runs) {
		end = len(m.runs)
	}

	if len(m.runs) == 0 {
		lines = append(lines, m.styles.panelTitleDim.Render("no runs"))
	} else {
		for i := start; i < end; i++ {
			r := m.runs[i]
			row := fmt.Sprintf("%s %-16s %-14s %-18s %-5s %-6s %s",
				padStatus(m.shortStatus(r.Status), 2),
				trimRight(r.Project, 16),
				trimRight(r.JobName, 14),
				trimRight(shortTime(r.UpdatedAt), 18),
				trimRight(durationShort(r.Duration), 5),
				exitCodeShort(r.ExitCode),
				trimRight(shortRunID(r.RunID), 12),
			)
			if i == m.selected {
				lines = append(lines, m.styles.rowSelected.Render(row))
			} else {
				lines = append(lines, m.styles.row.Render(row))
			}
		}
	}
	body := strings.Join(lines, "\n")
	panel := m.styles.panel.Width(width).Height(height).Render(body)
	if m.focus == focusList {
		panel = m.styles.focusOn.Width(width).Render(panel)
	} else {
		panel = m.styles.focusOff.Width(width).Render(panel)
	}
	return panel
}

func (m model) renderDetailPane(width, height int) string {
	title := "Run Detail"
	if m.focus == focusFilter {
		title = "Run Detail [focus=filter]"
	}
	lines := []string{m.styles.panelTitle.Render(title)}
	r := m.selectedRun()
	if r == nil {
		lines = append(lines, m.styles.panelTitleDim.Render("no run selected"))
	} else {
		lines = append(lines,
			m.kv("run_id", r.RunID),
			m.kv("status/event", fmt.Sprintf("%s / %s", dash(r.Status), dash(r.Event))),
			m.kv("project/job", fmt.Sprintf("%s / %s", dash(r.Project), dash(r.JobName))),
			m.kv("host/cwd", fmt.Sprintf("%s / %s", dash(r.Host), dash(r.CWD))),
			m.kv("git", fmt.Sprintf("%s@%s", dash(r.GitBranch), dash(r.GitCommit))),
			m.kv("start/end", fmt.Sprintf("%s / %s", shortTime(r.StartTime), shortTime(dash(r.EndTime)))),
			m.kv("updated/duration", fmt.Sprintf("%s / %.3fs", shortTime(r.UpdatedAt), r.Duration)),
			m.kv("pid/exit", fmt.Sprintf("%s / %s", intPtr(r.PID), exitCodeShort(r.ExitCode))),
			m.kv("tmux", dash(r.TmuxSession)),
			m.kv("log", dash(r.LogPath)),
			m.kv("heartbeat", dash(r.LastHeartbeat)),
			m.kv("error_summary", dash(m.errorSummary[r.RunID])),
			m.styles.panelTitleDim.Render("command:"),
		)
		lines = append(lines, wrapText(r.Cmd, width-6)...)
	}
	body := strings.Join(lines, "\n")
	return m.styles.panel.Width(width).Height(height).Render(body)
}

func (m model) renderStatusLine(width int) string {
	msg := m.notice
	if msg == "" {
		msg = "ready"
	}
	prefix := "OK"
	if m.noticeIsErr {
		prefix = "ERR"
	}
	content := fmt.Sprintf("[%s] %s", prefix, msg)
	if m.noticeIsErr {
		return fitWidth(m.styles.statusLine.Width(width).Render(m.styles.errText.Render(content)), width)
	}
	return fitWidth(m.styles.statusLine.Width(width).Render(m.styles.okText.Render(content)), width)
}

func (m model) renderHelpBar(width int) string {
	help := "↑↓ move  ←→ panel/filter  Tab focus  Enter apply  r refresh  p auto  t 24h  / search(p:/j:)  s stop  a attach  l logs  c clear  x cleanup  u setup  d doctor  q quit"
	return fitWidth(m.styles.helpBar.Width(width).Render(help), width)
}

func (m model) renderModal(width int) string {
	switch m.modal {
	case modalConfirmStop:
		body := "Stop selected run?\nrun_id=" + m.confirmRunID + "\n\n[y/Enter] confirm  [n/Esc] cancel"
		return m.styles.modal.Width(min(width-4, 90)).Render(m.styles.modalTitle.Render("Confirm Stop") + "\n" + body)
	case modalSearch:
		body := "Search syntax: p:<project> j:<job>\n\n" + m.searchInput + "_\n\n[Enter] apply  [Esc] cancel  [Ctrl+U] clear"
		return m.styles.modal.Width(min(width-4, 90)).Render(m.styles.modalTitle.Render("Search Filter") + "\n" + body)
	case modalSetup:
		lines := []string{m.styles.modalTitle.Render("Setup Config"), m.styles.modalHint.Render("Tab/Up/Down switch field, type to edit, Enter save at last field")}
		for i, f := range m.setup.fields {
			prefix := "  "
			if i == m.setup.index {
				prefix = "> "
			}
			lines = append(lines, fmt.Sprintf("%s%s: %s", prefix, f.label, f.value))
			if i == m.setup.index {
				lines = append(lines, m.styles.modalHint.Render("    hint: "+f.hint))
			}
		}
		lines = append(lines, "", "[Enter] next/save  [Esc] cancel  [Ctrl+U] clear field")
		return m.styles.modal.Width(min(width-4, 110)).Render(strings.Join(lines, "\n"))
	case modalInfo:
		body := m.modalBody
		if strings.TrimSpace(body) == "" {
			body = "(empty)"
		}
		return m.styles.modal.Width(min(width-4, 110)).Render(m.styles.modalTitle.Render(m.modalTitle) + "\n" + body + "\n\n[Enter/Esc] close")
	case modalLogs:
		page := m.logPageSize()
		total := len(m.logLines)
		start, end, shown := logWindow(m.logLines, m.logOffset, page)
		showRange := "0-0"
		if total > 0 && end >= start {
			showRange = fmt.Sprintf("%d-%d", start+1, end)
		}
		header := fmt.Sprintf("path=%s  tail=%d  follow=%v  lines=%d  show=%s",
			m.logPath, m.logTailLines, m.logFollow, total, showRange)
		content := formatLogLines(shown, min(width-10, 150))
		body := strings.Join([]string{
			m.styles.modalTitle.Render("Logs"),
			m.styles.modalHint.Render(header),
			"",
			content,
			"",
			"[r] reload  [f] follow on/off  [+/-] tail lines  [PgUp/PgDn/Home/End/j/k] scroll  [Esc] close",
		}, "\n")
		return m.styles.modal.Width(min(width-4, 160)).Render(body)
	case modalCleanup:
		lines := []string{
			m.styles.modalTitle.Render("Cleanup Actions"),
			m.styles.modalHint.Render("Use ↑↓ and Enter"),
			"",
		}
		for i, opt := range cleanupOptions {
			prefix := "  "
			if i == m.cleanupIndex {
				prefix = "> "
			}
			lines = append(lines, prefix+opt)
		}
		lines = append(lines, "", "[Enter] execute  [Esc] cancel")
		return m.styles.modal.Width(min(width-4, 80)).Render(strings.Join(lines, "\n"))
	default:
		return ""
	}
}

func (m model) renderStatusChips(width int) string {
	items := make([]string, 0, len(m.statusChips)+2)
	for i, chip := range m.statusChips {
		label := " " + chip + " "
		style := m.styles.panelTitleDim
		if i == m.chipIndex {
			style = m.styles.panelTitle
		}
		if containsStatus(m.filterStatuses, chipToStatus(chip)) || (chip == "all" && len(m.filterStatuses) == 0) {
			style = style.Bold(true)
		}
		items = append(items, style.Render(label))
	}
	if m.since24h {
		items = append(items, m.styles.statusInterrupt.Render(" 24h "))
	}
	if m.projectQuery != "" {
		items = append(items, m.styles.okText.Render(" p:"+m.projectQuery+" "))
	}
	if m.jobQuery != "" {
		items = append(items, m.styles.okText.Render(" j:"+m.jobQuery+" "))
	}
	return fitWidth(strings.Join(items, ""), width)
}

func (m model) kv(k string, v string) string {
	return m.styles.label.Render(trimRight(k, 14)+": ") + m.styles.value.Render(v)
}

func (m model) currentStatusLabel() string {
	if len(m.filterStatuses) == 0 {
		return "all"
	}
	return strings.ToLower(strings.Join(m.filterStatuses, ","))
}

func shortRunID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func chipToStatus(chip string) string {
	switch chip {
	case "running":
		return "RUNNING"
	case "failed":
		return "FAILED"
	case "succeeded":
		return "SUCCEEDED"
	case "interrupted":
		return "INTERRUPTED"
	case "stopped":
		return "STOPPED"
	default:
		return ""
	}
}

func containsStatus(list []string, target string) bool {
	if target == "" {
		return false
	}
	for _, x := range list {
		if strings.EqualFold(strings.TrimSpace(x), target) {
			return true
		}
	}
	return false
}

func (m model) shortStatus(status string) string {
	s := strings.ToUpper(strings.TrimSpace(status))
	switch s {
	case "RUNNING":
		return m.styles.statusRunning.Render("RN")
	case "FAILED":
		return m.styles.statusFailed.Render("FL")
	case "SUCCEEDED":
		return m.styles.statusSuccess.Render("OK")
	case "INTERRUPTED":
		return m.styles.statusInterrupt.Render("IN")
	case "STOPPED":
		return m.styles.statusStopped.Render("SP")
	default:
		return m.styles.statusUnknown.Render("--")
	}
}

func shortTime(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return "-"
	}
	if len(s) >= 19 {
		return strings.ReplaceAll(s[:19], "T", " ")
	}
	return s
}

func durationShort(v float64) string {
	if v <= 0 {
		return "-"
	}
	if v >= 3600 {
		return fmt.Sprintf("%.1fh", v/3600)
	}
	if v >= 60 {
		return fmt.Sprintf("%.1fm", v/60)
	}
	return fmt.Sprintf("%.0fs", v)
}

func intPtr(v *int) string {
	if v == nil {
		return "-"
	}
	return strconv.Itoa(*v)
}

func exitCodeShort(v *int) string {
	if v == nil {
		return "-"
	}
	return strconv.Itoa(*v)
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func trimRight(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

func fitWidth(s string, width int) string {
	if width <= 0 {
		return s
	}
	return lipgloss.NewStyle().Width(width).Render(s)
}

func wrapText(s string, width int) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{"-"}
	}
	if width <= 10 {
		return []string{trimRight(s, width)}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{s}
	}
	lines := []string{}
	current := ""
	for _, w := range words {
		if current == "" {
			current = w
			continue
		}
		if len(current)+1+len(w) <= width {
			current += " " + w
			continue
		}
		lines = append(lines, current)
		current = w
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func formatLogLines(shown []string, width int) string {
	if len(shown) == 0 {
		return "(log is empty)"
	}
	out := make([]string, 0, len(shown))
	for _, ln := range shown {
		out = append(out, trimRight(ln, width))
	}
	return strings.Join(out, "\n")
}

func logWindow(lines []string, offset, page int) (start int, end int, shown []string) {
	if page <= 0 {
		page = 1
	}
	total := len(lines)
	if total <= 0 {
		return 0, 0, nil
	}
	maxOffset := maxInt(0, total-page)
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	start = offset
	end = offset + page
	if end > total {
		end = total
	}
	return start, end, lines[start:end]
}

func padStatus(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(s)
}

func shortenPath(p string, max int) string {
	if p == "" {
		return "-"
	}
	if len(p) <= max {
		return p
	}
	if max <= 3 {
		return p[:max]
	}
	return "..." + p[len(p)-max+3:]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
