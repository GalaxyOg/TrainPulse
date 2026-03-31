from __future__ import annotations

import json
import os
import signal
import sqlite3
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Any


class _WebhookHandler(BaseHTTPRequestHandler):
    events: list[dict[str, Any]] = []
    lock = threading.Lock()

    def do_POST(self) -> None:  # noqa: N802
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length).decode("utf-8")
        payload = json.loads(body)
        with self.lock:
            self.events.append(payload)
        response = json.dumps({"code": 0, "msg": "ok"}).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(response)))
        self.end_headers()
        self.wfile.write(response)

    def log_message(self, fmt: str, *args: object) -> None:
        return


class CliIntegrationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        _WebhookHandler.events = []
        cls.server = HTTPServer(("127.0.0.1", 0), _WebhookHandler)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.webhook_url = f"http://127.0.0.1:{cls.server.server_address[1]}/hook"

    @classmethod
    def tearDownClass(cls) -> None:
        cls.server.shutdown()
        cls.thread.join(timeout=3)

    def setUp(self) -> None:
        with _WebhookHandler.lock:
            _WebhookHandler.events = []
        self.tmpdir = tempfile.TemporaryDirectory()
        self.root = Path(self.tmpdir.name)
        self.store_path = self.root / "runs.db"
        self.env = self._base_env()

    def tearDown(self) -> None:
        self.tmpdir.cleanup()

    def _base_env(self) -> dict[str, str]:
        root = str(Path(__file__).resolve().parents[1])
        env = os.environ.copy()
        env["PYTHONPATH"] = root + os.pathsep + env.get("PYTHONPATH", "")
        env["PYTHONDONTWRITEBYTECODE"] = "1"
        return env

    def _run_cli(self, args: list[str], cwd: Path) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, "-m", "train_notify.cli", *args],
            cwd=str(cwd),
            env=self.env,
            capture_output=True,
            text=True,
            check=False,
        )

    def test_run_success_failure_and_project_detect(self) -> None:
        repo1 = self.root / "repo_one"
        repo2 = self.root / "repo_two"
        repo1.mkdir()
        repo2.mkdir()
        subprocess.run(["git", "init"], cwd=repo1, check=True, capture_output=True)
        subprocess.run(["git", "init"], cwd=repo2, check=True, capture_output=True)

        ok = self._run_cli(
            [
                "run",
                "--webhook-url",
                self.webhook_url,
                "--store-path",
                str(self.store_path),
                "--",
                sys.executable,
                "-c",
                "print('ok')",
            ],
            cwd=repo1,
        )
        bad = self._run_cli(
            [
                "run",
                "--webhook-url",
                self.webhook_url,
                "--store-path",
                str(self.store_path),
                "--",
                sys.executable,
                "-c",
                "import sys; sys.exit(3)",
            ],
            cwd=repo2,
        )
        self.assertEqual(ok.returncode, 0, ok.stderr)
        self.assertEqual(bad.returncode, 3, bad.stderr)

        time.sleep(0.5)
        combined = json.dumps(_WebhookHandler.events, ensure_ascii=False)
        self.assertIn("repo_one", combined)
        self.assertIn("repo_two", combined)
        self.assertIn("SUCCEEDED", combined)
        self.assertIn("FAILED", combined)

    def test_run_interrupted(self) -> None:
        repo = self.root / "repo_interrupt"
        repo.mkdir()
        subprocess.run(["git", "init"], cwd=repo, check=True, capture_output=True)

        proc = subprocess.Popen(
            [
                sys.executable,
                "-m",
                "train_notify.cli",
                "run",
                "--webhook-url",
                self.webhook_url,
                "--store-path",
                str(self.store_path),
                "--",
                sys.executable,
                "-c",
                "import time; time.sleep(8)",
            ],
            cwd=str(repo),
            env=self.env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        time.sleep(1.0)
        proc.send_signal(signal.SIGINT)
        proc.communicate(timeout=15)
        self.assertNotEqual(proc.returncode, 0)

        time.sleep(0.5)
        all_events = json.dumps(_WebhookHandler.events, ensure_ascii=False)
        self.assertIn("INTERRUPTED", all_events)

    def test_parallel_runs_unique_run_id(self) -> None:
        repo = self.root / "repo_parallel"
        repo.mkdir()
        subprocess.run(["git", "init"], cwd=repo, check=True, capture_output=True)

        procs: list[subprocess.Popen[str]] = []
        for idx in range(3):
            procs.append(
                subprocess.Popen(
                    [
                        sys.executable,
                        "-m",
                        "train_notify.cli",
                        "run",
                        "--webhook-url",
                        self.webhook_url,
                        "--store-path",
                        str(self.store_path),
                        "--job-name",
                        f"job-{idx}",
                        "--",
                        sys.executable,
                        "-c",
                        "import time; time.sleep(0.8); print('done')",
                    ],
                    cwd=str(repo),
                    env=self.env,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True,
                )
            )
        for proc in procs:
            proc.communicate(timeout=20)
            self.assertEqual(proc.returncode, 0)

        conn = sqlite3.connect(str(self.store_path))
        try:
            rows = conn.execute("SELECT DISTINCT run_id FROM runs").fetchall()
        finally:
            conn.close()
        self.assertGreaterEqual(len(rows), 3)

    def test_failed_dry_run_has_visible_failed_output(self) -> None:
        repo = self.root / "repo_dryrun_fail"
        repo.mkdir()
        subprocess.run(["git", "init"], cwd=repo, check=True, capture_output=True)

        failed = self._run_cli(
            [
                "run",
                "--dry-run",
                "--store-path",
                str(self.store_path),
                "--",
                sys.executable,
                "-c",
                "import sys; sys.exit(3)",
            ],
            cwd=repo,
        )
        self.assertEqual(failed.returncode, 3, failed.stderr)
        self.assertIn("[train-notify][dry-run][FAILED]", failed.stderr)


if __name__ == "__main__":
    unittest.main()
