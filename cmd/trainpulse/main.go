package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/trainpulse/trainpulse/internal/config"
	ctxpkg "github.com/trainpulse/trainpulse/internal/context"
	"github.com/trainpulse/trainpulse/internal/doctor"
	"github.com/trainpulse/trainpulse/internal/events"
	"github.com/trainpulse/trainpulse/internal/notifier"
	"github.com/trainpulse/trainpulse/internal/runtime"
	statussvc "github.com/trainpulse/trainpulse/internal/status"
	"github.com/trainpulse/trainpulse/internal/store"
	"github.com/trainpulse/trainpulse/internal/tmux"
	tuisvc "github.com/trainpulse/trainpulse/internal/tui"
	"github.com/trainpulse/trainpulse/internal/version"
)

type optionalString struct {
	set bool
	val string
}

func (o *optionalString) String() string { return o.val }
func (o *optionalString) Set(s string) error {
	o.set = true
	o.val = s
	return nil
}

type optionalInt struct {
	set bool
	val int
}

func (o *optionalInt) String() string { return strconv.Itoa(o.val) }
func (o *optionalInt) Set(s string) error {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return err
	}
	o.set = true
	o.val = v
	return nil
}

type optionalBool struct {
	set bool
	val bool
}

func (o *optionalBool) String() string {
	if o.val {
		return "true"
	}
	return "false"
}
func (o *optionalBool) Set(s string) error {
	v, err := strconv.ParseBool(strings.TrimSpace(s))
	if err != nil {
		return err
	}
	o.set = true
	o.val = v
	return nil
}

type stringSlice struct {
	set    bool
	values []string
}

func (s *stringSlice) String() string {
	return strings.Join(s.values, ",")
}
func (s *stringSlice) Set(v string) error {
	s.set = true
	s.values = append(s.values, v)
	return nil
}

type runtimeFlags struct {
	configPath  string
	webhookURL  optionalString
	messageType optionalString
	storePath   optionalString
	errorLog    optionalString
	heartbeat   optionalInt
	dryRun      optionalBool
	redact      stringSlice
}

func addRuntimeFlags(fs *flag.FlagSet, rf *runtimeFlags, withHeartbeat bool) {
	fs.StringVar(&rf.configPath, "config", config.DefaultConfigPath, "config path")
	fs.Var(&rf.webhookURL, "webhook-url", "feishu webhook url")
	fs.Var(&rf.messageType, "message-type", "text or post")
	fs.Var(&rf.storePath, "store-path", "sqlite store path")
	fs.Var(&rf.errorLog, "error-log-path", "notifier error log path")
	if withHeartbeat {
		fs.Var(&rf.heartbeat, "heartbeat-minutes", "silent heartbeat interval minutes")
	}
	fs.Var(&rf.dryRun, "dry-run", "dry run true|false")
	fs.Var(&rf.redact, "redact", "redact regex (repeatable)")
}

func preprocessArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--no-dry-run" {
			out = append(out, "--dry-run=false")
			continue
		}
		if a == "--dry-run" {
			out = append(out, "--dry-run=true")
			continue
		}
		out = append(out, a)
	}
	return out
}

func runtimeInputFromFlags(rf runtimeFlags) config.RuntimeInput {
	in := config.RuntimeInput{ConfigPath: rf.configPath}
	if rf.webhookURL.set {
		in.WebhookURL = &rf.webhookURL.val
	}
	if rf.messageType.set {
		in.MessageType = &rf.messageType.val
	}
	if rf.storePath.set {
		in.StorePath = &rf.storePath.val
	}
	if rf.errorLog.set {
		in.ErrorLogPath = &rf.errorLog.val
	}
	if rf.heartbeat.set {
		in.HeartbeatMinutes = &rf.heartbeat.val
	}
	if rf.dryRun.set {
		in.DryRun = &rf.dryRun.val
	}
	if rf.redact.set {
		v := append([]string{}, rf.redact.values...)
		in.Redact = &v
	}
	return in
}

func generateRunID() string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%s-%s", runtime.NowCompact(), hex.EncodeToString(buf))
}

func normalizeCommandArgs(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}

func buildNotifier(rt config.Runtime) *notifier.FeishuNotifier {
	if rt.WebhookURL == "" && !rt.DryRun {
		return nil
	}
	return notifier.New(rt.WebhookURL, rt.MessageType, rt.DryRun, rt.ErrorLogPath)
}

func commandRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var rf runtimeFlags
	addRuntimeFlags(fs, &rf, true)
	jobName := fs.String("job-name", "", "job name")
	logPath := fs.String("log-path", "", "log path")
	runID := fs.String("run-id", "", "internal run id")
	if err := fs.Parse(preprocessArgs(args)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	cmd := normalizeCommandArgs(fs.Args())
	if len(cmd) == 0 {
		fmt.Fprintln(os.Stderr, "error: missing command, use: trainpulse run -- <command...>")
		return 2
	}
	rt, err := config.ResolveRuntime(runtimeInputFromFlags(rf))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve runtime failed: %v\n", err)
		return 2
	}
	cwd, _ := os.Getwd()
	rid := *runID
	if strings.TrimSpace(rid) == "" {
		rid = generateRunID()
	}
	jn := *jobName
	if strings.TrimSpace(jn) == "" {
		jn = ctxpkg.InferJobName(cmd)
	}
	ctx, err := ctxpkg.BuildRunContext(rid, jn, cmd, cwd, *logPath, rt.Redact, "", 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	st, err := store.New(rt.StorePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open store failed: %v\n", err)
		return 2
	}
	defer st.Close()

	runner := runtime.CommandRunner{Notifier: buildNotifier(rt), Store: st, HeartbeatMinutes: rt.HeartbeatMinutes}
	return runner.Run(cmd, ctx)
}

func commandTmuxRun(args []string) int {
	fs := flag.NewFlagSet("tmux-run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var rf runtimeFlags
	addRuntimeFlags(fs, &rf, true)
	session := fs.String("session", "", "tmux session")
	jobName := fs.String("job-name", "", "job name")
	logPath := fs.String("log-path", "", "log path")
	cwd := fs.String("cwd", "", "working directory")
	if err := fs.Parse(preprocessArgs(args)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*session) == "" {
		fmt.Fprintln(os.Stderr, "error: --session is required")
		return 2
	}
	cmd := normalizeCommandArgs(fs.Args())
	if len(cmd) == 0 {
		fmt.Fprintln(os.Stderr, "error: missing command, use: trainpulse tmux-run --session s -- <command...>")
		return 2
	}
	if !tmux.HasTmux() {
		fmt.Fprintln(os.Stderr, "error: tmux is not installed; use `trainpulse run` instead.")
		return 2
	}
	if tmux.SessionExists(*session) {
		fmt.Fprintf(os.Stderr, "error: tmux session already exists: %s\n", *session)
		return 2
	}
	rt, err := config.ResolveRuntime(runtimeInputFromFlags(rf))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve runtime failed: %v\n", err)
		return 2
	}
	rid := generateRunID()
	exePath, _ := os.Executable()
	inner := []string{exePath, "run", "--run-id", rid, "--config", rf.configPath, "--store-path", rt.StorePath, "--message-type", rt.MessageType, "--error-log-path", rt.ErrorLogPath}
	if *jobName != "" {
		inner = append(inner, "--job-name", *jobName)
	}
	if *logPath != "" {
		inner = append(inner, "--log-path", *logPath)
	}
	if rt.WebhookURL != "" {
		inner = append(inner, "--webhook-url", rt.WebhookURL)
	}
	if rt.HeartbeatMinutes > 0 {
		inner = append(inner, "--heartbeat-minutes", strconv.Itoa(rt.HeartbeatMinutes))
	}
	if rt.DryRun {
		inner = append(inner, "--dry-run=true")
	} else {
		inner = append(inner, "--dry-run=false")
	}
	for _, p := range rt.Redact {
		inner = append(inner, "--redact", p)
	}
	inner = append(inner, "--")
	inner = append(inner, cmd...)

	commandStr := ctxpkg.JoinCommand(inner)
	wrapped := fmt.Sprintf("TRAINPULSE_TMUX_SESSION=%s %s", shellQuote(*session), commandStr)
	targetCWD := *cwd
	if strings.TrimSpace(targetCWD) == "" {
		targetCWD, _ = os.Getwd()
	}
	if err := tmux.StartDetachedSession(*session, wrapped, targetCWD); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	fmt.Printf("tmux task started: run_id=%s session=%s\n", rid, *session)
	return 0
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func commandStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var rf runtimeFlags
	addRuntimeFlags(fs, &rf, false)
	limit := fs.Int("limit", 20, "list limit")
	runningOnly := fs.Bool("running-only", false, "only RUNNING")
	reconcile := fs.Bool("reconcile", false, "reconcile orphaned runs")
	var stale optionalInt
	fs.Var(&stale, "reconcile-stale-minutes", "reconcile only when stale")
	if err := fs.Parse(preprocessArgs(args)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	rt, err := config.ResolveRuntime(runtimeInputFromFlags(rf))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve runtime failed: %v\n", err)
		return 2
	}
	st, err := store.New(rt.StorePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open store failed: %v\n", err)
		return 2
	}
	defer st.Close()
	nt := buildNotifier(rt)

	if *reconcile {
		running, err := st.ListRuns(nil, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: list runs failed: %v\n", err)
			return 1
		}
		reconciled := 0
		nowISO := runtime.NowISO()
		var stalePtr *int
		if stale.set {
			stalePtr = &stale.val
		}
		for _, run := range running {
			if run.Status != "RUNNING" {
				continue
			}
			if !statussvc.IsOrphanedRunningRun(run) {
				continue
			}
			if !statussvc.IsReconcileTimeoutReached(run, stalePtr, nowISO) {
				continue
			}
			updated, err := statussvc.FinalizeRunStopped(st, run, nt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: reconcile run %s failed: %v\n", run.RunID, err)
				continue
			}
			if updated {
				reconciled++
			}
		}
		if reconciled > 0 {
			fmt.Printf("reconciled %d orphaned RUNNING run(s)\n", reconciled)
		}
	}

	var lp *int
	if *limit >= 0 {
		lp = limit
	}
	rows, err := st.ListRuns(lp, *runningOnly)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: list runs failed: %v\n", err)
		return 1
	}
	if len(rows) == 0 {
		fmt.Println("no runs found")
		return 0
	}
	fmt.Println("run_id | status | project | job_name | exit_code | updated_at | tmux_session")
	for _, row := range rows {
		exitCode := "<nil>"
		if row.ExitCode != nil {
			exitCode = strconv.Itoa(*row.ExitCode)
		}
		tm := row.TmuxSession
		if tm == "" {
			tm = "-"
		}
		fmt.Printf("%s | %s | %s | %s | %s | %s | %s\n", row.RunID, row.Status, row.Project, row.JobName, exitCode, row.UpdatedAt, tm)
	}
	return 0
}

func commandStop(args []string) int {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var rf runtimeFlags
	addRuntimeFlags(fs, &rf, false)
	runID := fs.String("run-id", "", "run id")
	if err := fs.Parse(preprocessArgs(args)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*runID) == "" {
		fmt.Fprintln(os.Stderr, "error: --run-id is required")
		return 2
	}
	rt, err := config.ResolveRuntime(runtimeInputFromFlags(rf))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve runtime failed: %v\n", err)
		return 2
	}
	st, err := store.New(rt.StorePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open store failed: %v\n", err)
		return 2
	}
	defer st.Close()
	r, err := st.GetRun(*runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: get run failed: %v\n", err)
		return 1
	}
	if r == nil {
		fmt.Fprintf(os.Stderr, "error: run not found: %s\n", *runID)
		return 1
	}
	if events.IsTerminalStatus(r.Status) {
		fmt.Printf("run already in terminal state: %s (%s)\n", *runID, r.Status)
		return 0
	}
	updated, signalSent, err := statussvc.StopRun(st, *r, buildNotifier(rt))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: stop failed: %v\n", err)
		return 1
	}
	if updated {
		if signalSent {
			fmt.Printf("stop signal sent and run finalized: %s\n", *runID)
		} else {
			fmt.Printf("run finalized as STOPPED (target already exited): %s\n", *runID)
		}
		return 0
	}
	fmt.Printf("warning: run already finished: %s\n", *runID)
	return 0
}

func commandLogs(args []string) int {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var rf runtimeFlags
	addRuntimeFlags(fs, &rf, false)
	runID := fs.String("run-id", "", "run id")
	tailN := fs.Int("tail", 80, "tail lines")
	follow := fs.Bool("follow", false, "follow mode")
	printPath := fs.Bool("print-path", false, "print log path only")
	if err := fs.Parse(preprocessArgs(args)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	rt, err := config.ResolveRuntime(runtimeInputFromFlags(rf))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve runtime failed: %v\n", err)
		return 2
	}
	st, err := store.New(rt.StorePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open store failed: %v\n", err)
		return 2
	}
	defer st.Close()

	r, err := pickRunForLogs(st, *runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if r.LogPath == "" {
		fmt.Fprintln(os.Stderr, "error: run has no log_path")
		return 1
	}
	if *printPath {
		fmt.Println(r.LogPath)
		return 0
	}
	if err := printTail(r.LogPath, *tailN); err != nil {
		fmt.Fprintf(os.Stderr, "error: tail logs failed: %v\n", err)
		return 1
	}
	if *follow {
		if err := followFile(r.LogPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: follow logs failed: %v\n", err)
			return 1
		}
	}
	return 0
}

func pickRunForLogs(st *store.Store, runID string) (store.Run, error) {
	if strings.TrimSpace(runID) != "" {
		r, err := st.GetRun(runID)
		if err != nil {
			return store.Run{}, err
		}
		if r == nil {
			return store.Run{}, fmt.Errorf("run not found: %s", runID)
		}
		return *r, nil
	}
	rows, err := st.ListRuns(nil, false)
	if err != nil {
		return store.Run{}, err
	}
	for _, r := range rows {
		if r.LogPath != "" {
			return r, nil
		}
	}
	return store.Run{}, errors.New("no runs with log_path found")
}

func printTail(path string, n int) error {
	if n <= 0 {
		n = 80
	}
	fp, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fp.Close()
	scanner := bufio.NewScanner(fp)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}

func followFile(path string) error {
	fp, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fp.Close()
	pos, err := fp.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	buf := make([]byte, 8192)
	for {
		time.Sleep(1 * time.Second)
		cur, err := fp.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		st, err := fp.Stat()
		if err != nil {
			return err
		}
		if st.Size() < pos {
			pos, _ = fp.Seek(0, io.SeekStart)
		}
		if st.Size() == cur {
			continue
		}
		for {
			n, err := fp.Read(buf)
			if n > 0 {
				_, _ = os.Stdout.Write(buf[:n])
				pos += int64(n)
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
		}
	}
}

func commandDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var rf runtimeFlags
	addRuntimeFlags(fs, &rf, true)
	if err := fs.Parse(preprocessArgs(args)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	rt, err := config.ResolveRuntime(runtimeInputFromFlags(rf))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve runtime failed: %v\n", err)
		return 2
	}
	report := doctor.Run(rt)
	for _, it := range report.Items {
		flag := "OK"
		if !it.OK {
			flag = "FAIL"
		}
		fmt.Printf("[%s] %s: %s\n", flag, it.Name, it.Message)
	}
	if report.AllOK() {
		return 0
	}
	return 1
}

func commandTUI(args []string) int {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var rf runtimeFlags
	addRuntimeFlags(fs, &rf, false)
	if err := fs.Parse(preprocessArgs(args)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	rt, err := config.ResolveRuntime(runtimeInputFromFlags(rf))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve runtime failed: %v\n", err)
		return 2
	}
	st, err := store.New(rt.StorePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open store failed: %v\n", err)
		return 2
	}
	defer st.Close()
	nt := buildNotifier(rt)
	stopFn := func(runID string) (string, error) {
		r, err := st.GetRun(runID)
		if err != nil {
			return "", err
		}
		if r == nil {
			return "", fmt.Errorf("run not found: %s", runID)
		}
		if events.IsTerminalStatus(r.Status) {
			return fmt.Sprintf("run already terminal: %s (%s)", runID, r.Status), nil
		}
		updated, signalSent, err := statussvc.StopRun(st, *r, nt)
		if err != nil {
			return "", err
		}
		if updated {
			if signalSent {
				return "stop signal sent and run finalized", nil
			}
			return "run finalized as STOPPED", nil
		}
		return "run already finished", nil
	}
	doctorFn := func() (string, error) {
		report := doctor.Run(rt)
		lines := make([]string, 0, len(report.Items)+1)
		for _, it := range report.Items {
			flag := "OK"
			if !it.OK {
				flag = "FAIL"
			}
			lines = append(lines, fmt.Sprintf("[%s] %s: %s", flag, it.Name, it.Message))
		}
		if report.AllOK() {
			lines = append(lines, "doctor summary: all checks passed")
		}
		return strings.Join(lines, "\n"), nil
	}
	if err := tuisvc.Run(tuisvc.Options{
		Store:           st,
		Stop:            stopFn,
		Doctor:          doctorFn,
		Version:         version.Version,
		StorePath:       rt.StorePath,
		ConfigPath:      rf.configPath,
		ErrorLogPath:    rt.ErrorLogPath,
		RefreshInterval: 3 * time.Second,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: tui failed: %v\n", err)
		return 1
	}
	return 0
}

func commandConfig(args []string) int {
	if len(args) == 0 {
		fmt.Println("usage: trainpulse config path|example|check")
		return 2
	}
	sub := args[0]
	switch sub {
	case "path":
		fmt.Println(config.ExpandPath(config.DefaultConfigPath))
		return 0
	case "example":
		fmt.Print(config.ExampleConfig())
		return 0
	case "check":
		fs := flag.NewFlagSet("config-check", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var rf runtimeFlags
		addRuntimeFlags(fs, &rf, true)
		if err := fs.Parse(preprocessArgs(args[1:])); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 2
		}
		rt, err := config.ResolveRuntime(runtimeInputFromFlags(rf))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: resolve runtime failed: %v\n", err)
			return 2
		}
		fmt.Printf("config_path=%s\n", rt.ConfigPath)
		fmt.Printf("webhook_url=%s\n", maskWebhook(rt.WebhookURL))
		fmt.Printf("message_type=%s\n", rt.MessageType)
		fmt.Printf("store_path=%s\n", rt.StorePath)
		fmt.Printf("error_log_path=%s\n", rt.ErrorLogPath)
		fmt.Printf("heartbeat_minutes=%d\n", rt.HeartbeatMinutes)
		fmt.Printf("dry_run=%v\n", rt.DryRun)
		fmt.Printf("redact=%v\n", rt.Redact)
		return 0
	default:
		fmt.Println("usage: trainpulse config path|example|check")
		return 2
	}
}

func maskWebhook(url string) string {
	if url == "" {
		return ""
	}
	idx := strings.Index(url, "/hook/")
	if idx < 0 {
		return "***"
	}
	return url[:idx+6] + "***"
}

func commandVersion() int {
	fmt.Printf("trainpulse version %s\n", version.Version)
	if version.Commit != "dev" || version.Date != "unknown" {
		fmt.Printf("commit=%s date=%s\n", version.Commit, version.Date)
	}
	return 0
}

func usage() {
	fmt.Println("TrainPulse (Go)")
	fmt.Println("usage: trainpulse <command> [args]")
	fmt.Println("commands:")
	fmt.Println("  run        run a command with notification + sqlite state")
	fmt.Println("  tmux-run   run command in detached tmux session")
	fmt.Println("  status     show runs and reconcile orphaned running runs")
	fmt.Println("  stop       stop a run by run_id")
	fmt.Println("  logs       show run logs")
	fmt.Println("  doctor     environment checks")
	fmt.Println("  tui        interactive operations console")
	fmt.Println("  config     path|example|check")
	fmt.Println("  version    show version")
}

func main() {
	os.Exit(runMain(os.Args[1:]))
}

func runMain(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "run":
		return commandRun(args[1:])
	case "tmux-run":
		return commandTmuxRun(args[1:])
	case "status":
		return commandStatus(args[1:])
	case "stop":
		return commandStop(args[1:])
	case "logs":
		return commandLogs(args[1:])
	case "doctor":
		return commandDoctor(args[1:])
	case "tui":
		return commandTUI(args[1:])
	case "config":
		return commandConfig(args[1:])
	case "version", "--version", "-v":
		return commandVersion()
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n", args[0])
		usage()
		return 2
	}
}
