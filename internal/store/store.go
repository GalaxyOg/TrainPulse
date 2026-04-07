package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	ctxpkg "github.com/trainpulse/trainpulse/internal/context"
	"github.com/trainpulse/trainpulse/internal/events"
	"github.com/trainpulse/trainpulse/internal/runtime"
)

type Run struct {
	RunID         string
	Event         string
	Status        string
	Project       string
	JobName       string
	Cmd           string
	Host          string
	CWD           string
	GitBranch     string
	GitCommit     string
	LogPath       string
	StartTime     string
	EndTime       string
	Duration      float64
	ExitCode      *int
	PID           *int
	TmuxSession   string
	LastHeartbeat string
	UpdatedAt     string
}

type Store struct {
	dbPath string
	db     *sql.DB
}

func New(path string) (*Store, error) {
	path = expand(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir store dir: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{dbPath: path, db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func expand(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) init() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS runs (
            run_id TEXT PRIMARY KEY,
            event TEXT NOT NULL,
            status TEXT NOT NULL,
            project TEXT NOT NULL,
            job_name TEXT NOT NULL,
            cmd TEXT NOT NULL,
            host TEXT NOT NULL,
            cwd TEXT NOT NULL,
            git_branch TEXT,
            git_commit TEXT,
            log_path TEXT,
            start_time TEXT NOT NULL,
            end_time TEXT,
            duration REAL,
            exit_code INTEGER,
            pid INTEGER,
            tmux_session TEXT,
            last_heartbeat TEXT,
            updated_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_runs_updated_at ON runs(updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init db: %w", err)
		}
	}
	return nil
}

func (s *Store) StartRun(c ctxpkg.RunContext) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO runs (
            run_id, event, status, project, job_name, cmd, host, cwd,
            git_branch, git_commit, log_path, start_time, end_time,
            duration, exit_code, pid, tmux_session, last_heartbeat, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.RunID,
		string(events.Started),
		"RUNNING",
		c.Project,
		c.JobName,
		c.Cmd,
		c.Host,
		c.CWD,
		nullIfEmpty(c.GitBranch),
		nullIfEmpty(c.GitCommit),
		nullIfEmpty(c.LogPath),
		c.StartTime,
		nil,
		nil,
		nil,
		nullIfZero(c.PID),
		nullIfEmpty(c.TmuxSession),
		nil,
		runtime.NowISO(),
	)
	if err != nil {
		return fmt.Errorf("start run: %w", err)
	}
	return nil
}

func (s *Store) Heartbeat(runID string) error {
	ts := runtime.NowISO()
	_, err := s.db.Exec(`UPDATE runs
        SET event = ?, last_heartbeat = ?, updated_at = ?
        WHERE run_id = ?`, string(events.Heartbeat), ts, ts, runID)
	if err != nil {
		return fmt.Errorf("heartbeat: %w", err)
	}
	return nil
}

func (s *Store) FinishRun(runID, event string, exitCode *int, endTime string, duration float64) (bool, error) {
	status := event
	if event == string(events.Succeeded) {
		status = "SUCCEEDED"
	}
	res, err := s.db.Exec(`UPDATE runs
        SET event = ?, status = ?, exit_code = ?, end_time = ?, duration = ?, updated_at = ?
        WHERE run_id = ? AND status = 'RUNNING'`,
		event, status, exitCode, endTime, duration, runtime.NowISO(), runID)
	if err != nil {
		return false, fmt.Errorf("finish run: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, nil
	}
	return n > 0, nil
}

func (s *Store) GetRun(runID string) (*Run, error) {
	row := s.db.QueryRow(`SELECT * FROM runs WHERE run_id = ?`, runID)
	r, ok, err := scanOne(row)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return &r, nil
}

func (s *Store) ListRuns(limit *int, runningOnly bool) ([]Run, error) {
	query := `SELECT * FROM runs`
	args := []any{}
	if runningOnly {
		query += ` WHERE status = 'RUNNING'`
	}
	query += ` ORDER BY updated_at DESC`
	if limit != nil {
		query += ` LIMIT ?`
		args = append(args, *limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	items := []Run{}
	for rows.Next() {
		r, err := scanRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListByFilters(statuses []string, sinceISO string, projectLike string, jobLike string, limit int) ([]Run, error) {
	query := `SELECT * FROM runs WHERE 1=1`
	args := []any{}
	if len(statuses) > 0 {
		placeholders := make([]string, 0, len(statuses))
		for _, st := range statuses {
			placeholders = append(placeholders, "?")
			args = append(args, st)
		}
		query += ` AND status IN (` + strings.Join(placeholders, ",") + `)`
	}
	if sinceISO != "" {
		query += ` AND updated_at >= ?`
		args = append(args, sinceISO)
	}
	if projectLike != "" {
		query += ` AND project LIKE ?`
		args = append(args, "%"+projectLike+"%")
	}
	if jobLike != "" {
		query += ` AND job_name LIKE ?`
		args = append(args, "%"+jobLike+"%")
	}
	query += ` ORDER BY updated_at DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list by filters: %w", err)
	}
	defer rows.Close()
	items := []Run{}
	for rows.Next() {
		r, err := scanRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	return items, rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func scanOne(row *sql.Row) (Run, bool, error) {
	var (
		r                                      Run
		gitBranch, gitCommit, logPath, endTime sql.NullString
		tmuxSession, heartbeat                 sql.NullString
		duration                               sql.NullFloat64
		exitCode, pid                          sql.NullInt64
	)
	err := row.Scan(
		&r.RunID,
		&r.Event,
		&r.Status,
		&r.Project,
		&r.JobName,
		&r.Cmd,
		&r.Host,
		&r.CWD,
		&gitBranch,
		&gitCommit,
		&logPath,
		&r.StartTime,
		&endTime,
		&duration,
		&exitCode,
		&pid,
		&tmuxSession,
		&heartbeat,
		&r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Run{}, false, nil
	}
	if err != nil {
		return Run{}, false, err
	}
	assignNullable(&r, gitBranch, gitCommit, logPath, endTime, duration, exitCode, pid, tmuxSession, heartbeat)
	return r, true, nil
}

func scanRows(rows *sql.Rows) (Run, error) {
	var (
		r                                      Run
		gitBranch, gitCommit, logPath, endTime sql.NullString
		tmuxSession, heartbeat                 sql.NullString
		duration                               sql.NullFloat64
		exitCode, pid                          sql.NullInt64
	)
	err := rows.Scan(
		&r.RunID,
		&r.Event,
		&r.Status,
		&r.Project,
		&r.JobName,
		&r.Cmd,
		&r.Host,
		&r.CWD,
		&gitBranch,
		&gitCommit,
		&logPath,
		&r.StartTime,
		&endTime,
		&duration,
		&exitCode,
		&pid,
		&tmuxSession,
		&heartbeat,
		&r.UpdatedAt,
	)
	if err != nil {
		return Run{}, err
	}
	assignNullable(&r, gitBranch, gitCommit, logPath, endTime, duration, exitCode, pid, tmuxSession, heartbeat)
	return r, nil
}

func assignNullable(r *Run, gitBranch, gitCommit, logPath, endTime sql.NullString, duration sql.NullFloat64, exitCode, pid sql.NullInt64, tmuxSession, heartbeat sql.NullString) {
	if gitBranch.Valid {
		r.GitBranch = gitBranch.String
	}
	if gitCommit.Valid {
		r.GitCommit = gitCommit.String
	}
	if logPath.Valid {
		r.LogPath = logPath.String
	}
	if endTime.Valid {
		r.EndTime = endTime.String
	}
	if duration.Valid {
		r.Duration = duration.Float64
	}
	if exitCode.Valid {
		v := int(exitCode.Int64)
		r.ExitCode = &v
	}
	if pid.Valid {
		v := int(pid.Int64)
		r.PID = &v
	}
	if tmuxSession.Valid {
		r.TmuxSession = tmuxSession.String
	}
	if heartbeat.Valid {
		r.LastHeartbeat = heartbeat.String
	}
}
