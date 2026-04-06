from __future__ import annotations

import argparse
import os
import signal
import unittest
from unittest.mock import patch

from trainpulse.cli import cmd_status, cmd_stop


def _base_run(
    run_id: str = "run-1",
    status: str = "RUNNING",
    event: str = "HEARTBEAT",
    pid: int | None = 12345,
    tmux_session: str | None = None,
) -> dict:
    return {
        "run_id": run_id,
        "event": event,
        "status": status,
        "project": "demo",
        "job_name": "train",
        "cmd": "python train.py",
        "host": "node-a",
        "cwd": "/tmp/demo",
        "git_branch": "main",
        "git_commit": "abc1234",
        "log_path": None,
        "start_time": "2026-04-06T19:00:00+08:00",
        "end_time": None,
        "duration": None,
        "exit_code": None,
        "pid": pid,
        "tmux_session": tmux_session,
        "last_heartbeat": "2026-04-06T19:05:00+08:00",
        "updated_at": "2026-04-06T19:05:00+08:00",
    }


def _stop_args(run_id: str) -> argparse.Namespace:
    return argparse.Namespace(
        config="/tmp/nonexistent-config.toml",
        webhook_url="https://example/hook",
        message_type="text",
        store_path="/tmp/nonexistent-runs.db",
        error_log_path=None,
        dry_run=None,
        run_id=run_id,
    )


def _status_args() -> argparse.Namespace:
    return argparse.Namespace(
        config="/tmp/nonexistent-config.toml",
        webhook_url="https://example/hook",
        message_type="text",
        store_path="/tmp/nonexistent-runs.db",
        error_log_path=None,
        dry_run=None,
        limit=20,
        running_only=False,
        reconcile=True,
        reconcile_stale_minutes=None,
    )


class _FakeNotifier:
    def __init__(self) -> None:
        self.payloads: list[dict] = []

    def send(self, payload: dict) -> bool:
        self.payloads.append(payload)
        return True


class _FakeStore:
    def __init__(self, runs: list[dict]) -> None:
        self.runs = {str(row["run_id"]): dict(row) for row in runs}

    def get_run(self, run_id: str) -> dict | None:
        row = self.runs.get(str(run_id))
        return dict(row) if row else None

    def finish_run(
        self,
        run_id: str,
        event: str,
        exit_code: int | None,
        end_time: str,
        duration: float,
    ) -> bool:
        row = self.runs.get(str(run_id))
        if not row or row.get("status") != "RUNNING":
            return False
        row["event"] = event
        row["status"] = "SUCCEEDED" if event == "SUCCEEDED" else event
        row["exit_code"] = exit_code
        row["end_time"] = end_time
        row["duration"] = duration
        row["updated_at"] = end_time
        return True

    def list_runs(self, limit: int | None = 20, running_only: bool = False) -> list[dict]:
        rows = [dict(row) for row in self.runs.values()]
        rows.sort(key=lambda item: str(item.get("updated_at", "")), reverse=True)
        if running_only:
            rows = [row for row in rows if row.get("status") == "RUNNING"]
        if limit is not None:
            rows = rows[:limit]
        return rows


class CliStopReconcileTests(unittest.TestCase):
    def test_stop_active_run_updates_terminal_and_notifies(self) -> None:
        store = _FakeStore([_base_run()])
        notifier = _FakeNotifier()
        args = _stop_args("run-1")
        with patch.dict(os.environ, {}, clear=True):
            with patch("trainpulse.cli.RunStore", return_value=store):
                with patch("trainpulse.cli.FeishuNotifier", return_value=notifier):
                    with patch("trainpulse.cli.os.kill") as kill_mock:
                        rc = cmd_stop(args)
        self.assertEqual(rc, 0)
        self.assertEqual(store.runs["run-1"]["status"], "STOPPED")
        self.assertEqual(store.runs["run-1"]["exit_code"], 143)
        self.assertEqual(len(notifier.payloads), 1)
        self.assertEqual(notifier.payloads[0]["event"], "STOPPED")
        kill_mock.assert_called_once_with(12345, signal.SIGTERM)

    def test_stop_dead_process_still_converges_and_notifies(self) -> None:
        store = _FakeStore([_base_run()])
        notifier = _FakeNotifier()
        args = _stop_args("run-1")
        with patch.dict(os.environ, {}, clear=True):
            with patch("trainpulse.cli.RunStore", return_value=store):
                with patch("trainpulse.cli.FeishuNotifier", return_value=notifier):
                    with patch("trainpulse.cli.os.kill", side_effect=ProcessLookupError):
                        rc = cmd_stop(args)
        self.assertEqual(rc, 0)
        self.assertEqual(store.runs["run-1"]["status"], "STOPPED")
        self.assertEqual(len(notifier.payloads), 1)
        self.assertEqual(notifier.payloads[0]["event"], "STOPPED")

    def test_stop_terminal_run_is_idempotent(self) -> None:
        store = _FakeStore([_base_run(status="FAILED", event="FAILED", pid=None)])
        notifier = _FakeNotifier()
        args = _stop_args("run-1")
        with patch.dict(os.environ, {}, clear=True):
            with patch("trainpulse.cli.RunStore", return_value=store):
                with patch("trainpulse.cli.FeishuNotifier", return_value=notifier):
                    rc = cmd_stop(args)
        self.assertEqual(rc, 0)
        self.assertEqual(store.runs["run-1"]["status"], "FAILED")
        self.assertEqual(len(notifier.payloads), 0)

    def test_status_reconcile_converges_zombie_running_and_is_idempotent(self) -> None:
        store = _FakeStore([_base_run(pid=45678, tmux_session=None)])
        notifier = _FakeNotifier()
        args = _status_args()
        with patch.dict(os.environ, {}, clear=True):
            with patch("trainpulse.cli.RunStore", return_value=store):
                with patch("trainpulse.cli.FeishuNotifier", return_value=notifier):
                    with patch("trainpulse.cli.os.kill", side_effect=ProcessLookupError):
                        rc_first = cmd_status(args)
                        rc_second = cmd_status(args)
        self.assertEqual(rc_first, 0)
        self.assertEqual(rc_second, 0)
        self.assertEqual(store.runs["run-1"]["status"], "STOPPED")
        self.assertEqual(len(notifier.payloads), 1)
        self.assertEqual(notifier.payloads[0]["event"], "STOPPED")


if __name__ == "__main__":
    unittest.main()
