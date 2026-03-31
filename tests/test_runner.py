from __future__ import annotations

import unittest

from train_notify.core.models import Event
from train_notify.core.runner import determine_final_event, normalize_exit_code


class RunnerTests(unittest.TestCase):
    def test_determine_final_event(self) -> None:
        self.assertEqual(determine_final_event(0, interrupted=False), Event.SUCCEEDED)
        self.assertEqual(determine_final_event(1, interrupted=False), Event.FAILED)
        self.assertEqual(determine_final_event(130, interrupted=True), Event.INTERRUPTED)

    def test_normalize_exit_code(self) -> None:
        self.assertEqual(normalize_exit_code(-2), 130)
        self.assertEqual(normalize_exit_code(3), 3)


if __name__ == "__main__":
    unittest.main()
