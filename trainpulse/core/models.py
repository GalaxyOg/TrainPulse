from __future__ import annotations

from dataclasses import dataclass
from enum import Enum
from typing import Optional


class Event(str, Enum):
    STARTED = "STARTED"
    SUCCEEDED = "SUCCEEDED"
    FAILED = "FAILED"
    INTERRUPTED = "INTERRUPTED"
    STOPPED = "STOPPED"
    HEARTBEAT = "HEARTBEAT"


@dataclass
class RunContext:
    run_id: str
    project: str
    job_name: str
    host: str
    cwd: str
    git_branch: Optional[str]
    git_commit: Optional[str]
    cmd: str
    log_path: Optional[str]
    tmux_session: Optional[str]
    pid: Optional[int]
    start_time: str


@dataclass
class RunResult:
    event: Event
    exit_code: Optional[int]
    end_time: str
    duration_seconds: float
