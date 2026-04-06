from __future__ import annotations

import argparse
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from trainpulse.cli import _resolve_runtime


class CliConfigTests(unittest.TestCase):
    def test_config_priority_cli_over_env_over_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            cfg = Path(tmp) / "config.toml"
            cfg.write_text(
                "\n".join(
                    [
                        "[trainpulse]",
                        "webhook_url = 'https://file.example/hook'",
                        "message_type = 'post'",
                        "heartbeat_minutes = 30",
                    ]
                ),
                encoding="utf-8",
            )
            args = argparse.Namespace(
                config=str(cfg),
                webhook_url=None,
                message_type=None,
                store_path=None,
                error_log_path=None,
                heartbeat_minutes=None,
                dry_run=None,
                redact=None,
            )

            with patch.dict(
                os.environ,
                {
                    "TRAINPULSE_WEBHOOK_URL": "https://env.example/hook",
                    "TRAINPULSE_MESSAGE_TYPE": "text",
                },
                clear=False,
            ):
                runtime = _resolve_runtime(args)
                self.assertEqual(runtime["webhook_url"], "https://env.example/hook")
                self.assertEqual(runtime["message_type"], "text")

                args.webhook_url = "https://cli.example/hook"
                runtime_cli = _resolve_runtime(args)
                self.assertEqual(runtime_cli["webhook_url"], "https://cli.example/hook")
                self.assertEqual(runtime_cli["message_type"], "text")

    def test_heartbeat_defaults_to_30_when_unset(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            cfg = Path(tmp) / "config.toml"
            cfg.write_text("[trainpulse]\nmessage_type = 'text'\n", encoding="utf-8")
            args = argparse.Namespace(
                config=str(cfg),
                webhook_url=None,
                message_type=None,
                store_path=None,
                error_log_path=None,
                heartbeat_minutes=None,
                dry_run=None,
                redact=None,
            )
            with patch.dict(os.environ, {}, clear=True):
                runtime = _resolve_runtime(args)
            self.assertEqual(runtime["heartbeat_minutes"], 30)

    def test_heartbeat_invalid_env_falls_back_to_30(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            cfg = Path(tmp) / "config.toml"
            cfg.write_text("[trainpulse]\n", encoding="utf-8")
            args = argparse.Namespace(
                config=str(cfg),
                webhook_url=None,
                message_type=None,
                store_path=None,
                error_log_path=None,
                heartbeat_minutes=None,
                dry_run=None,
                redact=None,
            )
            with patch.dict(
                os.environ,
                {"TRAINPULSE_HEARTBEAT_MINUTES": "invalid"},
                clear=False,
            ):
                runtime = _resolve_runtime(args)
            self.assertEqual(runtime["heartbeat_minutes"], 30)


if __name__ == "__main__":
    unittest.main()
