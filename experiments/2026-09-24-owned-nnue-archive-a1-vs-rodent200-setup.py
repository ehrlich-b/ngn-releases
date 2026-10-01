#!/usr/bin/env python3
"""Freeze the archive-labeled K4 versus borrowed Rodent match.

Everything except the K4 network is copied from the audited 2026-09-23 clock
match: same engine binary, Rodent network, opening pairs 4241-4340, seed,
10+0.1 clock, concurrency and affinity. The per-opening-pair scores are therefore
paired with the original 20M K4's scores against the same opponent.
"""

from __future__ import annotations

import hashlib
import importlib.util
import json
import sys
from pathlib import Path

ROOT = Path('/home/ehrli/nnue-owned-k4-20260920')
BASE_INTEGRATION = ROOT / 'integration-v1/k4-main20m-vs-rodent-200-v1'
BASE = BASE_INTEGRATION / 'manifest-held.json'
SOURCE = ROOT / 'integration-v1/k4-owned-vs-rodent-v1-source'
EXPECTED_BASE = '60e7df9d8c4eefefaa2fb15d28a718803430c02ec4a78ed1129d0ceacb60f919'
OLD_K4 = 'cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6'


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main() -> None:
    integration, model, label = Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3]
    assert sha256(BASE) == EXPECTED_BASE
    assert not integration.exists()
    model_sha = sha256(model)
    assert model_sha != OLD_K4

    value = json.loads(BASE.read_text())
    assert value['schema'] == 'ngn-candidate-match-borrowed-v1' and value['status'] == 'HELD'
    k4, rodent = value['inputs']['roles']
    assert k4['id'] == 'k4' and k4['network']['sha256'] == OLD_K4
    assert rodent['backend'] == 'rodent-v1.1-anand'
    k4['display_name'] = f'NGN-Owned-K4-{label}'
    k4['network'] = {'format': 'ngn-k4-768-v1', 'path': str(model), 'sha256': model_sha}
    value['purpose'] = (
        f'Fixed 200-game, 100-pair, 10+0.1 archive-labeled owned K4 ({label}) versus '
        'borrowed Rodent V1.1 Anand; identical engine, openings 4241-4340, seed and '
        'settings as the audited 20M K4 clock match, paired by opening. No rating claim.'
    )
    for key in ('opening_pgn', 'opening_prefixes'):
        assert sha256(Path(value['inputs'][key]['path'])) == value['inputs'][key]['sha256']

    spec = importlib.util.spec_from_file_location('candidate_manifest', SOURCE / 'manifest.py')
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    module.validate_manifest(value)

    integration.mkdir(parents=True)
    held = integration / 'manifest-held.json'
    with held.open('x') as out:
        json.dump(value, out, indent=2)
        out.write('\n')
    receipt = {
        'schema': 'ngn-k4-archive-vs-rodent200-setup-v1',
        'base_manifest_sha256': EXPECTED_BASE,
        'held_manifest_sha256': sha256(held),
        'review_subject_sha256': module.review_subject_sha256(value),
        'k4_model_sha256': model_sha,
        'paired_with_k4_model_sha256': OLD_K4,
        'decision_rule': (
            'Promote over the 20M SF18-labeled K4 only if the paired per-opening '
            'score difference (archive minus 20M, same 100 pairs) has a 95% '
            'bootstrap lower bound above zero (100,000 resamples, seed 2026092401).'
        ),
    }
    with (integration / 'setup-receipt.json').open('x') as out:
        json.dump(receipt, out, indent=2)
        out.write('\n')
    print(json.dumps(receipt, indent=2))


if __name__ == '__main__':
    main()
