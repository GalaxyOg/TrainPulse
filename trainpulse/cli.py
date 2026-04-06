from __future__ import annotations

import argparse
import ast
import os
import shlex
import signal
import sys
import uuid
from datetime import datetime
from pathlib import Path
from typing import Any, Optional

from trainpulse.core.context import build_run_context, infer_job_name
from trainpulse.core.models import Event
from trainpulse.core.notifier import FeishuNotifier
from trainpulse.core.runner import CommandRunner
from trainpulse.core.store import RunStore
from trainpulse.core.timeutil import now_compact, now_iso
from trainpulse.integrations.tmux import (
    has_tmux,
    session_exists,
    start_detached_session,
    stop_session,
)

try:
    import tomllib  # type: ignore[attr-defined]
except ModuleNotFoundError:  # pragma: no cover
    tomllib = None  # type: ignore[assignment]


DEFAULT_CONFIG_PATH = "~/.config/trainpulse/config.toml"
DEFAULT_STORE_PATH = "~/.local/state/trainpulse/runs.db"
DEFAULT_ERROR_LOG_PATH = "~/.local/state/trainpulse/notifier_errors.log"
DEFAULT_HEARTBEAT_MINUTES = 30
STOP_EXIT_CODE = 143
TERMINAL_STATUSES = {
    Event.SUCCEEDED.value,
    Event.FAILED.value,
    Event.INTERRUPTED.value,
    Event.STOPPED.value,
}


def _str_to_bool(value: Optional[str]) -> Optional[bool]:
    if value is None:
        return None
    lowered = value.strip().lower()
    if lowered in {"1", "true", "yes", "on"}:
        return True
    if lowered in {"0", "false", "no", "off"}:
        return False
    return None


def _load_config_file(path: str) -> dict[str, Any]:
    file_path = Path(path).expanduser()
    if not file_path.exists():
        return {}
    content = file_path.read_text(encoding="utf-8")
    if tomllib is None:
        data = _parse_basic_toml(content)
        if isinstance(data.get("trainpulse"), dict):
            return data["trainpulse"]
        return data
    try:
        data = tomllib.loads(content)
    except Exception:
        return {}
    if not isinstance(data, dict):
        return {}
    if isinstance(data.get("trainpulse"), dict):
        return data["trainpulse"]
    return data


def _parse_toml_value(raw: str) -> Any:
    value = raw.strip()
    if value.startswith('"') and value.endswith('"'):
        return value[1:-1]
    if value.startswith("'") and value.endswith("'"):
        return value[1:-1]
    lowered = value.lower()
    if lowered == "true":
        return True
    if lowered == "false":
        return False
    if value.startswith("[") and value.endswith("]"):
        inner = value[1:-1].strip()
        if not inner:
            return []
        parts = [part.strip() for part in inner.split(",")]
        return [_parse_toml_value(part) for part in parts]
    if value and (value.isdigit() or (value.startswith("-") and value[1:].isdigit())):
        return int(value)
    try:
        return ast.literal_eval(value)
    except Exception:
        return value


def _parse_basic_toml(content: str) -> dict[str, Any]:
    result: dict[str, Any] = {}
    section: Optional[str] = None
    for raw_line in content.splitlines():
        line = raw_line.split("#", 1)[0].strip()
        if not line:
            continue
        if line.startswith("[") and line.endswith("]"):
            section = line[1:-1].strip()
            if section and section not in result:
                result[section] = {}
            continue
        if "=" not in line:
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        parsed = _parse_toml_value(value.strip())
        if section:
            target = result.setdefault(section, {})
            if isinstance(target, dict):
                target[key] = parsed
        else:
            result[key] = parsed
    return result


def _resolve_runtime(args: argparse.Namespace) -> dict[str, Any]:
    file_cfg = _load_config_file(getattr(args, "config", DEFAULT_CONFIG_PATH))
    env = os.environ

    def pick(cli_value: Any, env_name: str, cfg_name: str, default: Any) -> Any:
        if cli_value is not None:
            return cli_value
        if env_name in env and env[env_name] != "":
            return env[env_name]
        if cfg_name in file_cfg:
            return file_cfg[cfg_name]
        return default

    dry_run_cli = getattr(args, "dry_run", None)
    dry_run_env = _str_to_bool(env.get("TRAINPULSE_DRY_RUN"))
    dry_run_cfg = file_cfg.get("dry_run")
    if dry_run_cli is not None:
        dry_run = dry_run_cli
    elif dry_run_env is not None:
        dry_run = dry_run_env
    elif isinstance(dry_run_cfg, bool):
        dry_run = dry_run_cfg
    else:
        dry_run = False

    heartbeat_cli = getattr(args, "heartbeat_minutes", None)
    heartbeat_env = env.get("TRAINPULSE_HEARTBEAT_MINUTES")
    heartbeat_cfg = file_cfg.get("heartbeat_minutes")
    heartbeat_minutes = heartbeat_cli
    if heartbeat_minutes is None and heartbeat_env:
        try:
            heartbeat_minutes = int(heartbeat_env)
        except ValueError:
            heartbeat_minutes = None
    if heartbeat_minutes is None and isinstance(heartbeat_cfg, int):
        heartbeat_minutes = heartbeat_cfg
    if not isinstance(heartbeat_minutes, int) or heartbeat_minutes <= 0:
        heartbeat_minutes = DEFAULT_HEARTBEAT_MINUTES

    redact_cli = getattr(args, "redact", None)
    redact_env = env.get("TRAINPULSE_REDACT")
    redact_cfg = file_cfg.get("redact")
    if redact_cli is not None:
        redact = redact_cli
    elif redact_env:
        redact = [x.strip() for x in redact_env.split(",") if x.strip()]
    elif isinstance(redact_cfg, list):
        redact = [str(x) for x in redact_cfg]
    else:
        redact = []

    message_type = str(
        pick(
            getattr(args, "message_type", None),
            "TRAINPULSE_MESSAGE_TYPE",
            "message_type",
            "text",
        )
    ).lower()
    if message_type not in {"text", "post"}:
        message_type = "text"

    return {
        "webhook_url": pick(
            getattr(args, "webhook_url", None),
            "TRAINPULSE_WEBHOOK_URL",
            "webhook_url",
            None,
        ),
        "message_type": message_type,
        "store_path": str(
            Path(
                pick(
                    getattr(args, "store_path", None),
                    "TRAINPULSE_STORE_PATH",
                    "store_path",
                    DEFAULT_STORE_PATH,
                )
            ).expanduser()
        ),
        "error_log_path": str(
            Path(
                pick(
                    getattr(args, "error_log_path", None),
                    "TRAINPULSE_ERROR_LOG_PATH",
                    "error_log_path",
                    DEFAULT_ERROR_LOG_PATH,
                )
            ).expanduser()
        ),
        "heartbeat_minutes": heartbeat_minutes,
        "dry_run": dry_run,
        "redact": redact,
    }


def _normalize_cmd(cmd: list[str]) -> list[str]:
    if cmd and cmd[0] == "--":
        return cmd[1:]
    return cmd


def _generate_run_id() -> str:
    ts = now_compact()
    suffix = uuid.uuid4().hex[:8]
    return f"{ts}-{suffix}"


def _parse_iso(value: Any) -> Optional[datetime]:
    if not isinstance(value, str) or not value:
        return None
    try:
        return datetime.fromisoformat(value)
    except ValueError:
        return None


def _duration_seconds(start_time: Any, end_time: str) -> float:
    start_dt = _parse_iso(start_time)
    end_dt = _parse_iso(end_time)
    if not start_dt or not end_dt:
        return 0.0
    seconds = (end_dt - start_dt).total_seconds()
    if seconds < 0:
        return 0.0
    return round(seconds, 3)


def _build_terminal_payload(run: dict[str, Any], event: Event, end_time: str, duration: float) -> dict:
    return {
        "run_id": run.get("run_id"),
        "project": run.get("project"),
        "job_name": run.get("job_name"),
        "host": run.get("host"),
        "cwd": run.get("cwd"),
        "git_branch": run.get("git_branch"),
        "git_commit": run.get("git_commit"),
        "cmd": run.get("cmd"),
        "log_path": run.get("log_path"),
        "start_time": run.get("start_time"),
        "end_time": end_time,
        "duration": duration,
        "exit_code": STOP_EXIT_CODE,
        "event": event.value,
    }


def _build_notifier(runtime: dict[str, Any]) -> Optional[FeishuNotifier]:
    if not runtime.get("webhook_url") and not runtime.get("dry_run"):
        return None
    return FeishuNotifier(
        webhook_url=runtime["webhook_url"],
        message_type=runtime["message_type"],
        dry_run=runtime["dry_run"],
        error_log_path=runtime["error_log_path"],
    )


def _pid_exists(pid: Any) -> bool:
    if pid is None:
        return False
    try:
        os.kill(int(pid), 0)
        return True
    except (TypeError, ValueError, ProcessLookupError):
        return False
    except PermissionError:
        return True


def _session_reachable(session: Any) -> bool:
    if not isinstance(session, str) or not session:
        return False
    if not has_tmux():
        return False
    try:
        return session_exists(session)
    except Exception:
        return False


def _is_orphaned_running_run(run: dict[str, Any]) -> bool:
    pid_missing = not _pid_exists(run.get("pid"))
    session_missing = not _session_reachable(run.get("tmux_session"))
    return pid_missing and session_missing


def _is_reconcile_timeout_reached(run: dict[str, Any], stale_minutes: Optional[int], now_time: str) -> bool:
    if stale_minutes is None or stale_minutes <= 0:
        return True
    threshold_seconds = stale_minutes * 60
    last_touch = (
        run.get("last_heartbeat")
        or run.get("updated_at")
        or run.get("start_time")
    )
    last_touch_dt = _parse_iso(last_touch)
    now_dt = _parse_iso(now_time)
    if not last_touch_dt or not now_dt:
        return True
    return (now_dt - last_touch_dt).total_seconds() >= threshold_seconds


def _finalize_run_stopped(
    store: RunStore,
    run: dict[str, Any],
    notifier: Optional[FeishuNotifier],
) -> bool:
    end_time = now_iso()
    duration = _duration_seconds(run.get("start_time"), end_time)
    updated = store.finish_run(
        run_id=str(run["run_id"]),
        event=Event.STOPPED.value,
        exit_code=STOP_EXIT_CODE,
        end_time=end_time,
        duration=duration,
    )
    if updated and notifier:
        notifier.send(_build_terminal_payload(run, Event.STOPPED, end_time, duration))
    return updated


def cmd_run(args: argparse.Namespace) -> int:
    runtime = _resolve_runtime(args)
    command = _normalize_cmd(args.cmd)
    if not command:
        print("error: missing command, use: trainpulse run -- <command...>", file=sys.stderr)
        return 2

    run_id = args.run_id or _generate_run_id()
    cwd = os.getcwd()
    job_name = args.job_name or infer_job_name(command)
    context = build_run_context(
        run_id=run_id,
        job_name=job_name,
        cmd=command,
        cwd=cwd,
        log_path=args.log_path,
        redact_patterns=runtime["redact"],
    )
    notifier = FeishuNotifier(
        webhook_url=runtime["webhook_url"],
        message_type=runtime["message_type"],
        dry_run=runtime["dry_run"],
        error_log_path=runtime["error_log_path"],
    )
    store = RunStore(runtime["store_path"])
    runner = CommandRunner(
        notifier=notifier,
        store=store,
        heartbeat_minutes=runtime["heartbeat_minutes"],
    )
    return runner.run(command, context)


def cmd_tmux_run(args: argparse.Namespace) -> int:
    runtime = _resolve_runtime(args)
    command = _normalize_cmd(args.cmd)
    if not command:
        print("error: missing command, use: trainpulse tmux-run --session s -- <command...>", file=sys.stderr)
        return 2
    if not has_tmux():
        print("error: tmux is not installed; use `trainpulse run` instead.", file=sys.stderr)
        return 2
    if session_exists(args.session):
        print(f"error: tmux session already exists: {args.session}", file=sys.stderr)
        return 2

    run_id = _generate_run_id()
    inner = [
        sys.executable,
        "-m",
        "trainpulse.cli",
        "run",
        "--run-id",
        run_id,
        "--config",
        args.config,
        "--store-path",
        runtime["store_path"],
        "--message-type",
        runtime["message_type"],
        "--error-log-path",
        runtime["error_log_path"],
    ]

    if args.job_name:
        inner += ["--job-name", args.job_name]
    if args.log_path:
        inner += ["--log-path", args.log_path]
    if runtime["webhook_url"]:
        inner += ["--webhook-url", runtime["webhook_url"]]
    if runtime["heartbeat_minutes"] is not None:
        inner += ["--heartbeat-minutes", str(runtime["heartbeat_minutes"])]
    inner += ["--dry-run"] if runtime["dry_run"] else ["--no-dry-run"]
    for pattern in runtime["redact"]:
        inner += ["--redact", pattern]
    inner += ["--", *command]

    command_str = shlex.join(inner)
    wrapped_command = f"TRAINPULSE_TMUX_SESSION={shlex.quote(args.session)} {command_str}"
    start_detached_session(args.session, wrapped_command, cwd=args.cwd)
    print(f"tmux task started: run_id={run_id} session={args.session}")
    return 0


def cmd_status(args: argparse.Namespace) -> int:
    runtime = _resolve_runtime(args)
    store = RunStore(runtime["store_path"])
    notifier = _build_notifier(runtime)

    if args.reconcile:
        running_rows = store.list_runs(limit=None, running_only=True)
        reconciled = 0
        now_time = now_iso()
        for run in running_rows:
            if run.get("status") != "RUNNING":
                continue
            if not _is_orphaned_running_run(run):
                continue
            if not _is_reconcile_timeout_reached(run, args.reconcile_stale_minutes, now_time):
                continue
            if _finalize_run_stopped(store, run, notifier):
                reconciled += 1
        if reconciled:
            print(f"reconciled {reconciled} orphaned RUNNING run(s)")

    rows = store.list_runs(limit=args.limit, running_only=args.running_only)
    if not rows:
        print("no runs found")
        return 0
    print("run_id | status | project | job_name | exit_code | updated_at | tmux_session")
    for row in rows:
        print(
            f"{row['run_id']} | {row['status']} | {row['project']} | {row['job_name']} | "
            f"{row['exit_code']} | {row['updated_at']} | {row['tmux_session'] or '-'}"
        )
    return 0


def cmd_stop(args: argparse.Namespace) -> int:
    runtime = _resolve_runtime(args)
    store = RunStore(runtime["store_path"])
    run = store.get_run(args.run_id)
    if not run:
        print(f"error: run not found: {args.run_id}", file=sys.stderr)
        return 1
    if run.get("status") in TERMINAL_STATUSES:
        print(f"run already in terminal state: {args.run_id} ({run.get('status')})")
        return 0

    stop_signal_sent = False
    session = run.get("tmux_session")
    if session and _session_reachable(session):
        stop_session(session)
        stop_signal_sent = True

    pid = run.get("pid")
    if pid:
        try:
            os.kill(int(pid), signal.SIGTERM)
            stop_signal_sent = True
        except ProcessLookupError:
            pass
        except (PermissionError, ValueError, TypeError):
            pass

    notifier = _build_notifier(runtime)
    updated = _finalize_run_stopped(store, run, notifier)
    if updated:
        if stop_signal_sent:
            print(f"stop signal sent and run finalized: {args.run_id}")
        else:
            print(f"run finalized as STOPPED (target already exited): {args.run_id}")
        return 0

    print(f"warning: run already finished: {args.run_id}")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="trainpulse")
    sub = parser.add_subparsers(dest="subcmd", required=True)

    def add_runtime_flags(p: argparse.ArgumentParser) -> None:
        p.add_argument("--config", default=DEFAULT_CONFIG_PATH)
        p.add_argument("--webhook-url", default=None)
        p.add_argument("--message-type", choices=["text", "post"], default=None)
        p.add_argument("--store-path", default=None)
        p.add_argument("--error-log-path", default=None)
        p.add_argument(
            "--heartbeat-minutes",
            type=int,
            default=None,
            help="Silent liveness check interval in minutes (default: 30)",
        )
        p.add_argument("--dry-run", action=argparse.BooleanOptionalAction, default=None)
        p.add_argument("--redact", action="append", default=None)

    def add_notify_flags(p: argparse.ArgumentParser) -> None:
        p.add_argument("--config", default=DEFAULT_CONFIG_PATH)
        p.add_argument("--webhook-url", default=None)
        p.add_argument("--message-type", choices=["text", "post"], default=None)
        p.add_argument("--store-path", default=None)
        p.add_argument("--error-log-path", default=None)
        p.add_argument("--dry-run", action=argparse.BooleanOptionalAction, default=None)

    run_p = sub.add_parser("run", help="Run a command with abnormal-exit alerts")
    add_runtime_flags(run_p)
    run_p.add_argument("--job-name", default=None)
    run_p.add_argument("--log-path", default=None)
    run_p.add_argument("--run-id", default=None, help=argparse.SUPPRESS)
    run_p.add_argument("cmd", nargs=argparse.REMAINDER)
    run_p.set_defaults(func=cmd_run)

    tmux_p = sub.add_parser("tmux-run", help="Run command in detached tmux session with alerts")
    add_runtime_flags(tmux_p)
    tmux_p.add_argument("--session", required=True)
    tmux_p.add_argument("--job-name", default=None)
    tmux_p.add_argument("--log-path", default=None)
    tmux_p.add_argument("--cwd", default=os.getcwd())
    tmux_p.add_argument("cmd", nargs=argparse.REMAINDER)
    tmux_p.set_defaults(func=cmd_tmux_run)

    status_p = sub.add_parser("status", help="Show run status")
    add_notify_flags(status_p)
    status_p.add_argument("--limit", type=int, default=20)
    status_p.add_argument("--running-only", action="store_true")
    status_p.add_argument(
        "--reconcile",
        action="store_true",
        help="Mark orphaned RUNNING runs as STOPPED and emit one terminal notification",
    )
    status_p.add_argument(
        "--reconcile-stale-minutes",
        type=int,
        default=None,
        help="Only reconcile orphaned runs when last heartbeat/update is older than N minutes",
    )
    status_p.set_defaults(func=cmd_status)

    stop_p = sub.add_parser("stop", help="Stop a run by run_id")
    add_notify_flags(stop_p)
    stop_p.add_argument("--run-id", required=True)
    stop_p.set_defaults(func=cmd_stop)

    return parser


def main(argv: Optional[list[str]] = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    return int(args.func(args))


if __name__ == "__main__":
    raise SystemExit(main())
