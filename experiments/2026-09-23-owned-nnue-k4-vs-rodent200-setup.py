#!/usr/bin/env python3
"""Freeze a same-code K4 versus borrowed Rodent match on disjoint openings."""

from __future__ import annotations

import copy
import hashlib
import importlib.util
import json
import shutil
import subprocess
from pathlib import Path

ROOT = Path('/home/ehrli/nnue-owned-k4-20260920')
BASE = ROOT / 'integration-v1/k4-main20m-lr1-confirm-400-v1/manifest-held.json'
SOURCE_OLD = ROOT / 'integration-v1/k4-pilot-screen-40-v1/inputs/source/candidate-match'
SOURCE_NEW = ROOT / 'integration-v1/k4-owned-vs-rodent-v1-source'
INTEGRATION = ROOT / 'integration-v1/k4-main20m-vs-rodent-200-v1'
OPENINGS = INTEGRATION / 'inputs/openings'
SOURCE_OPENINGS = Path('/mnt/c/Users/ehrli/sprt_openings.txt')
OLD_PREFIXES = ROOT / 'integration-v1/k4-main20m-lr1-confirm-400-v1/inputs/openings/opening-prefixes-4041-4240.txt'
CONVERTER = Path('/home/ehrli/repos/ngn-next/output/nnue-smp-b0-20260905/opening-pgn-conversion/convert_uci_openings')
MODEL = ROOT / 'runs/k4-main-m1-lr1-20260922/model-selected.nnue'
RODENT = Path('/home/ehrli/repos/ngn-smp-depth-stagger-gate-20260918/run-001/inputs/rodent_anand_512hl.bin')
STAGED = {
    'manifest.py': Path('/mnt/c/Users/ehrli/ngn-candidate-match-borrowed-manifest.py'),
    'manifest.schema.json': Path('/mnt/c/Users/ehrli/ngn-candidate-match-borrowed-schema.json'),
    'tests/test_manifest.py': Path('/mnt/c/Users/ehrli/ngn-candidate-match-borrowed-test-manifest.py'),
    'README.md': Path('/mnt/c/Users/ehrli/ngn-candidate-match-borrowed-readme.md'),
}
EXPECTED = {
    BASE: '8054dac3d04f0d71f621bc18792a2167d9d1d6830159372f915e262c565b4536',
    SOURCE_OPENINGS: '974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222',
    OLD_PREFIXES: '645894923fa6541a687eac7b65c5d3f09c094c3b503f94ccd26f18c5c695148e',
    CONVERTER: '3c99382a07760ac72e7cf0461f5d04a3ff9614338a9058dbcd6f3dcf49cdbcb5',
    MODEL: 'cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6',
    RODENT: '5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb',
    STAGED['manifest.py']: 'd1ad1c5e1bb70b2f2532eb09ba20f6235d2e8a92de586179017f8a1e5d50cb84',
    STAGED['manifest.schema.json']: '8911954c0a0f031f04e56513fc072bff3e71b6605d68b297292e064882fccac4',
    STAGED['tests/test_manifest.py']: 'a6bcc8691578222d1c179759f1c5e4fe510ae39613bbcc86b1b6003a62e8c6de',
}
SOURCE_FILES = {
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


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


for path, expected in EXPECTED.items():
    assert path.is_file() and sha256(path) == expected, path
for key, filename in SOURCE_FILES.items():
    old_path = SOURCE_OLD / filename
    assert old_path.is_file(), old_path
assert not SOURCE_NEW.exists() and not INTEGRATION.exists()

shutil.copytree(SOURCE_OLD, SOURCE_NEW, ignore=shutil.ignore_patterns('__pycache__', '*.pyc'))
for name, staged in STAGED.items():
    dest = SOURCE_NEW / name
    shutil.copy2(staged, dest)
    assert sha256(dest) == sha256(staged)
subprocess.run(
    ['python3', '-m', 'unittest', 'discover', '-s', str(SOURCE_NEW / 'tests'), '-p', 'test_*.py'],
    check=True, cwd=SOURCE_NEW,
)

source_lines = [line.strip() for line in SOURCE_OPENINGS.read_text().splitlines() if line.strip()]
old_lines = [line.strip() for line in OLD_PREFIXES.read_text().splitlines() if line.strip()]
assert len(source_lines) == 5000 and old_lines == source_lines[4040:4240]
new_lines = source_lines[4240:4340]
assert len(new_lines) == 100 and len(set(new_lines)) == 100
assert set(new_lines).isdisjoint(old_lines)

OPENINGS.mkdir(parents=True)
prefixes = OPENINGS / 'opening-prefixes-4241-4340.txt'
pgn = OPENINGS / 'openings-4241-4340.pgn'
prefixes.write_text('\n'.join(new_lines) + '\n')
subprocess.run([str(CONVERTER), str(prefixes), str(pgn), '100'], check=True)
assert pgn.read_text().count('[Event ') == 100

value = json.loads(BASE.read_text())
assert value['schema'] == 'ngn-candidate-match-v1' and value['status'] == 'HELD'
value['schema'] = 'ngn-candidate-match-borrowed-v1'
value['purpose'] = (
    'Fixed 200-game, 100-pair, 10+0.1 same-code owned K4 versus borrowed '
    'Rodent V1.1 Anand evaluator diagnostic; opening pairs 4241-4340 are '
    'disjoint from all prior K4 screens and confirmation. No rating claim.'
)
for key, filename in SOURCE_FILES.items():
    path = SOURCE_NEW / filename
    value['inputs'][key] = {'path': str(path), 'sha256': sha256(path)}
value['inputs']['opening_pgn'] = {'path': str(pgn), 'sha256': sha256(pgn)}
value['inputs']['opening_prefixes'] = {'path': str(prefixes), 'sha256': sha256(prefixes)}
k4 = copy.deepcopy(value['inputs']['roles'][1])
k4['id'] = 'k4'
rodent = copy.deepcopy(k4)
rodent['id'] = 'rodent'
rodent['display_name'] = 'NGN-Borrowed-Rodent-V1.1-Anand'
rodent['backend'] = 'rodent-v1.1-anand'
rodent['network'] = {'format': 'rodent-v1.1-anand', 'path': str(RODENT), 'sha256': EXPECTED[RODENT]}
for option in rodent['uci_options']:
    if option['name'] == 'EvalBackend':
        option['value'] = 'rodent-v1.1-anand'
value['inputs']['roles'] = [k4, rodent]
value['match'].update({
    'seed': 2026092301,
    'games': 200,
    'pairs': 100,
    'time_control': '10+0.1',
    'concurrency': 2,
    'affinity_cpus': [12, 14],
})
value['limits'].update({
    'match_seconds': 14400,
    'trace_audit_seconds': 1800,
    'chess_audit_seconds': 1800,
})
value['acceptance']['expected_games'] = 200
value['acceptance']['expected_pairs'] = 100

spec = importlib.util.spec_from_file_location('candidate_manifest', SOURCE_NEW / 'manifest.py')
assert spec is not None and spec.loader is not None
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
module.validate_manifest(value)
held = INTEGRATION / 'manifest-held.json'
with held.open('x') as out:
    json.dump(value, out, indent=2)
    out.write('\n')
receipt = {
    'schema': 'ngn-k4-vs-rodent200-setup-v1',
    'base_manifest_sha256': sha256(BASE),
    'source_module_sha256': sha256(SOURCE_NEW / 'manifest.py'),
    'source_schema_sha256': sha256(SOURCE_NEW / 'manifest.schema.json'),
    'new_prefixes_sha256': sha256(prefixes),
    'new_pgn_sha256': sha256(pgn),
    'held_manifest_sha256': sha256(held),
    'k4_model_sha256': sha256(MODEL),
    'rodent_model_sha256': sha256(RODENT),
    'new_indices': [4241, 4340],
    'prefix_sets_disjoint': True,
    'review_subject_sha256': module.review_subject_sha256(value),
    'bootstrap_seed': 2026092301,
    'bootstrap_replicates': 100000,
}
with (INTEGRATION / 'setup-receipt.json').open('x') as out:
    json.dump(receipt, out, indent=2)
    out.write('\n')
print(json.dumps(receipt, indent=2))
