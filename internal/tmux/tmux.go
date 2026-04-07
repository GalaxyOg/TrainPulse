package tmux

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func HasTmux() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

func SessionExists(session string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", session)
	return cmd.Run() == nil
}

func StartDetachedSession(session, command, cwd string) error {
	shellCmd := command
	if strings.TrimSpace(cwd) != "" {
		shellCmd = fmt.Sprintf("cd %s && %s", shellEscape(cwd), command)
	}
	cmd := exec.Command("tmux", "new-session", "-d", "-s", session, shellCmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("failed to start tmux session: %s", msg)
	}
	return nil
}

func shellEscape(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func SendCtrlC(session string) bool {
	cmd := exec.Command("tmux", "send-keys", "-t", session, "C-c")
	return cmd.Run() == nil
}

func StopSession(session string, grace time.Duration) {
	_ = SendCtrlC(session)
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if !SessionExists(session) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = exec.Command("tmux", "kill-session", "-t", session).Run()
}
