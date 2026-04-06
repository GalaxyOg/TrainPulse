from __future__ import annotations

import sqlite3
from contextlib import contextmanager
from pathlib import Path
from typing import Iterator, Optional

from .timeutil import now_iso

def _now() -> str:
    return now_iso()


class RunStore:
    def __init__(self, db_path: str) -> None:
        self.db_path = Path(db_path).expanduser()
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        self._init_db()

    @contextmanager
    def _conn(self) -> Iterator[sqlite3.Connection]:
        conn = sqlite3.connect(str(self.db_path), timeout=10)
        try:
            conn.row_factory = sqlite3.Row
            yield conn
            conn.commit()
        finally:
            conn.close()

    def _init_db(self) -> None:
        with self._conn() as conn:
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS runs (
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
                )
                """
            )
            conn.execute(
                "CREATE INDEX IF NOT EXISTS idx_runs_updated_at ON runs(updated_at DESC)"
            )
            conn.execute(
                "CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status)"
            )

    def start_run(self, context: dict) -> None:
        with self._conn() as conn:
            conn.execute(
                """
                INSERT OR REPLACE INTO runs (
                    run_id, event, status, project, job_name, cmd, host, cwd,
                    git_branch, git_commit, log_path, start_time, end_time,
                    duration, exit_code, pid, tmux_session, last_heartbeat, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    context["run_id"],
                    "STARTED",
                    "RUNNING",
                    context["project"],
                    context["job_name"],
                    context["cmd"],
                    context["host"],
                    context["cwd"],
                    context.get("git_branch"),
                    context.get("git_commit"),
                    context.get("log_path"),
                    context["start_time"],
                    None,
                    None,
                    None,
                    context.get("pid"),
                    context.get("tmux_session"),
                    None,
                    _now(),
                ),
            )

    def heartbeat(self, run_id: str) -> None:
        ts = _now()
        with self._conn() as conn:
            conn.execute(
                """
                UPDATE runs
                SET event = ?, last_heartbeat = ?, updated_at = ?
                WHERE run_id = ?
                """,
                ("HEARTBEAT", ts, ts, run_id),
            )

    def finish_run(
        self,
        run_id: str,
        event: str,
        exit_code: Optional[int],
        end_time: str,
        duration: float,
    ) -> bool:
        status = "SUCCEEDED" if event == "SUCCEEDED" else event
        with self._conn() as conn:
            result = conn.execute(
                """
                UPDATE runs
                SET event = ?, status = ?, exit_code = ?, end_time = ?, duration = ?, updated_at = ?
                WHERE run_id = ? AND status = 'RUNNING'
                """,
                (event, status, exit_code, end_time, duration, _now(), run_id),
            )
            return result.rowcount > 0

    def get_run(self, run_id: str) -> Optional[dict]:
        with self._conn() as conn:
            row = conn.execute("SELECT * FROM runs WHERE run_id = ?", (run_id,)).fetchone()
            return dict(row) if row else None

    def list_runs(self, limit: Optional[int] = 20, running_only: bool = False) -> list[dict]:
        with self._conn() as conn:
            if running_only:
                if limit is None:
                    rows = conn.execute(
                        """
                        SELECT * FROM runs
                        WHERE status = 'RUNNING'
                        ORDER BY updated_at DESC
                        """
                    ).fetchall()
                else:
                    rows = conn.execute(
                        """
                        SELECT * FROM runs
                        WHERE status = 'RUNNING'
                        ORDER BY updated_at DESC
                        LIMIT ?
                        """,
                        (limit,),
                    ).fetchall()
            else:
                if limit is None:
                    rows = conn.execute(
                        """
                        SELECT * FROM runs
                        ORDER BY updated_at DESC
                        """
                    ).fetchall()
                else:
                    rows = conn.execute(
                        """
                        SELECT * FROM runs
                        ORDER BY updated_at DESC
                        LIMIT ?
                        """,
                        (limit,),
                    ).fetchall()
            return [dict(row) for row in rows]
