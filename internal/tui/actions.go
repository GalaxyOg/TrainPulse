package tui

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/trainpulse/trainpulse/internal/config"
	"github.com/trainpulse/trainpulse/internal/doctor"
	statussvc "github.com/trainpulse/trainpulse/internal/status"
	"github.com/trainpulse/trainpulse/internal/store"
)

var cleanupOptions = []string{
	"Clear Filters",
	"Clear Notifier Error Log",
	"Reconcile Orphaned RUNNING",
}

func stopCmd(runID string, fn StopFunc) tea.Cmd {
	return func() tea.Msg {
		if fn == nil {
			return actionMsg{kind: "stop", err: fmt.Errorf("stop action is unavailable")}
		}
		msg, err := fn(runID)
		return actionMsg{kind: "stop", message: msg, err: err}
	}
}

func loadLogCmd(runID, path string, tail int) tea.Cmd {
	return func() tea.Msg {
		lines, err := readTailLines(path, tail)
		if err != nil {
			return logMsg{runID: runID, path: path, tail: tail, err: err}
		}
		summary := extractErrorSummary(lines)
		if summary == "" {
			summary = "-"
		}
		return logMsg{
			runID:   runID,
			path:    path,
			tail:    tail,
			lines:   lines,
			summary: summary,
		}
	}
}

func clearErrorLogCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(path) == "" {
			return actionMsg{kind: "cleanup", err: fmt.Errorf("error log path is empty")}
		}
		full := config.ExpandPath(path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return actionMsg{kind: "cleanup", err: err}
		}
		if err := os.WriteFile(full, []byte(""), 0o644); err != nil {
			return actionMsg{kind: "cleanup", err: err}
		}
		return actionMsg{kind: "cleanup", message: "notifier error log cleared: " + full}
	}
}

func reconcileCmd(st *store.Store) tea.Cmd {
	return func() tea.Msg {
		if st == nil {
			return actionMsg{kind: "cleanup", err: fmt.Errorf("store is nil")}
		}
		running, err := st.ListRuns(nil, true)
		if err != nil {
			return actionMsg{kind: "cleanup", err: err}
		}
		reconciled := 0
		for _, run := range running {
			if run.Status != "RUNNING" {
				continue
			}
			if !statussvc.IsOrphanedRunningRun(run) {
				continue
			}
			updated, err := statussvc.FinalizeRunStopped(st, run, nil)
			if err != nil {
				return actionMsg{kind: "cleanup", err: err}
			}
			if updated {
				reconciled++
			}
		}
		return actionMsg{kind: "cleanup", message: fmt.Sprintf("reconciled orphaned runs: %d", reconciled)}
	}
}

func doctorCmd(fn DoctorFunc, cfgPath, storePath string) tea.Cmd {
	return func() tea.Msg {
		if fn != nil {
			msg, err := fn()
			return actionMsg{kind: "doctor", message: msg, err: err}
		}
		rt, err := config.ResolveRuntime(config.RuntimeInput{ConfigPath: cfgPath, StorePath: &storePath})
		if err != nil {
			return actionMsg{kind: "doctor", err: err}
		}
		report := doctor.Run(rt)
		lines := make([]string, 0, len(report.Items)+1)
		for _, item := range report.Items {
			flag := "OK"
			if !item.OK {
				flag = "FAIL"
			}
			lines = append(lines, fmt.Sprintf("[%s] %s: %s", flag, item.Name, item.Message))
		}
		if report.AllOK() {
			lines = append(lines, "doctor summary: all checks passed")
		}
		return actionMsg{kind: "doctor", message: strings.Join(lines, "\n")}
	}
}

func parseSearchInput(raw string) (project string, job string) {
	for _, part := range strings.Fields(strings.TrimSpace(raw)) {
		switch {
		case strings.HasPrefix(part, "p:"):
			project = strings.TrimSpace(strings.TrimPrefix(part, "p:"))
		case strings.HasPrefix(part, "project:"):
			project = strings.TrimSpace(strings.TrimPrefix(part, "project:"))
		case strings.HasPrefix(part, "j:"):
			job = strings.TrimSpace(strings.TrimPrefix(part, "j:"))
		case strings.HasPrefix(part, "job:"):
			job = strings.TrimSpace(strings.TrimPrefix(part, "job:"))
		default:
			if project == "" {
				project = part
			} else if job == "" {
				job = part
			}
		}
	}
	return project, job
}

func readTailLines(path string, n int) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("empty log path")
	}
	if n <= 0 {
		n = 80
	}
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	scanner := bufio.NewScanner(fp)
	lines := make([]string, 0, n)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return []string{"(log is empty)"}, nil
	}
	return lines, nil
}

func extractErrorSummary(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	patterns := []string{"error", "exception", "traceback", "fatal", "panic", "failed"}
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		for _, p := range patterns {
			if strings.Contains(lower, p) {
				return trimRight(line, 140)
			}
		}
	}
	return trimRight(strings.TrimSpace(lines[len(lines)-1]), 140)
}

func (m *model) openInfo(title, body string) {
	m.modal = modalInfo
	m.modalTitle = title
	m.modalBody = body
}

func (m *model) openCleanupModal() {
	m.modal = modalCleanup
	if m.cleanupIndex < 0 {
		m.cleanupIndex = 0
	}
	if m.cleanupIndex >= len(cleanupOptions) {
		m.cleanupIndex = 0
	}
}

func (m *model) openSetupModal() {
	cfgPath := config.ExpandPath(m.opts.ConfigPath)
	fileCfg, _ := config.LoadFile(cfgPath)
	messageType := fileCfg.MessageType
	if messageType == "" {
		messageType = "post"
	}
	storePath := fileCfg.StorePath
	if storePath == "" {
		storePath = config.DefaultStorePath
	}
	errorLogPath := fileCfg.ErrorLogPath
	if errorLogPath == "" {
		errorLogPath = config.DefaultErrorLogPath
	}
	heartbeat := fileCfg.HeartbeatMinutes
	if heartbeat <= 0 {
		heartbeat = config.DefaultHeartbeatMinute
	}
	dryRun := "false"
	if fileCfg.DryRun != nil && *fileCfg.DryRun {
		dryRun = "true"
	}
	m.setup = setupState{
		index: 0,
		fields: []setupField{
			{key: "webhook_url", label: "Webhook URL", hint: "can be empty", value: fileCfg.WebhookURL},
			{key: "message_type", label: "Message Type", hint: "text or post", value: messageType},
			{key: "store_path", label: "Store Path", hint: "sqlite db path", value: storePath},
			{key: "error_log_path", label: "Error Log Path", hint: "notifier error log", value: errorLogPath},
			{key: "heartbeat_minutes", label: "Heartbeat Minutes", hint: "positive integer", value: strconv.Itoa(heartbeat)},
			{key: "dry_run", label: "Dry Run", hint: "true or false", value: dryRun},
		},
	}
	m.modal = modalSetup
}

func (m *model) saveSetupCmd() tea.Cmd {
	cfgPath := config.ExpandPath(m.opts.ConfigPath)
	fields := append([]setupField{}, m.setup.fields...)
	return func() tea.Msg {
		if err := validateSetup(fields); err != nil {
			return actionMsg{kind: "setup", err: err}
		}
		if err := writeSetupConfig(cfgPath, fields); err != nil {
			return actionMsg{kind: "setup", err: err}
		}
		return actionMsg{kind: "setup", message: "config saved: " + cfgPath}
	}
}

func validateSetup(fields []setupField) error {
	kv := map[string]string{}
	for _, f := range fields {
		kv[f.key] = strings.TrimSpace(f.value)
	}
	messageType := strings.ToLower(kv["message_type"])
	if messageType != "text" && messageType != "post" {
		return fmt.Errorf("message_type must be text or post")
	}
	heartbeat, err := strconv.Atoi(kv["heartbeat_minutes"])
	if err != nil || heartbeat <= 0 {
		return fmt.Errorf("heartbeat_minutes must be a positive integer")
	}
	dry := strings.ToLower(kv["dry_run"])
	if dry != "true" && dry != "false" {
		return fmt.Errorf("dry_run must be true or false")
	}
	if kv["store_path"] == "" {
		return fmt.Errorf("store_path cannot be empty")
	}
	if kv["error_log_path"] == "" {
		return fmt.Errorf("error_log_path cannot be empty")
	}
	return nil
}

func writeSetupConfig(cfgPath string, fields []setupField) error {
	kv := map[string]string{}
	for _, f := range fields {
		kv[f.key] = strings.TrimSpace(f.value)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}
	content := strings.Join([]string{
		"[trainpulse]",
		fmt.Sprintf("webhook_url = %q", kv["webhook_url"]),
		fmt.Sprintf("message_type = %q", strings.ToLower(kv["message_type"])),
		fmt.Sprintf("store_path = %q", kv["store_path"]),
		fmt.Sprintf("error_log_path = %q", kv["error_log_path"]),
		fmt.Sprintf("heartbeat_minutes = %s", kv["heartbeat_minutes"]),
		fmt.Sprintf("dry_run = %s", strings.ToLower(kv["dry_run"])),
		`redact = ["(?i)(token=)\\S+"]`,
		"",
	}, "\n")
	return os.WriteFile(cfgPath, []byte(content), 0o644)
}
