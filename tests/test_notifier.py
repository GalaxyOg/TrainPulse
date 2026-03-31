from __future__ import annotations

import unittest
from contextlib import redirect_stderr
from io import StringIO

from train_notify.core.notifier import FeishuNotifier


def _payload() -> dict:
    return {
        "event": "FAILED",
        "project": "demo",
        "job_name": "train",
        "run_id": "rid-1",
        "host": "host-1",
        "cwd": "/tmp/demo",
        "cmd": "python train.py",
        "exit_code": 2,
        "duration": 1.23,
        "log_path": "/tmp/train.log",
        "git_branch": "main",
        "git_commit": "abc123",
    }


class NotifierTests(unittest.TestCase):
    def test_text_message_format(self) -> None:
        notifier = FeishuNotifier(webhook_url=None, message_type="text", dry_run=True)
        message = notifier.build_message(_payload())
        self.assertEqual(message["msg_type"], "text")
        text = message["content"]["text"]
        self.assertIn("[FAILED] demo", text)
        self.assertIn("job=train", text)
        self.assertIn("exit=2", text)

    def test_post_message_format(self) -> None:
        notifier = FeishuNotifier(webhook_url=None, message_type="post", dry_run=True)
        message = notifier.build_message(_payload())
        self.assertEqual(message["msg_type"], "post")
        title = message["content"]["post"]["zh_cn"]["title"]
        self.assertIn("[FAILED] demo", title)

    def test_dry_run_send_prints_failed_event(self) -> None:
        notifier = FeishuNotifier(webhook_url=None, message_type="text", dry_run=True)
        buffer = StringIO()
        with redirect_stderr(buffer):
            ok = notifier.send(_payload())
        self.assertTrue(ok)
        output = buffer.getvalue()
        self.assertIn("[train-notify][dry-run][FAILED]", output)
        self.assertIn("[FAILED] demo", output)

    def test_send_without_webhook_prints_visible_warning(self) -> None:
        payload = dict(_payload(), event="FAILED")
        notifier = FeishuNotifier(webhook_url=None, message_type="text", dry_run=False)
        buffer = StringIO()
        with redirect_stderr(buffer):
            ok = notifier.send(payload)
        self.assertFalse(ok)
        output = buffer.getvalue()
        self.assertIn("webhook_url is empty", output)
        self.assertIn("[FAILED]", output)


if __name__ == "__main__":
    unittest.main()
