from __future__ import annotations

import os
import shlex
import socket
import subprocess
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterable, Optional

from .redact import redact_text


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def _run_git(args: list[str], cwd: Path) -> Optional[str]:
    try:
        proc = subprocess.run(
            ["git", *args],
            cwd=str(cwd),
            check=False,
            capture_output=True,
            text=True,
        )
    except Exception:
        return None
    if proc.returncode != 0:
        return None
    value = proc.stdout.strip()
    return value or None


def detect_project_name(cwd: str | Path) -> str:
    path = Path(cwd).resolve()
    top_level = _run_git(["rev-parse", "--show-toplevel"], cwd=path)
    if top_level:
        return Path(top_level).name
    return path.name


def detect_git_branch(cwd: str | Path) -> Optional[str]:
    return _run_git(["rev-parse", "--abbrev-ref", "HEAD"], cwd=Path(cwd))


def detect_git_commit(cwd: str | Path) -> Optional[str]:
    return _run_git(["rev-parse", "--short", "HEAD"], cwd=Path(cwd))


def infer_job_name(cmd: Iterable[str]) -> str:
    parts = list(cmd)
    if not parts:
        return "unknown-job"

    first = Path(parts[0]).name
    if first == "uv" and len(parts) >= 3 and parts[1] == "run":
        return infer_job_name(parts[2:])

    if first in {"python", "python3", "python3.10", "python3.11", "uv"}:
        for token in parts[1:]:
            if token.startswith("-"):
                continue
            return Path(token).stem
    if first == "conda":
        if "run" in parts:
            run_index = parts.index("run")
            rest = parts[run_index + 1 :]
            value_options = {"-n", "--name", "-p", "--prefix"}
            idx = 0
            while idx < len(rest) and rest[idx].startswith("-"):
                opt = rest[idx]
                idx += 2 if opt in value_options and idx + 1 < len(rest) else 1
            if idx < len(rest):
                return infer_job_name(rest[idx:])
            return "conda-task"
        for idx, token in enumerate(parts[1:], start=1):
            if token.startswith("-"):
                continue
            return Path(token).stem
        return "conda-task"
    return Path(parts[0]).stem


def format_cmd(cmd: Iterable[str], redact_patterns: list[str]) -> str:
    raw = shlex.join(list(cmd))
    return redact_text(raw, redact_patterns)


def build_run_context(
    run_id: str,
    job_name: str,
    cmd: Iterable[str],
    cwd: str | Path,
    log_path: Optional[str],
    redact_patterns: list[str],
    tmux_session: Optional[str] = None,
    pid: Optional[int] = None,
) -> dict:
    cwd_path = Path(cwd).resolve()
    return {
        "run_id": run_id,
        "project": detect_project_name(cwd_path),
        "job_name": job_name,
        "host": socket.gethostname(),
        "cwd": str(cwd_path),
        "git_branch": detect_git_branch(cwd_path),
        "git_commit": detect_git_commit(cwd_path),
        "cmd": format_cmd(cmd, redact_patterns),
        "log_path": str(Path(log_path).resolve()) if log_path else None,
        "tmux_session": tmux_session or os.getenv("TRAIN_NOTIFY_TMUX_SESSION"),
        "pid": pid,
        "start_time": now_iso(),
    }
