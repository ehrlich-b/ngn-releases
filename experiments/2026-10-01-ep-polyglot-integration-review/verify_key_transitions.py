#!/usr/bin/env python3
"""Verify the twelve test-only EP Polyglot expected-value transitions."""

from __future__ import annotations

import hashlib
import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
TASK = ROOT.parent
EVIDENCE = Path(__file__).resolve().parent / "evidence"
ORACLE = EVIDENCE / "integration-independent-key-transition.json"
ORIGINAL_ORACLE = TASK / "integration-independent-key-transition.json"
OLD = EVIDENCE / "enpassant_repetition_correctness_test.legacy.go.txt"
NEW = ROOT / "engine/enpassant_repetition_correctness_test.go"


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


assert sha256(ORACLE) == "172796acc6b60cc450fe5614734d03c5068d3b5fa4be208e51dc4a44d284623d"
assert ORACLE.read_bytes() == ORIGINAL_ORACLE.read_bytes()
oracle = json.loads(ORACLE.read_text())
assert oracle["schema"] == "ngn-independent-ep-canonical-book-key-transition-v1"
assert oracle["source_fixture_sha256"] == sha256(OLD) == "3e9b1cb951a6640e36c86e2ebb1e7ba42914a2e1f9254227c4ff152922347c98"
assert len(oracle["rows"]) == 12

old_text = OLD.read_text()
new_text = NEW.read_text()
expected = old_text.replace(
    "// The Polyglot values are exact NGN proof-baseline values: they deliberately\n"
    "// retain adjacency-only EP semantics and must not follow the repetition-key fix.\n",
    "// The Polyglot values are independently frozen standard book keys. They retain\n"
    "// adjacency-only EP semantics and deliberately do not follow the repetition key.\n",
)
expected = expected.replace("want proof-baseline %016X", "want standard Polyglot %016X")

transitions = []
for row in oracle["rows"]:
    old = "0x" + row["legacy_key"].upper()
    new = "0x" + row["standard_key"].upper()
    assert expected.count(old) == 1, (row["fixture"], row["value_field"], old)
    expected = expected.replace(old, new)
    transitions.append(
        {
            "fixture": row["fixture"],
            "field": row["value_field"],
            "fen": row["fen"],
            "legacy": row["legacy_key"],
            "standard": row["standard_key"],
            "legal_ep": row["legal_ep"],
            "pseudo_legal_ep": row["pseudo_legal_ep"],
        }
    )

assert expected == new_text, "active source differs beyond the twelve values and two obsolete descriptions"

blocks = {
    match.group("name"): {
        "wantPolyglotEP": match.group("ep").lower(),
        "wantPolyglotNoEP": match.group("noep").lower(),
    }
    for match in re.finditer(
        r'name:\s*"(?P<name>[^"]+)".*?wantPolyglotEP:\s*0x(?P<ep>[0-9A-F]+),'
        r"\s*wantPolyglotNoEP:\s*0x(?P<noep>[0-9A-F]+)",
        new_text,
        re.DOTALL,
    )
}
assert len(blocks) == 6
for row in oracle["rows"]:
    assert blocks[row["fixture"]][row["value_field"]] == row["standard_key"]

print(
    json.dumps(
        {
            "state": "EXACTLY_TWELVE_EXTERNAL_STANDARD_KEY_TRANSITIONS",
            "oracle_sha256": sha256(ORACLE),
            "old_source_sha256": sha256(OLD),
            "new_source_sha256": sha256(NEW),
            "transition_count": len(transitions),
            "transitions": transitions,
        },
        indent=2,
        sort_keys=True,
    )
)
