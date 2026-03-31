from __future__ import annotations

import os
import signal
import subprocess
import sys
import threading
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional, TextIO

from .models import Event
from .notifier import FeishuNotifier
from .store import RunStore


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def determine_final_event(exit_code: int, interrupted: bool) -> Event:
    if interrupted:
        return Event.INTERRUPTED
    if exit_code == 0:
        return Event.SUCCEEDED
    return Event.FAILED


def normalize_exit_code(exit_code: int) -> int:
    if exit_code < 0:
        return 128 + abs(exit_code)
    return exit_code


def _stream_pump(src: TextIO, dst: TextIO, log_fp: Optional[TextIO]) -> None:
    try:
        for line in iter(src.readline, ""):
            dst.write(line)
            dst.flush()
            if log_fp:
                log_fp.write(line)
                log_fp.flush()
    finally:
        src.close()


class CommandRunner:
    def __init__(
        self,
        notifier: FeishuNotifier,
        store: RunStore,
        heartbeat_minutes: Optional[int] = None,
    ) -> None:
        self.notifier = notifier
        self.store = store
        self.heartbeat_seconds = None
        if heartbeat_minutes and heartbeat_minutes > 0:
            self.heartbeat_seconds = int(heartbeat_minutes * 60)

    def run(self, command: list[str], context: dict) -> int:
        started = time.monotonic()
        interrupted = False
        child_proc: Optional[subprocess.Popen[str]] = None
        log_fp: Optional[TextIO] = None
        previous_handlers: dict[int, signal.Handlers] = {}
        heartbeat_deadline = (
            started + self.heartbeat_seconds if self.heartbeat_seconds else None
        )

        def _forward_signal(signum: int, _frame: object) -> None:
            nonlocal interrupted
            interrupted = True
            if child_proc and child_proc.poll() is None:
                try:
                    if hasattr(os, "killpg"):
                        os.killpg(child_proc.pid, signum)
                    else:
                        child_proc.send_signal(signum)
                except ProcessLookupError:
                    pass

        try:
            if context.get("log_path"):
                log_path = Path(context["log_path"])
                log_path.parent.mkdir(parents=True, exist_ok=True)
                log_fp = log_path.open("a", encoding="utf-8")

            popen_kwargs = {
                "stdout": subprocess.PIPE,
                "stderr": subprocess.PIPE,
                "text": True,
                "bufsize": 1,
            }
            if hasattr(os, "setsid"):
                popen_kwargs["preexec_fn"] = os.setsid
            child_proc = subprocess.Popen(command, **popen_kwargs)
            context["pid"] = child_proc.pid

            self.store.start_run(context)
            self.notifier.send(dict(context, event=Event.STARTED.value))

            for signum in (signal.SIGINT, signal.SIGTERM):
                previous_handlers[signum] = signal.getsignal(signum)
                signal.signal(signum, _forward_signal)

            stdout_thread = threading.Thread(
                target=_stream_pump,
                args=(child_proc.stdout, sys.stdout, log_fp),
                daemon=True,
            )
            stderr_thread = threading.Thread(
                target=_stream_pump,
                args=(child_proc.stderr, sys.stderr, log_fp),
                daemon=True,
            )
            stdout_thread.start()
            stderr_thread.start()

            exit_code = 1
            while True:
                rc = child_proc.poll()
                if rc is not None:
                    exit_code = normalize_exit_code(rc)
                    break
                if heartbeat_deadline is not None and time.monotonic() >= heartbeat_deadline:
                    self.store.heartbeat(context["run_id"])
                    self.notifier.send(dict(context, event=Event.HEARTBEAT.value))
                    heartbeat_deadline = time.monotonic() + self.heartbeat_seconds
                time.sleep(0.2)

            stdout_thread.join(timeout=2)
            stderr_thread.join(timeout=2)

            final_event = determine_final_event(exit_code, interrupted)
            end_time = now_iso()
            duration = round(time.monotonic() - started, 3)
            self.store.finish_run(
                run_id=context["run_id"],
                event=final_event.value,
                exit_code=exit_code,
                end_time=end_time,
                duration=duration,
            )
            payload = dict(
                context,
                event=final_event.value,
                end_time=end_time,
                duration=duration,
                exit_code=exit_code,
            )
            self.notifier.send(payload)
            return exit_code
        except FileNotFoundError:
            end_time = now_iso()
            duration = round(time.monotonic() - started, 3)
            self.store.start_run(context)
            self.store.finish_run(context["run_id"], Event.FAILED.value, 127, end_time, duration)
            self.notifier.send(
                dict(
                    context,
                    event=Event.FAILED.value,
                    end_time=end_time,
                    duration=duration,
                    exit_code=127,
                )
            )
            return 127
        finally:
            for signum, handler in previous_handlers.items():
                signal.signal(signum, handler)
            if log_fp:
                log_fp.close()
