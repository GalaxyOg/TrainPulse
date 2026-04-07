package context

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type RunContext struct {
	RunID       string
	Project     string
	JobName     string
	Host        string
	CWD         string
	GitBranch   string
	GitCommit   string
	Cmd         string
	LogPath     string
	TmuxSession string
	PID         int
	StartTime   string
}

var utcPlus8 = time.FixedZone("UTC+8", 8*3600)

func nowISO() string {
	return time.Now().In(utcPlus8).Format(time.RFC3339)
}

func runGit(cwd string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

func DetectProjectName(cwd string) string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		abs = cwd
	}
	top := runGit(abs, "rev-parse", "--show-toplevel")
	if top != "" {
		return filepath.Base(top)
	}
	return filepath.Base(abs)
}

func DetectGitBranch(cwd string) string {
	return runGit(cwd, "rev-parse", "--abbrev-ref", "HEAD")
}

func DetectGitCommit(cwd string) string {
	return runGit(cwd, "rev-parse", "--short", "HEAD")
}

func InferJobName(parts []string) string {
	if len(parts) == 0 {
		return "unknown-job"
	}
	first := filepath.Base(parts[0])
	if first == "uv" && len(parts) >= 3 && parts[1] == "run" {
		return InferJobName(parts[2:])
	}
	if first == "python" || first == "python3" || strings.HasPrefix(first, "python3.") || first == "uv" {
		for _, token := range parts[1:] {
			if strings.HasPrefix(token, "-") {
				continue
			}
			return strings.TrimSuffix(filepath.Base(token), filepath.Ext(token))
		}
	}
	if first == "conda" {
		idx := slices.Index(parts, "run")
		if idx >= 0 {
			rest := parts[idx+1:]
			valueOptions := map[string]bool{"-n": true, "--name": true, "-p": true, "--prefix": true}
			i := 0
			for i < len(rest) && strings.HasPrefix(rest[i], "-") {
				if valueOptions[rest[i]] && i+1 < len(rest) {
					i += 2
				} else {
					i++
				}
			}
			if i < len(rest) {
				return InferJobName(rest[i:])
			}
			return "conda-task"
		}
		for _, token := range parts[1:] {
			if strings.HasPrefix(token, "-") {
				continue
			}
			return strings.TrimSuffix(filepath.Base(token), filepath.Ext(token))
		}
		return "conda-task"
	}
	return strings.TrimSuffix(filepath.Base(parts[0]), filepath.Ext(parts[0]))
}

func JoinCommand(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		quoted = append(quoted, shellQuote(p))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\\' || r == '"' || r == '\'' || r == '$' || r == '`' || r == '!' || r == '|' || r == '&' || r == ';' || r == '<' || r == '>' || r == '(' || r == ')'
	}) == -1 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func BuildRunContext(runID, jobName string, cmd []string, cwd, logPath string, redactPatterns []string, tmuxSession string, pid int) (RunContext, error) {
	absCWD, err := filepath.Abs(cwd)
	if err != nil {
		return RunContext{}, fmt.Errorf("resolve cwd: %w", err)
	}
	host, _ := os.Hostname()
	lp := ""
	if logPath != "" {
		abslp, err := filepath.Abs(logPath)
		if err != nil {
			return RunContext{}, fmt.Errorf("resolve log path: %w", err)
		}
		lp = abslp
	}
	if tmuxSession == "" {
		tmuxSession = os.Getenv("TRAINPULSE_TMUX_SESSION")
	}
	return RunContext{
		RunID:       runID,
		Project:     DetectProjectName(absCWD),
		JobName:     jobName,
		Host:        host,
		CWD:         absCWD,
		GitBranch:   DetectGitBranch(absCWD),
		GitCommit:   DetectGitCommit(absCWD),
		Cmd:         RedactText(JoinCommand(cmd), redactPatterns),
		LogPath:     lp,
		TmuxSession: tmuxSession,
		PID:         pid,
		StartTime:   nowISO(),
	}, nil
}
