#!/usr/bin/env python3
"""Freeze and validate the disjoint 40-game 20M K4 screen inputs."""

from __future__ import annotations

import hashlib
import importlib.util
import json
import subprocess
from pathlib import Path

ROOT = Path('/home/ehrli/nnue-owned-k4-20260920')
BASE = ROOT / 'integration-v1/k4-probe5m-lr1-screen-40-v1/manifest-held.json'
INTEGRATION = ROOT / 'integration-v1/k4-main20m-lr1-screen-40-v1'
OPENINGS = INTEGRATION / 'inputs/openings'
SOURCE = Path('/mnt/c/Users/ehrli/sprt_openings.txt')
OLD_PREFIXES = Path('/mnt/c/Users/ehrli/ngn-k4-pilot-openings-4001-4020.txt')
CONVERTER = Path('/home/ehrli/repos/ngn-next/output/nnue-smp-b0-20260905/opening-pgn-conversion/convert_uci_openings')
MODULE = ROOT / 'integration-v1/k4-pilot-screen-40-v1/inputs/source/candidate-match/manifest.py'
MODEL = ROOT / 'runs/k4-main-m1-lr1-20260922/model-selected.nnue'

EXPECTED = {
    SOURCE: '974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222',
    OLD_PREFIXES: '1d044000dca4cf29aac0987cc84bfb11e307a6a27826bc9d068f738ce93d1e3a',
    CONVERTER: '3c99382a07760ac72e7cf0461f5d04a3ff9614338a9058dbcd6f3dcf49cdbcb5',
    MODEL: 'cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6',
}


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


for path, expected in EXPECTED.items():
    assert path.is_file() and sha256(path) == expected, path
assert not INTEGRATION.exists()

source_lines = [line.strip() for line in SOURCE.read_text().splitlines() if line.strip()]
old_lines = [line.strip() for line in OLD_PREFIXES.read_text().splitlines() if line.strip()]
assert len(source_lines) == 5000 and old_lines == source_lines[4000:4020]
new_lines = source_lines[4020:4040]
assert len(new_lines) == 20 and len(set(new_lines)) == 20
assert set(new_lines).isdisjoint(old_lines)

OPENINGS.mkdir(parents=True)
prefixes = OPENINGS / 'opening-prefixes-4021-4040.txt'
pgn = OPENINGS / 'openings-4021-4040.pgn'
prefixes.write_text('\n'.join(new_lines) + '\n')
subprocess.run([str(CONVERTER), str(prefixes), str(pgn), '20'], check=True)
assert pgn.read_text().count('[Event ') == 20

value = json.loads(BASE.read_text())
assert value['schema'] == 'ngn-candidate-match-v1' and value['status'] == 'HELD'
value['purpose'] = (
    'Forty-game paired-opening 10+0.1 same-code HCE versus owned K4 20M '
    'lower-rate update-20480 screen on the optimized AVX2 engine; opening '
    'pairs 4021-4040 are disjoint from all prior K4 screens; diagnostic '
    'cutoff 50%, no Elo or promotion claim.'
)
value['match']['seed'] = 20260923
value['inputs']['opening_pgn'] = {'path': str(pgn), 'sha256': sha256(pgn)}
value['inputs']['opening_prefixes'] = {'path': str(prefixes), 'sha256': sha256(prefixes)}
for role in value['inputs']['roles']:
    if role['id'] == 'k4':
        role['display_name'] = 'NGN-Owned-K4-20M-LR1-20480'
        role['network'] = {
            'format': 'ngn-k4-768-v1',
            'path': str(MODEL),
            'sha256': EXPECTED[MODEL],
        }

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
    'schema': 'ngn-k4-main20m-screen-setup-v1',
    'source_sha256': sha256(SOURCE),
    'old_prefixes_sha256': sha256(OLD_PREFIXES),
    'new_prefixes_sha256': sha256(prefixes),
    'new_pgn_sha256': sha256(pgn),
    'held_manifest_sha256': sha256(held),
    'model_sha256': sha256(MODEL),
    'old_indices': [4001, 4020],
    'new_indices': [4021, 4040],
    'prefix_sets_disjoint': True,
    'review_subject_sha256': module.review_subject_sha256(value),
}
with (INTEGRATION / 'setup-receipt.json').open('x') as out:
    json.dump(receipt, out, indent=2)
    out.write('\n')
print(json.dumps(receipt, indent=2))
