package status

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/trainpulse/trainpulse/internal/events"
	"github.com/trainpulse/trainpulse/internal/runtime"
	"github.com/trainpulse/trainpulse/internal/store"
	"github.com/trainpulse/trainpulse/internal/tmux"
)

const StopExitCode = 143

type Notifier interface {
	Send(payload map[string]any) bool
}

func SessionReachable(session string) bool {
	if session == "" {
		return false
	}
	if !tmux.HasTmux() {
		return false
	}
	return tmux.SessionExists(session)
}

func IsOrphanedRunningRun(run store.Run) bool {
	pidMissing := true
	if run.PID != nil {
		pidMissing = !runtime.PIDExists(*run.PID)
	}
	sessionMissing := !SessionReachable(run.TmuxSession)
	return pidMissing && sessionMissing
}

func IsReconcileTimeoutReached(run store.Run, staleMinutes *int, nowISO string) bool {
	if staleMinutes == nil || *staleMinutes <= 0 {
		return true
	}
	lastTouch := run.LastHeartbeat
	if lastTouch == "" {
		lastTouch = run.UpdatedAt
	}
	if lastTouch == "" {
		lastTouch = run.StartTime
	}
	lastDT, okLast := runtime.ParseISO(lastTouch)
	nowDT, okNow := runtime.ParseISO(nowISO)
	if !okLast || !okNow {
		return true
	}
	threshold := time.Duration(*staleMinutes) * time.Minute
	return nowDT.Sub(lastDT) >= threshold
}

func BuildTerminalPayload(run store.Run, event events.Event, endTime string, duration float64, exitCode int) map[string]any {
	payload := map[string]any{
		"run_id":     run.RunID,
		"project":    run.Project,
		"job_name":   run.JobName,
		"host":       run.Host,
		"cwd":        run.CWD,
		"git_branch": run.GitBranch,
		"git_commit": run.GitCommit,
		"cmd":        run.Cmd,
		"start_time": run.StartTime,
		"end_time":   endTime,
		"duration":   duration,
		"exit_code":  exitCode,
		"event":      string(event),
	}
	if run.LogPath != "" {
		payload["log_path"] = run.LogPath
	}
	if run.TmuxSession != "" {
		payload["tmux_session"] = run.TmuxSession
	}
	return payload
}

func FinalizeRunStopped(st *store.Store, run store.Run, nt Notifier) (bool, error) {
	endTime := runtime.NowISO()
	duration := runtime.DurationSeconds(run.StartTime, endTime)
	exitCode := StopExitCode
	updated, err := st.FinishRun(run.RunID, string(events.Stopped), &exitCode, endTime, duration)
	if err != nil {
		return false, err
	}
	if updated && nt != nil {
		nt.Send(BuildTerminalPayload(run, events.Stopped, endTime, duration, exitCode))
	}
	return updated, nil
}

func StopRun(st *store.Store, run store.Run, nt Notifier) (bool, bool, error) {
	signalSent := false
	if run.TmuxSession != "" && SessionReachable(run.TmuxSession) {
		tmux.StopSession(run.TmuxSession, 3*time.Second)
		signalSent = true
	}
	if run.PID != nil && *run.PID > 0 {
		err := syscall.Kill(*run.PID, syscall.SIGTERM)
		if err == nil {
			signalSent = true
		} else if err != nil && err != os.ErrProcessDone {
			if err.Error() == "no such process" {
				// ignore dead process
			} else {
				fmt.Fprintf(os.Stderr, "[trainpulse] stop pid failed: %v\n", err)
			}
		}
	}
	updated, err := FinalizeRunStopped(st, run, nt)
	return updated, signalSent, err
}
