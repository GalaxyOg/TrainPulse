package tui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/trainpulse/trainpulse/internal/store"
)

type StopFunc func(runID string) (string, error)
type SetupFunc func() (string, error)

type Filter struct {
	Statuses    []string
	Since24h    bool
	ProjectLike string
	JobLike     string
	Limit       int
}

func Run(st *store.Store, stop StopFunc, setup SetupFunc) error {
	f := Filter{Limit: 20}
	reader := bufio.NewReader(os.Stdin)
	for {
		runs, err := queryRuns(st, f)
		if err != nil {
			return err
		}
		printScreen(runs, f)
		fmt.Print("tui> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])
		switch cmd {
		case "q", "quit", "exit":
			return nil
		case "h", "help":
			printHelp()
		case "r", "refresh":
			continue
		case "f", "filter":
			if len(parts) < 2 {
				fmt.Println("usage: f running|failed|succeeded|interrupted|all|24h|clear24|project=<k>|job=<k>|clear")
				continue
			}
			applyFilter(&f, parts[1])
		case "d", "detail":
			if len(parts) < 2 {
				fmt.Println("usage: d <idx|run_id>")
				continue
			}
			r, ok := pickRun(runs, parts[1])
			if !ok {
				fmt.Println("run not found")
				continue
			}
			printDetail(r)
		case "s", "stop":
			if len(parts) < 2 {
				fmt.Println("usage: s <idx|run_id>")
				continue
			}
			r, ok := pickRun(runs, parts[1])
			if !ok {
				fmt.Println("run not found")
				continue
			}
			if stop == nil {
				fmt.Println("stop is not configured")
				continue
			}
			msg, err := stop(r.RunID)
			if err != nil {
				fmt.Printf("stop failed: %v\n", err)
				continue
			}
			fmt.Println(msg)
		case "a", "attach":
			if len(parts) < 2 {
				fmt.Println("usage: a <idx|run_id>")
				continue
			}
			r, ok := pickRun(runs, parts[1])
			if !ok {
				fmt.Println("run not found")
				continue
			}
			if r.TmuxSession == "" {
				fmt.Println("tmux session not available")
			} else {
				fmt.Printf("tmux attach -t %s\n", r.TmuxSession)
			}
		case "l", "log":
			if len(parts) < 2 {
				fmt.Println("usage: l <idx|run_id>")
				continue
			}
			r, ok := pickRun(runs, parts[1])
			if !ok {
				fmt.Println("run not found")
				continue
			}
			if r.LogPath == "" {
				fmt.Println("log path not available")
			} else {
				fmt.Println(r.LogPath)
			}
		case "setup", "init":
			if setup == nil {
				fmt.Println("setup is not configured")
				continue
			}
			msg, err := setup()
			if err != nil {
				fmt.Printf("setup failed: %v\n", err)
				continue
			}
			fmt.Println(msg)
		default:
			fmt.Println("unknown command, type h for help")
		}
	}
}

func queryRuns(st *store.Store, f Filter) ([]store.Run, error) {
	since := ""
	if f.Since24h {
		since = time.Now().Add(-24 * time.Hour).In(time.FixedZone("UTC+8", 8*3600)).Format(time.RFC3339)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	return st.ListByFilters(f.Statuses, since, f.ProjectLike, f.JobLike, limit)
}

func printScreen(runs []store.Run, f Filter) {
	fmt.Print("\033[2J\033[H")
	fmt.Println("TrainPulse TUI (operations console)")
	fmt.Printf("filters: status=%v since24h=%v project=%q job=%q limit=%d\n", f.Statuses, f.Since24h, f.ProjectLike, f.JobLike, f.Limit)
	fmt.Println("idx | run_id | status | project | job | updated_at")
	for i, r := range runs {
		fmt.Printf("%d | %s | %s | %s | %s | %s\n", i+1, shortRunID(r.RunID), r.Status, r.Project, r.JobName, r.UpdatedAt)
	}
	fmt.Println("first-time: type `setup` to create/update config")
	fmt.Println("commands: h help | setup | r refresh | f ... | d <idx|run_id> | s <idx|run_id> | a <idx|run_id> | l <idx|run_id> | q")
}

func printHelp() {
	fmt.Println("setup            interactive first-time setup wizard")
	fmt.Println("f running|failed|succeeded|interrupted|all|24h|clear24|project=<k>|job=<k>|clear")
	fmt.Println("d <idx|run_id>  show run detail")
	fmt.Println("s <idx|run_id>  stop run")
	fmt.Println("a <idx|run_id>  show tmux attach command")
	fmt.Println("l <idx|run_id>  print log path")
	fmt.Println("q               quit")
}

func applyFilter(f *Filter, token string) {
	t := strings.ToLower(strings.TrimSpace(token))
	switch {
	case t == "running":
		f.Statuses = []string{"RUNNING"}
	case t == "failed":
		f.Statuses = []string{"FAILED"}
	case t == "succeeded":
		f.Statuses = []string{"SUCCEEDED"}
	case t == "interrupted":
		f.Statuses = []string{"INTERRUPTED"}
	case t == "stopped":
		f.Statuses = []string{"STOPPED"}
	case t == "all":
		f.Statuses = nil
	case t == "24h":
		f.Since24h = true
	case t == "clear24":
		f.Since24h = false
	case t == "clear":
		f.Statuses = nil
		f.Since24h = false
		f.ProjectLike = ""
		f.JobLike = ""
	case strings.HasPrefix(t, "project="):
		f.ProjectLike = strings.TrimSpace(strings.TrimPrefix(token, "project="))
	case strings.HasPrefix(t, "job="):
		f.JobLike = strings.TrimSpace(strings.TrimPrefix(token, "job="))
	}
}

func pickRun(runs []store.Run, key string) (store.Run, bool) {
	if idx, err := strconv.Atoi(key); err == nil {
		if idx >= 1 && idx <= len(runs) {
			return runs[idx-1], true
		}
	}
	for _, r := range runs {
		if r.RunID == key || shortRunID(r.RunID) == key {
			return r, true
		}
	}
	return store.Run{}, false
}

func shortRunID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func printDetail(r store.Run) {
	fmt.Println("---------------- run detail ----------------")
	fmt.Printf("run_id: %s\n", r.RunID)
	fmt.Printf("status: %s event: %s\n", r.Status, r.Event)
	fmt.Printf("project: %s job: %s\n", r.Project, r.JobName)
	fmt.Printf("host: %s cwd: %s\n", r.Host, r.CWD)
	fmt.Printf("git: %s@%s\n", dash(r.GitBranch), dash(r.GitCommit))
	fmt.Printf("start_time: %s\n", r.StartTime)
	fmt.Printf("end_time: %s\n", dash(r.EndTime))
	fmt.Printf("updated_at: %s\n", r.UpdatedAt)
	if r.ExitCode != nil {
		fmt.Printf("exit_code: %d\n", *r.ExitCode)
	} else {
		fmt.Println("exit_code: -")
	}
	fmt.Printf("duration: %.3fs\n", r.Duration)
	fmt.Printf("pid: %s\n", intPtr(r.PID))
	fmt.Printf("tmux_session: %s\n", dash(r.TmuxSession))
	fmt.Printf("log_path: %s\n", dash(r.LogPath))
	fmt.Printf("last_heartbeat: %s\n", dash(r.LastHeartbeat))
	fmt.Printf("command: %s\n", r.Cmd)
	fmt.Println("--------------------------------------------")
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func intPtr(v *int) string {
	if v == nil {
		return "-"
	}
	return strconv.Itoa(*v)
}
