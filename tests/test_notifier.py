from __future__ import annotations

import unittest

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


if __name__ == "__main__":
    unittest.main()
