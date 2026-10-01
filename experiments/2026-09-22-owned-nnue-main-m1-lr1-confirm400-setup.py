#!/usr/bin/env python3
"""Freeze disjoint openings and a HELD 400-game K4 versus HCE manifest."""

from __future__ import annotations

import hashlib
import importlib.util
import json
import subprocess
from pathlib import Path

ROOT = Path('/home/ehrli/nnue-owned-k4-20260920')
BASE = ROOT / 'integration-v1/k4-main20m-lr1-screen-40-v1/manifest-held.json'
INTEGRATION = ROOT / 'integration-v1/k4-main20m-lr1-confirm-400-v1'
OPENINGS = INTEGRATION / 'inputs/openings'
SOURCE = Path('/mnt/c/Users/ehrli/sprt_openings.txt')
OLD_A = Path('/mnt/c/Users/ehrli/ngn-k4-pilot-openings-4001-4020.txt')
OLD_B = ROOT / 'integration-v1/k4-main20m-lr1-screen-40-v1/inputs/openings/opening-prefixes-4021-4040.txt'
CONVERTER = Path('/home/ehrli/repos/ngn-next/output/nnue-smp-b0-20260905/opening-pgn-conversion/convert_uci_openings')
MODULE = ROOT / 'integration-v1/k4-pilot-screen-40-v1/inputs/source/candidate-match/manifest.py'
MODEL = ROOT / 'runs/k4-main-m1-lr1-20260922/model-selected.nnue'

EXPECTED = {
    BASE: '4aed850b0b0c8b916513811b7f8fea03a5409d9fd7d267341cff04bce689ec15',
    SOURCE: '974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222',
    OLD_A: '1d044000dca4cf29aac0987cc84bfb11e307a6a27826bc9d068f738ce93d1e3a',
    OLD_B: '5da172ae1d740d788d7e149ea48847e9b9e340accad3ef0c99440ffe3c315e66',
    CONVERTER: '3c99382a07760ac72e7cf0461f5d04a3ff9614338a9058dbcd6f3dcf49cdbcb5',
    MODEL: 'cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6',
}


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


for path, expected in EXPECTED.items():
    assert path.is_file() and sha256(path) == expected, path
assert not INTEGRATION.exists()

source_lines = [line.strip() for line in SOURCE.read_text().splitlines() if line.strip()]
old_a = [line.strip() for line in OLD_A.read_text().splitlines() if line.strip()]
old_b = [line.strip() for line in OLD_B.read_text().splitlines() if line.strip()]
assert len(source_lines) == 5000
assert old_a == source_lines[4000:4020] and old_b == source_lines[4020:4040]
new_lines = source_lines[4040:4240]
assert len(new_lines) == 200 and len(set(new_lines)) == 200
assert set(new_lines).isdisjoint(old_a + old_b)

OPENINGS.mkdir(parents=True)
prefixes = OPENINGS / 'opening-prefixes-4041-4240.txt'
pgn = OPENINGS / 'openings-4041-4240.pgn'
prefixes.write_text('\n'.join(new_lines) + '\n')
subprocess.run([str(CONVERTER), str(prefixes), str(pgn), '200'], check=True)
assert pgn.read_text().count('[Event ') == 200

value = json.loads(BASE.read_text())
assert value['schema'] == 'ngn-candidate-match-v1' and value['status'] == 'HELD'
value['purpose'] = (
    'Fixed 400-game, 200-pair, 30+0.3 same-code HCE versus owned K4 20M '
    'lower-rate update-20480 confirmation. Opening pairs 4041-4240 are '
    'disjoint from all prior K4 screens. Milestone requires paired-bootstrap '
    '95% lower score bound above 50%; no absolute rating claim.'
)
value['match'].update({
    'seed': 2026092201,
    'games': 400,
    'pairs': 200,
    'time_control': '30+0.3',
    'concurrency': 2,
    'affinity_cpus': [12, 14],
})
value['limits'].update({
    'match_seconds': 86400,
    'trace_audit_seconds': 3600,
    'chess_audit_seconds': 3600,
})
value['acceptance']['expected_games'] = 400
value['acceptance']['expected_pairs'] = 200
value['inputs']['opening_pgn'] = {'path': str(pgn), 'sha256': sha256(pgn)}
value['inputs']['opening_prefixes'] = {'path': str(prefixes), 'sha256': sha256(prefixes)}

spec = importlib.util.spec_from_file_location('candidate_manifest', MODULE)
assert spec is not None and spec.loader is not None
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
module.validate_manifest(value)
held = INTEGRATION / 'manifest-held.json'
with held.open('x') as out:
    json.dump(value, out, indent=2)
    out.write('\n')

receipt = {
    'schema': 'ngn-k4-main20m-confirm400-setup-v1',
    'source_sha256': sha256(SOURCE),
    'old_a_sha256': sha256(OLD_A),
    'old_b_sha256': sha256(OLD_B),
    'new_prefixes_sha256': sha256(prefixes),
    'new_pgn_sha256': sha256(pgn),
    'held_manifest_sha256': sha256(held),
    'model_sha256': sha256(MODEL),
    'new_indices': [4041, 4240],
    'prefix_sets_disjoint': True,
    'review_subject_sha256': module.review_subject_sha256(value),
    'bootstrap_seed': 2026092201,
    'bootstrap_replicates': 100000,
}
with (INTEGRATION / 'setup-receipt.json').open('x') as out:
    json.dump(receipt, out, indent=2)
    out.write('\n')
print(json.dumps(receipt, indent=2))
