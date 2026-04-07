package runtime

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	ctxpkg "github.com/trainpulse/trainpulse/internal/context"
	"github.com/trainpulse/trainpulse/internal/events"
)

type Notifier interface {
	Send(payload map[string]any) bool
}

type Store interface {
	StartRun(c ctxpkg.RunContext) error
	Heartbeat(runID string) error
	FinishRun(runID, event string, exitCode *int, endTime string, duration float64) (bool, error)
}

type CommandRunner struct {
	Notifier         Notifier
	Store            Store
	HeartbeatMinutes int
}

func DetermineFinalEvent(exitCode int, interrupted bool) events.Event {
	if interrupted {
		return events.Interrupted
	}
	if exitCode == 0 {
		return events.Succeeded
	}
	return events.Failed
}

func buildPayload(c ctxpkg.RunContext, event events.Event, endTime string, duration float64, exitCode *int) map[string]any {
	payload := map[string]any{
		"run_id":       c.RunID,
		"project":      c.Project,
		"job_name":     c.JobName,
		"host":         c.Host,
		"cwd":          c.CWD,
		"git_branch":   c.GitBranch,
		"git_commit":   c.GitCommit,
		"cmd":          c.Cmd,
		"log_path":     emptyToNil(c.LogPath),
		"start_time":   c.StartTime,
		"event":        string(event),
		"tmux_session": emptyToNil(c.TmuxSession),
	}
	if endTime != "" {
		payload["end_time"] = endTime
	}
	if duration >= 0 {
		payload["duration"] = duration
	}
	if exitCode != nil {
		payload["exit_code"] = *exitCode
	}
	return payload
}

func emptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *CommandRunner) Run(command []string, c ctxpkg.RunContext) int {
	startMono := time.Now()
	if r.HeartbeatMinutes <= 0 {
		r.HeartbeatMinutes = 30
	}
	var logFile *os.File
	if c.LogPath != "" {
		if err := EnsureParentDir(c.LogPath); err == nil {
			fp, err := os.OpenFile(c.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err == nil {
				logFile = fp
				defer logFile.Close()
			}
		}
	}

	cmd := exec.Command(command[0], command[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return r.failBeforeStart(c, 127, startMono)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return r.failBeforeStart(c, 127, startMono)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return r.failBeforeStart(c, 127, startMono)
	}
	c.PID = cmd.Process.Pid
	if err := r.Store.StartRun(c); err != nil {
		fmt.Fprintf(os.Stderr, "[trainpulse] start_run failed: %v\n", err)
	}
	if r.Notifier != nil {
		r.Notifier.Send(buildPayload(c, events.Started, "", -1, nil))
	}

	outWriter := io.Writer(os.Stdout)
	errWriter := io.Writer(os.Stderr)
	if logFile != nil {
		outWriter = io.MultiWriter(os.Stdout, logFile)
		errWriter = io.MultiWriter(os.Stderr, logFile)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(outWriter, stdout)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(errWriter, stderr)
	}()

	interrupted := false
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer func() {
		signal.Stop(sigCh)
		close(sigCh)
	}()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	heartbeatTicker := time.NewTicker(time.Duration(r.HeartbeatMinutes) * time.Minute)
	defer heartbeatTicker.Stop()

	var waitErr error
	for {
		select {
		case sig := <-sigCh:
			if sig == nil {
				continue
			}
			interrupted = true
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, sig.(syscall.Signal))
			}
		case waitErr = <-waitCh:
			goto FINISH
		case <-heartbeatTicker.C:
			_ = r.Store.Heartbeat(c.RunID)
		}
	}

FINISH:
	wg.Wait()
	exitCode := exitCodeFromWaitErr(waitErr)
	finalEvent := DetermineFinalEvent(exitCode, interrupted)
	endTime := NowISO()
	duration := float64(int64(time.Since(startMono).Seconds()*1000+0.5)) / 1000
	exitCodePtr := &exitCode
	updated, err := r.Store.FinishRun(c.RunID, string(finalEvent), exitCodePtr, endTime, duration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[trainpulse] finish_run failed: %v\n", err)
	}
	if updated && r.Notifier != nil && events.ShouldNotify(finalEvent) {
		r.Notifier.Send(buildPayload(c, finalEvent, endTime, duration, exitCodePtr))
	}
	return exitCode
}

func (r *CommandRunner) failBeforeStart(c ctxpkg.RunContext, code int, startMono time.Time) int {
	if err := r.Store.StartRun(c); err != nil {
		fmt.Fprintf(os.Stderr, "[trainpulse] start_run failed: %v\n", err)
	}
	endTime := NowISO()
	duration := float64(int64(time.Since(startMono).Seconds()*1000+0.5)) / 1000
	exitCode := code
	updated, _ := r.Store.FinishRun(c.RunID, string(events.Failed), &exitCode, endTime, duration)
	if updated && r.Notifier != nil {
		r.Notifier.Send(buildPayload(c, events.Failed, endTime, duration, &exitCode))
	}
	return code
}

func exitCodeFromWaitErr(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				return 128 + int(status.Signal())
			}
			if status.Exited() {
				return status.ExitStatus()
			}
		}
		code := exitErr.ExitCode()
		if code >= 0 {
			return NormalizeExitCode(code)
		}
	}
	return 1
}
