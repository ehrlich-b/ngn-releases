#!/usr/bin/env python3
"""Independent positive/negative checks for the retained PV classifier."""

from __future__ import annotations

import importlib.util
import json
import sys
from pathlib import Path


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def main() -> int:
    if len(sys.argv) != 4:
        raise SystemExit("usage: classifier_regression.py PROBE_PY FRONTIER_JSON PROBE_RESULT_JSON")
    probe_path, frontier_path, result_path = map(Path, sys.argv[1:])
    spec = importlib.util.spec_from_file_location("rodent_pv_probe", probe_path)
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot load probe module")
    probe = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(probe)

    import chess

    frontier = json.loads(frontier_path.read_text())["rodent_v12"]["independent_chess"]
    result = json.loads(result_path.read_text())
    historical_checks = 0
    completion_checks = 0
    for fixture in frontier["fixtures"]:
        tokens = probe.parse_pv(fixture["info"])
        classified = probe.classify_tokens(chess, chess.Board(fixture["root_fen"]), tokens)
        require(not classified["legal"], "historical suffix-less line classified legal")
        require(classified["index"] == 9 and classified["token"] == "g2g1", "wrong historical failure")
        require(classified["suffixless_last_rank_pawn"], "historical failure not identified as promotion")
        require(sorted(classified["legal_promotion_alternatives"]) == sorted(probe.LEGAL_PROMOTIONS), "wrong alternatives")
        historical_checks += 1
        expected = {item["replacement"]: item["every_move_legal"] for item in fixture["promotion_completions"]}
        for replacement, want_legal in expected.items():
            replaced = list(tokens)
            replaced[9] = replacement
            got = probe.classify_tokens(chess, chess.Board(fixture["root_fen"]), replaced)["legal"]
            require(got == want_legal, f"completion {replacement}: got {got}, want {want_legal}")
            completion_checks += 1
        require(
            probe.classify_tokens(chess, chess.Board(fixture["root_fen"]), [fixture["actual_recorded_bestmove"]])["legal"],
            "recorded bestmove is not legal",
        )

    forced = chess.Board(probe.FORCED_PROMOTION_FEN)
    forced_missing = probe.classify_tokens(chess, forced, ["g2g1"])
    require(not forced_missing["legal"] and forced_missing["suffixless_last_rank_pawn"], "positive detector failed")
    require(sorted(forced_missing["legal_promotion_alternatives"]) == sorted(probe.LEGAL_PROMOTIONS), "positive alternatives failed")
    for token in probe.LEGAL_PROMOTIONS:
        require(probe.classify_tokens(chess, forced, [token])["legal"], f"legal control {token} rejected")

    new_line_checks = 0
    for search in result["new_runtime"]:
        for line in search["pv_lines"]:
            require(line["classification"]["legal"], f"new line unexpectedly illegal in {search['name']}")
            new_line_checks += 1
        require(search["bestmove_classification"]["legal"], f"new bestmove illegal in {search['name']}")
    require(result["controls"]["forced_promotion_bestmove"] in probe.LEGAL_PROMOTIONS, "forced renderer control failed")
    require(result["controls"]["ordinary_negative_all_pvs_legal"], "ordinary negative control failed")

    print(
        json.dumps(
            {
                "state": "CLASSIFIER_REGRESSIONS_PASS",
                "historical_suffixless_positive_checks": historical_checks,
                "promotion_completion_checks": completion_checks,
                "synthetic_missing_suffix_positive_check": True,
                "legal_promotion_negative_checks": len(probe.LEGAL_PROMOTIONS),
                "new_legal_pv_negative_checks": new_line_checks,
            },
            sort_keys=True,
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
