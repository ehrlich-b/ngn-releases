#!/usr/bin/env python3
"""Freeze a fixed-node counterpart to the audited K4/Rodent clock match."""

from __future__ import annotations

import hashlib
import json
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
REPO = HERE.parent
sys.path.insert(0, str(REPO / 'scripts' / 'candidate-match'))
from manifest import review_subject_sha256, validate_manifest  # noqa: E402

BASELINE = HERE / '2026-09-23-owned-nnue-k4-vs-rodent200-held.json'
BASELINE_SHA256 = '60e7df9d8c4eefefaa2fb15d28a718803430c02ec4a78ed1129d0ceacb60f919'
OUTPUT = HERE / '2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-held.json'
SOURCE_ROOT = '/home/ehrli/nnue-owned-k4-20260920/integration-v1/k4-fixednodes-source-20260923'
SOURCE_NAMES = {
    'runner': 'run_candidate_match.py',
    'schema': 'manifest.schema.json',
    'common': 'common.py',
    'manifest_module': 'manifest.py',
    'supervisor': 'process_supervisor.py',
    'uci_preflight': 'uci_preflight.py',
    'match_stage': 'run_match_stage.py',
    'trace_auditor': 'trace_audit.py',
    'role_exec': 'role_exec.py',
}


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def main() -> None:
    if OUTPUT.exists():
        raise FileExistsError(OUTPUT)
    original = BASELINE.read_bytes()
    if sha256(original) != BASELINE_SHA256:
        raise ValueError('clock-match HELD manifest changed')
    value = json.loads(original)
    for key, filename in SOURCE_NAMES.items():
        local = REPO / 'scripts' / 'candidate-match' / filename
        value['inputs'][key] = {'path': f'{SOURCE_ROOT}/{filename}', 'sha256': sha256(local.read_bytes())}
    value['schema'] = 'ngn-candidate-match-borrowed-fixed-nodes-v1'
    value['purpose'] = (
        'Fixed 200-game, 100-pair same-code owned K4 versus borrowed Rodent V1.1 '
        'Anand diagnostic, same openings 4241-4340 as the 10+0.1 match, '
        '160000 nodes per move per side. No absolute rating claim.'
    )
    value['match'].pop('time_control')
    value['match']['node_limit'] = 160_000
    value['match']['seed'] = 2026092302
    value['limits']['match_seconds'] = 10_800
    validate_manifest(value)
    OUTPUT.write_text(json.dumps(value, indent=2, sort_keys=True) + '\n')
    print(json.dumps({
        'held_manifest_sha256': sha256(OUTPUT.read_bytes()),
        'review_subject_sha256': review_subject_sha256(value),
        'games': value['match']['games'],
        'node_limit': value['match']['node_limit'],
        'opening_sha256': value['inputs']['opening_pgn']['sha256'],
    }, indent=2))


if __name__ == '__main__':
    main()
