from __future__ import annotations

import sys
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))

from common import safe_environment


class CommonTests(unittest.TestCase):
    def test_safe_environment_is_exact_and_excludes_debug_tuning(self) -> None:
        self.assertEqual(
            safe_environment(),
            {"PATH": "/usr/bin:/bin", "LANG": "C", "LC_ALL": "C", "TZ": "UTC"},
        )
        self.assertFalse(any(key.startswith("NGN_") for key in safe_environment()))


if __name__ == "__main__":
    unittest.main()
