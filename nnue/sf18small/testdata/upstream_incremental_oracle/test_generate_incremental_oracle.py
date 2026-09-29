#!/usr/bin/env python3
import unittest

from generate_incremental_oracle import require_declared_root_fen


class RootFENAssociationTest(unittest.TestCase):
    def test_declared_root_is_accepted(self):
        fen = "7k/8/8/8/8/8/8/K7 w - - 0 1"
        require_declared_root_fen("root-positive", fen, "root", {"fen": fen})
        require_declared_root_fen("non-root-ignored", fen, "push", {"fen": "changed"})

    def test_wrong_root_is_rejected(self):
        with self.assertRaisesRegex(SystemExit, "root FEN"):
            require_declared_root_fen(
                "root-negative",
                "7k/8/8/8/8/8/8/K7 w - - 0 1",
                "root",
                {"fen": "7k/8/8/8/8/8/8/1K6 w - - 0 1"},
            )


if __name__ == "__main__":
    unittest.main()
