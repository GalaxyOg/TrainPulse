package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/trainpulse/trainpulse/internal/store"
)

type layoutMode int

const (
	layoutModeTiny layoutMode = iota
	layoutModeSingle
	layoutModeDual
)

type viewLayout struct {
	totalW  int
	totalH  int
	headerH int
	statusH int
	helpH   int
	bodyH   int
	mode    layoutMode
	leftW   int
	rightW  int
	listH   int
	detailH int
}

func (m model) View() string {
	layout := calcViewLayout(m.width, m.height)
	if layout.mode == layoutModeTiny {
		return fitCanvas(m.styles.root.Render(m.renderTinyView(layout)), layout.totalW, layout.totalH)
	}

	header := m.renderHeader(layout.totalW)
	statusLine := m.renderStatusLine(layout.totalW)
	help := m.renderHelpBar(layout.totalW)

	body := m.renderMainBody(layout)
	if m.modal != modalNone {
		body = m.renderModalBody(layout)
	}

	ui := lipgloss.JoinVertical(lipgloss.Left, header, body, statusLine, help)
	return fitCanvas(m.styles.root.Render(ui), layout.totalW, layout.totalH)
}

func calcViewLayout(width, height int) viewLayout {
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 30
	}
	layout := viewLayout{
		totalW:  width,
		totalH:  height,
		headerH: 3,
		statusH: 1,
		helpH:   1,
	}
	layout.bodyH = layout.totalH - layout.headerH - layout.statusH - layout.helpH
	if layout.totalW < 52 || layout.totalH < 8 || layout.bodyH < 3 {
		layout.mode = layoutModeTiny
		return layout
	}
	if layout.totalW >= 110 && layout.bodyH >= 10 {
		gap := 1
		minLeft := 38
		minRight := 34
		maxLeft := layout.totalW - gap - minRight
		if maxLeft >= minLeft {
			left := int(float64(layout.totalW) * 0.54)
			if left < minLeft {
				left = minLeft
			}
			if left > maxLeft {
				left = maxLeft
			}
			layout.mode = layoutModeDual
			layout.leftW = left
			layout.rightW = layout.totalW - gap - left
			return layout
		}
	}

	layout.mode = layoutModeSingle
	layout.leftW = layout.totalW
	if layout.bodyH >= 11 {
		layout.listH = (layout.bodyH * 3) / 5
		if layout.listH < 6 {
			layout.listH = 6
		}
		layout.detailH = layout.bodyH - layout.listH - 1
		if layout.detailH < 4 {
			layout.detailH = 0
			layout.listH = layout.bodyH
		}
	} else {
		layout.listH = layout.bodyH
		layout.detailH = 0
	}
	return layout
}

func (m model) renderTinyView(layout viewLayout) string {
	lines := []string{
		m.styles.header.Render(trimRight(fmt.Sprintf("TrainPulse TUI v%s", m.opts.Version), layout.totalW)),
		m.styles.headerMuted.Render(trimRight(fmt.Sprintf("terminal too small: %dx%d (need >= 52x8)", layout.totalW, layout.totalH), layout.totalW)),
		m.styles.headerMuted.Render(trimRight("resize terminal for full dashboard", layout.totalW)),
		trimRight(fmt.Sprintf("runs=%d  selected=%s", len(m.runs), dash(shortRunID(m.selectedRunID))), layout.totalW),
		trimRight("[q] quit  [r] refresh  [/] search  [u] setup", layout.totalW),
	}
	return strings.Join(lines, "\n")
}

func (m model) renderMainBody(layout viewLayout) string {
	switch layout.mode {
	case layoutModeDual:
		listPane := m.renderListPane(layout.leftW, layout.bodyH)
		detailPane := m.renderDetailPane(layout.rightW, layout.bodyH)
		gap := lipgloss.NewStyle().Width(1).Render("")
		return fitCanvas(lipgloss.JoinHorizontal(lipgloss.Top, listPane, gap, detailPane), layout.totalW, layout.bodyH)
	case layoutModeSingle:
		listPane := m.renderListPane(layout.leftW, layout.listH)
		if layout.detailH <= 0 {
			return fitCanvas(listPane, layout.totalW, layout.bodyH)
		}
		detailPane := m.renderDetailPane(layout.leftW, layout.detailH)
		gap := lipgloss.NewStyle().Width(layout.totalW).Render("")
		return fitCanvas(lipgloss.JoinVertical(lipgloss.Left, listPane, gap, detailPane), layout.totalW, layout.bodyH)
	default:
		return fitCanvas("", layout.totalW, layout.bodyH)
	}
}

func (m model) renderModalBody(layout viewLayout) string {
	modal := m.renderModal(layout.totalW)
	modal = clipLines(modal, layout.bodyH)
	return lipgloss.Place(layout.totalW, layout.bodyH, lipgloss.Center, lipgloss.Center, modal)
}

func (m model) renderHeader(width int) string {
	now := time.Now().In(time.FixedZone("UTC+8", 8*3600)).Format("2006-01-02 15:04:05")
	filterSummary := fmt.Sprintf("status=%s 24h=%v project=%q job=%q", m.currentStatusLabel(), m.since24h, m.projectQuery, m.jobQuery)
	refreshSummary := "auto=off"
	if m.autoRefresh {
		refreshSummary = "auto=" + m.opts.RefreshInterval.String()
	}
	line1 := fmt.Sprintf("TrainPulse TUI v%s  now=%s  store=%s  config=%s",
		m.opts.Version, now, shortenPath(m.opts.StorePath, 40), shortenPath(m.opts.ConfigPath, 40))
	counts := fmt.Sprintf("running=%d failed=%d succeeded=%d interrupted=%d stopped=%d",
		m.counts["RUNNING"], m.counts["FAILED"], m.counts["SUCCEEDED"], m.counts["INTERRUPTED"], m.counts["STOPPED"])
	line2 := fmt.Sprintf("%s  %s", filterSummary, refreshSummary)
	line3 := fmt.Sprintf("%s  last_failed=%s  last_active=%s", counts, shortTime(dash(m.lastFailed)), shortTime(dash(m.lastActive)))
	lines := []string{
		m.styles.header.Render(trimRight(line1, width)),
		m.styles.headerMuted.Render(trimRight(line2, width)),
		m.styles.headerMuted.Render(trimRight(line3, width)),
	}
	return fitCanvas(strings.Join(lines, "\n"), width, 3)
}

func (m model) renderListPane(width, height int) string {
	if width <= 4 || height <= 2 {
		return fitCanvas("list", width, height)
	}
	styleW, styleH, contentW, contentH := resolveBox(m.styles.panel, width, height)

	title := "Runs List"
	if m.focus == focusList {
		title = "Runs List [focus]"
	}
	chips := m.renderStatusChips(contentW)
	headerLine := listHeaderLine(contentW)
	lines := []string{
		m.styles.panelTitle.Render(trimRight(title, contentW)),
		chips,
		m.styles.panelTitleDim.Render(trimRight(headerLine, contentW)),
	}

	visible := contentH - 4
	if visible < 1 {
		visible = 1
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
		lines = append(lines, m.styles.panelTitleDim.Render(trimRight("no runs", contentW)))
	} else {
		for i := start; i < end; i++ {
			r := m.runs[i]
			row := listRowLine(r, contentW)
			row = trimRight(row, contentW)
			if i == m.selected {
				lines = append(lines, m.styles.rowSelected.Render(row))
			} else {
				lines = append(lines, m.styles.row.Render(row))
			}
		}
	}
	body := strings.Join(lines, "\n")
	panelStyle := m.styles.panel.Width(styleW).Height(styleH)
	if m.focus == focusList {
		panelStyle = panelStyle.BorderForeground(lipgloss.Color("45"))
	} else {
		panelStyle = panelStyle.BorderForeground(lipgloss.Color("238"))
	}
	return fitCanvas(panelStyle.Render(body), width, height)
}

func (m model) renderDetailPane(width, height int) string {
	if width <= 4 || height <= 2 {
		return fitCanvas("detail", width, height)
	}
	styleW, styleH, contentW, _ := resolveBox(m.styles.panel, width, height)

	title := "Run Detail"
	if m.focus == focusFilter {
		title = "Run Detail [focus=filter]"
	}
	lines := []string{m.styles.panelTitle.Render(trimRight(title, contentW))}
	r := m.selectedRun()
	if r == nil {
		lines = append(lines, m.styles.panelTitleDim.Render(trimRight("no run selected", contentW)))
	} else {
		lines = append(lines,
			trimRight(m.kv("run_id", r.RunID), contentW),
			trimRight(m.kv("status/event", fmt.Sprintf("%s / %s", dash(r.Status), dash(r.Event))), contentW),
			trimRight(m.kv("project/job", fmt.Sprintf("%s / %s", dash(r.Project), dash(r.JobName))), contentW),
			trimRight(m.kv("host/cwd", fmt.Sprintf("%s / %s", dash(r.Host), dash(r.CWD))), contentW),
			trimRight(m.kv("git", fmt.Sprintf("%s@%s", dash(r.GitBranch), dash(r.GitCommit))), contentW),
			trimRight(m.kv("start/end", fmt.Sprintf("%s / %s", shortTime(r.StartTime), shortTime(dash(r.EndTime)))), contentW),
			trimRight(m.kv("updated/duration", fmt.Sprintf("%s / %.3fs", shortTime(r.UpdatedAt), r.Duration)), contentW),
			trimRight(m.kv("pid/exit", fmt.Sprintf("%s / %s", intPtr(r.PID), exitCodeShort(r.ExitCode))), contentW),
			trimRight(m.kv("tmux", dash(r.TmuxSession)), contentW),
			trimRight(m.kv("log", dash(r.LogPath)), contentW),
			trimRight(m.kv("heartbeat", dash(r.LastHeartbeat)), contentW),
			trimRight(m.kv("error_summary", dash(m.errorSummary[r.RunID])), contentW),
			m.styles.panelTitleDim.Render(trimRight("command:", contentW)),
		)
		for _, ln := range wrapText(r.Cmd, contentW) {
			lines = append(lines, trimRight(ln, contentW))
		}
	}
	body := strings.Join(lines, "\n")
	panelStyle := m.styles.panel.Width(styleW).Height(styleH)
	if m.focus == focusFilter {
		panelStyle = panelStyle.BorderForeground(lipgloss.Color("45"))
	}
	return fitCanvas(panelStyle.Render(body), width, height)
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
	content := trimRight(fmt.Sprintf("[%s] %s", prefix, msg), width)
	if m.noticeIsErr {
		return renderBar(m.styles.statusLine, m.styles.errText, width, content)
	}
	return renderBar(m.styles.statusLine, m.styles.okText, width, content)
}

func (m model) renderHelpBar(width int) string {
	help := "↑↓ move  ←→ panel/filter  Tab focus  Enter apply  r refresh  p auto  t 24h  / search(p:/j:)  s stop  a attach  l logs  c clear  x cleanup  u setup  d doctor  q quit"
	return renderBar(m.styles.helpBar, m.styles.panelTitleDim, width, trimRight(help, width))
}

func (m model) renderModal(width int) string {
	switch m.modal {
	case modalConfirmStop:
		body := "Stop selected run?\nrun_id=" + m.confirmRunID + "\n\n[y/Enter] confirm  [n/Esc] cancel"
		return m.renderModalFrame(width, 90, m.styles.modalTitle.Render("Confirm Stop")+"\n"+body)
	case modalSearch:
		body := "Search syntax: p:<project> j:<job>\n\n" + m.searchInput + "_\n\n[Enter] apply  [Esc] cancel  [Ctrl+U] clear"
		return m.renderModalFrame(width, 90, m.styles.modalTitle.Render("Search Filter")+"\n"+body)
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
		return m.renderModalFrame(width, 110, strings.Join(lines, "\n"))
	case modalInfo:
		body := m.modalBody
		if strings.TrimSpace(body) == "" {
			body = "(empty)"
		}
		return m.renderModalFrame(width, 110, m.styles.modalTitle.Render(m.modalTitle)+"\n"+body+"\n\n[Enter/Esc] close")
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
		return m.renderModalFrame(width, 160, body)
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
		return m.renderModalFrame(width, 80, strings.Join(lines, "\n"))
	default:
		return ""
	}
}

func (m model) renderModalFrame(totalWidth, maxOuter int, content string) string {
	outer := min(totalWidth-2, maxOuter)
	if outer < 20 {
		outer = totalWidth
	}
	styleW, _, contentW, _ := resolveBox(m.styles.modal, outer, 1)
	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = trimRight(lines[i], contentW)
	}
	return m.styles.modal.Width(styleW).Render(strings.Join(lines, "\n"))
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
	return trimRight(k, 14) + ": " + v
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

func shortStatusCode(status string) string {
	s := strings.ToUpper(strings.TrimSpace(status))
	switch s {
	case "RUNNING":
		return "RN"
	case "FAILED":
		return "FL"
	case "SUCCEEDED":
		return "OK"
	case "INTERRUPTED":
		return "IN"
	case "STOPPED":
		return "SP"
	default:
		return "--"
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
	if lipgloss.Width(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	target := max - 1
	out := strings.Builder{}
	for _, r := range s {
		next := out.String() + string(r)
		if lipgloss.Width(next) > target {
			break
		}
		out.WriteRune(r)
	}
	return out.String() + "…"
}

func fitWidth(s string, width int) string {
	if width <= 0 {
		return s
	}
	return lipgloss.NewStyle().Width(width).Render(s)
}

func fitCanvas(s string, width, height int) string {
	if width <= 0 || height <= 0 {
		return s
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(s)
}

func clipLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
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
		if lipgloss.Width(current+" "+w) <= width {
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

func renderBar(style lipgloss.Style, textStyle lipgloss.Style, outerW int, content string) string {
	if outerW <= 0 {
		return content
	}
	styleW, _, contentW, _ := resolveBox(style, outerW, 1)
	trimmed := trimRight(content, contentW)
	return fitCanvas(style.Width(styleW).Render(textStyle.Render(trimmed)), outerW, 1)
}

func listHeaderLine(innerW int) string {
	switch {
	case innerW >= 74:
		return "st project           job            updated             dur   exit   run_id"
	case innerW >= 56:
		return "st project        job          updated           run_id"
	default:
		return "st project      run_id"
	}
}

func listRowLine(r store.Run, innerW int) string {
	switch {
	case innerW >= 74:
		return fmt.Sprintf("%-2s %-16s %-14s %-18s %-5s %-6s %-12s",
			shortStatusCode(r.Status),
			trimRight(r.Project, 16),
			trimRight(r.JobName, 14),
			trimRight(shortTime(r.UpdatedAt), 18),
			trimRight(durationShort(r.Duration), 5),
			trimRight(exitCodeShort(r.ExitCode), 6),
			trimRight(shortRunID(r.RunID), 12),
		)
	case innerW >= 56:
		return fmt.Sprintf("%-2s %-14s %-12s %-16s %-10s",
			shortStatusCode(r.Status),
			trimRight(r.Project, 14),
			trimRight(r.JobName, 12),
			trimRight(shortTime(r.UpdatedAt), 16),
			trimRight(shortRunID(r.RunID), 10),
		)
	default:
		return fmt.Sprintf("%-2s %-12s %-10s",
			shortStatusCode(r.Status),
			trimRight(r.Project, 12),
			trimRight(shortRunID(r.RunID), 10),
		)
	}
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

func resolveBox(style lipgloss.Style, outerW, outerH int) (styleW, styleH, contentW, contentH int) {
	borderW := style.GetHorizontalFrameSize() - style.GetHorizontalPadding()
	borderH := style.GetVerticalFrameSize() - style.GetVerticalPadding()
	if borderW < 0 {
		borderW = 0
	}
	if borderH < 0 {
		borderH = 0
	}
	styleW = maxInt(1, outerW-borderW)
	styleH = maxInt(1, outerH-borderH)
	contentW = maxInt(1, styleW-style.GetHorizontalPadding())
	contentH = maxInt(1, styleH-style.GetVerticalPadding())
	return styleW, styleH, contentW, contentH
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
