from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path

from trainpulse.core.context import detect_project_name, infer_job_name


class ContextTests(unittest.TestCase):
    def test_detect_project_name_fallback(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            workdir = Path(tmp) / "non_git_repo"
            workdir.mkdir()
            self.assertEqual(detect_project_name(workdir), "non_git_repo")

    def test_detect_project_name_from_git(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            workdir = Path(tmp) / "repo_a"
            workdir.mkdir()
            subprocess.run(["git", "init"], cwd=workdir, check=True, capture_output=True)
            self.assertEqual(detect_project_name(workdir), "repo_a")

    def test_infer_job_name(self) -> None:
        self.assertEqual(infer_job_name(["python", "train.py", "--x", "1"]), "train")
        self.assertEqual(infer_job_name(["uv", "run", "python", "foo/bar.py"]), "bar")
        self.assertEqual(infer_job_name(["conda", "run", "-n", "x", "python", "algo.py"]), "algo")
        self.assertEqual(infer_job_name(["bash", "run.sh"]), "bash")


if __name__ == "__main__":
    unittest.main()
