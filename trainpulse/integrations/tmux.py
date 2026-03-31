from __future__ import annotations

import shlex
import shutil
import subprocess
import time
from typing import Optional


def has_tmux() -> bool:
    return shutil.which("tmux") is not None


def session_exists(session: str) -> bool:
    proc = subprocess.run(
        ["tmux", "has-session", "-t", session],
        check=False,
        capture_output=True,
        text=True,
    )
    return proc.returncode == 0


def start_detached_session(session: str, command: str, cwd: Optional[str] = None) -> None:
    shell_cmd = command
    if cwd:
        shell_cmd = f"cd {shlex.quote(cwd)} && {command}"
    proc = subprocess.run(
        ["tmux", "new-session", "-d", "-s", session, shell_cmd],
        check=False,
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise RuntimeError(proc.stderr.strip() or "failed to start tmux session")


def send_ctrl_c(session: str) -> bool:
    proc = subprocess.run(
        ["tmux", "send-keys", "-t", session, "C-c"],
        check=False,
        capture_output=True,
        text=True,
    )
    return proc.returncode == 0


def stop_session(session: str, grace_seconds: float = 3.0) -> None:
    send_ctrl_c(session)
    start = time.monotonic()
    while time.monotonic() - start <= grace_seconds:
        if not session_exists(session):
            return
        time.sleep(0.2)
    subprocess.run(["tmux", "kill-session", "-t", session], check=False)
