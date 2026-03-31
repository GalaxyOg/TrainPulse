from __future__ import annotations

import json
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional


def _iso_now() -> str:
    return datetime.now(timezone.utc).isoformat()


class FeishuNotifier:
    EVENT_STYLE = {
        "STARTED": ("🚀", "Task Started"),
        "SUCCEEDED": ("✅", "Task Succeeded"),
        "FAILED": ("❌", "Task Failed"),
        "INTERRUPTED": ("⛔", "Task Interrupted"),
        "HEARTBEAT": ("💓", "Task Heartbeat"),
    }

    def __init__(
        self,
        webhook_url: Optional[str],
        message_type: str = "text",
        dry_run: bool = False,
        retries: int = 3,
        timeout_seconds: int = 8,
        error_log_path: Optional[str] = None,
    ) -> None:
        self.webhook_url = webhook_url
        self.message_type = message_type
        self.dry_run = dry_run
        self.retries = max(1, retries)
        self.timeout_seconds = timeout_seconds
        self.error_log_path = (
            Path(error_log_path).expanduser()
            if error_log_path
            else Path("~/.local/state/train-notify/notifier_errors.log").expanduser()
        )

    def _event_style(self, event: str) -> tuple[str, str]:
        return self.EVENT_STYLE.get(event, ("🔔", "Task Event"))

    def _write_error(self, message: str) -> None:
        try:
            self.error_log_path.parent.mkdir(parents=True, exist_ok=True)
            with self.error_log_path.open("a", encoding="utf-8") as fp:
                fp.write(f"{_iso_now()} {message}\n")
        except Exception:
            pass

    def _build_text(self, payload: dict) -> str:
        event = str(payload.get("event", "UNKNOWN"))
        emoji, label = self._event_style(event)
        lines = [
            f"{emoji} [{event}] {label} | {payload.get('project', '-')}",
            f"🧩 job: {payload.get('job_name', '-')}",
            f"🆔 run_id: {payload.get('run_id', '-')}",
            f"🖥️ host: {payload.get('host', '-')}",
        ]
        if payload.get("cwd"):
            lines.append(f"📂 cwd: {payload.get('cwd')}")
        if payload.get("git_branch") or payload.get("git_commit"):
            lines.append(
                f"🌿 git: {payload.get('git_branch') or '-'}@{payload.get('git_commit') or '-'}"
            )
        if payload.get("start_time"):
            lines.append(f"🕒 start: {payload.get('start_time')}")
        if payload.get("end_time"):
            lines.append(f"🕓 end: {payload.get('end_time')}")
        if payload.get("exit_code") is not None:
            lines.append(f"📉 exit_code: {payload.get('exit_code')}")
        if payload.get("duration") is not None:
            lines.append(f"⏱️ duration: {payload.get('duration')}s")
        if payload.get("log_path"):
            lines.append(f"📝 log: {payload.get('log_path')}")
        if payload.get("cmd"):
            lines.append(f"💻 cmd: {payload.get('cmd')}")
        return "\n".join(lines)

    def build_message(self, payload: dict) -> dict:
        event = str(payload.get("event", "UNKNOWN"))
        emoji, label = self._event_style(event)
        if self.message_type == "post":
            lines = [
                [{"tag": "text", "text": f"{emoji} event: {event}"}],
                [{"tag": "text", "text": f"📦 project: {payload.get('project', '-')}"}],
                [{"tag": "text", "text": f"🧩 job: {payload.get('job_name', '-')}"}],
                [{"tag": "text", "text": f"🆔 run_id: {payload.get('run_id', '-')}"}],
                [{"tag": "text", "text": f"🖥️ host: {payload.get('host', '-')}"}],
                [{"tag": "text", "text": f"📂 cwd: {payload.get('cwd', '-')}"}],
            ]
            if payload.get("git_branch") or payload.get("git_commit"):
                lines.append(
                    [
                        {
                            "tag": "text",
                            "text": f"🌿 git: {payload.get('git_branch') or '-'}@{payload.get('git_commit') or '-'}",
                        }
                    ]
                )
            if payload.get("start_time"):
                lines.append([{"tag": "text", "text": f"🕒 start: {payload.get('start_time')}"}])
            if payload.get("end_time"):
                lines.append([{"tag": "text", "text": f"🕓 end: {payload.get('end_time')}"}])
            if payload.get("duration") is not None:
                lines.append([{"tag": "text", "text": f"⏱️ duration: {payload.get('duration')}s"}])
            if payload.get("exit_code") is not None:
                lines.append([{"tag": "text", "text": f"📉 exit_code: {payload.get('exit_code')}"}])
            if payload.get("log_path"):
                lines.append([{"tag": "text", "text": f"📝 log: {payload.get('log_path')}"}])
            if payload.get("cmd"):
                lines.append([{"tag": "text", "text": f"💻 cmd: {payload.get('cmd')}"}])
            return {
                "msg_type": "post",
                "content": {
                    "post": {
                        "zh_cn": {
                            "title": f"{emoji} {label} · {payload.get('project', '-')}",
                            "content": lines,
                        }
                    }
                },
            }

        return {
            "msg_type": "text",
            "content": {"text": self._build_text(payload)},
        }

    def send(self, payload: dict) -> bool:
        body = self.build_message(payload)
        event = str(payload.get("event", "UNKNOWN"))
        if self.dry_run:
            summary = self._build_text(payload)
            print(f"[train-notify][dry-run][{event}] {summary}", file=sys.stderr)
            print(
                f"[train-notify][dry-run][payload] {json.dumps(body, ensure_ascii=False)}",
                file=sys.stderr,
            )
            return True

        if not self.webhook_url:
            message = f"[train-notify][notify][{event}] webhook_url is empty, skip notification"
            print(message, file=sys.stderr)
            self._write_error(message)
            return False

        request = urllib.request.Request(
            self.webhook_url,
            data=json.dumps(body).encode("utf-8"),
            headers={"Content-Type": "application/json"},
            method="POST",
        )

        delay = 1.0
        for attempt in range(1, self.retries + 1):
            try:
                with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                    payload_body = response.read().decode("utf-8", errors="replace")
                    if response.status < 200 or response.status >= 300:
                        raise RuntimeError(f"status={response.status}, body={payload_body}")
                    try:
                        parsed = json.loads(payload_body or "{}")
                    except json.JSONDecodeError:
                        parsed = {}
                    if isinstance(parsed, dict) and parsed.get("code", 0) not in (0, "0"):
                        raise RuntimeError(f"feishu code={parsed.get('code')} msg={parsed.get('msg')}")
                    return True
            except (urllib.error.URLError, RuntimeError, TimeoutError) as exc:
                self._write_error(f"attempt={attempt} send failed: {exc}")
                if attempt < self.retries:
                    time.sleep(delay)
                    delay = min(delay * 2, 8.0)
        print(
            f"[train-notify][notify][{event}] delivery failed after {self.retries} attempt(s)",
            file=sys.stderr,
        )
        return False
