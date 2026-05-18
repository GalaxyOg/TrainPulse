from __future__ import annotations

import argparse
import io
import unittest
from contextlib import redirect_stdout
from unittest.mock import patch

from trainpulse.cli import cmd_tmux_run


def _tmux_args(session: str | None = None) -> argparse.Namespace:
    return argparse.Namespace(
        config="~/.config/trainpulse/config.toml",
        session=session,
        job_name=None,
        log_path=None,
        cwd="/tmp",
        cmd=["--", "python", "-c", "print('ok')"],
    )


def _runtime() -> dict[str, object]:
    return {
        "webhook_url": None,
        "message_type": "text",
        "store_path": "/tmp/runs.db",
        "error_log_path": "/tmp/notifier_errors.log",
        "heartbeat_minutes": 30,
        "dry_run": False,
        "redact": [],
    }


class CliTmuxRunTests(unittest.TestCase):
    def test_tmux_run_auto_generates_session_when_missing(self) -> None:
        args = _tmux_args(session=None)
        output = io.StringIO()
        with patch("trainpulse.cli._resolve_runtime", return_value=_runtime()):
            with patch("trainpulse.cli._generate_run_id", return_value="20260518-aabbccdd"):
                with patch("trainpulse.cli.has_tmux", return_value=True):
                    with patch("trainpulse.cli.session_exists", return_value=False):
                        with patch("trainpulse.cli.start_detached_session") as start_mock:
                            with redirect_stdout(output):
                                rc = cmd_tmux_run(args)
        self.assertEqual(rc, 0)
        start_mock.assert_called_once()
        session_name, wrapped_command = start_mock.call_args.args[:2]
        self.assertEqual(session_name, "trainpulse-20260518-aabbccdd")
        self.assertIn("TRAINPULSE_TMUX_SESSION=trainpulse-20260518-aabbccdd", wrapped_command)
        self.assertIn("tp_exit=$?", wrapped_command)
        self.assertIn('exec "${SHELL:-/bin/bash}"', wrapped_command)
        self.assertIn("attach: tmux attach -t trainpulse-20260518-aabbccdd", output.getvalue())

    def test_tmux_run_keeps_explicit_session(self) -> None:
        args = _tmux_args(session="exp1")
        with patch("trainpulse.cli._resolve_runtime", return_value=_runtime()):
            with patch("trainpulse.cli._generate_run_id", return_value="20260518-aabbccdd"):
                with patch("trainpulse.cli.has_tmux", return_value=True):
                    with patch("trainpulse.cli.session_exists", return_value=False):
                        with patch("trainpulse.cli.start_detached_session") as start_mock:
                            rc = cmd_tmux_run(args)
        self.assertEqual(rc, 0)
        session_name = start_mock.call_args.args[0]
        self.assertEqual(session_name, "exp1")


if __name__ == "__main__":
    unittest.main()
