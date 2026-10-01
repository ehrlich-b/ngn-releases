#!/usr/bin/env python3
"""Compare retained NGN receipts with the independently pinned Stockfish oracle."""
import hashlib
import json
import pathlib
import sys

directory = pathlib.Path(sys.argv[1])
ngn_path = directory / 'ngn-baseline-probe.stdout'
sf_path = directory / 'stockfish-probe.json'
ngn = json.loads(ngn_path.read_text())
sf = json.loads(sf_path.read_text())
assert ngn['source_head'] == sf['source_head'] == '701328646490f1bc8ed214002f578a58c9b0723e'
assert ngn['all_four_defects_reproduced'] is True
assert ngn['both_legal_ep_controls_valid'] is True
assert len(ngn['fixtures']) == len(sf['fixtures']) == 6
oracle = {row['name']: row for row in sf['fixtures']}
results = []
for row in ngn['fixtures']:
    name = row['fixture']['name']
    independent = oracle[name]
    fixture = dict(row['fixture'])
    fixture.setdefault('cycle', [])  # Go omits the empty cycle on legal controls.
    assert fixture == independent['fixture'], name
    for stage in ('incremental', 'fen_ep', 'fen_no_ep'):
        actual = row[stage]
        expected = independent[stage]
        assert actual['legal_moves'] == sorted(expected['perft']['1']['moves']), (name, stage, 'legal moves')
        assert actual['perft_2'] == expected['perft']['2']['nodes'], (name, stage, 'perft 2')
    illegal = row['fixture']['kind'].startswith('illegal_')
    if illegal:
        assert row['true_legal_identity_occurrences'] == 3
        assert row['ngn_hash_occurrences'] == 2
        assert row['defect_reproduced'] is True
        assert all(row['restoration'].values()), (name, 'restoration')
        assert len(row['played_history']) == len(independent['history']) == 9
        for actual, expected in zip(row['played_history'], independent['history']):
            assert actual['ply'] == expected['ply'] and actual['move'] == expected['move']
            assert actual['fide_legal_identity'] == ' '.join(expected['after']['fen'].split()[:4])
        repetitions = [row['played_history'][ply - 1] for ply in (1, 5, 9)]
        assert repetitions[0]['hash'] != repetitions[1]['hash'] == repetitions[2]['hash']
        assert [entry['ngn_hash_count'] for entry in repetitions] == [1, 1, 2]
        assert all(not entry['ngn_is_fide_draw_rule'] for entry in repetitions)
        sf_keys = [independent['history'][ply - 1]['after']['key'] for ply in (1, 5, 9)]
        assert len(set(sf_keys)) == 1
    else:
        assert row['legal_control_valid'] is True
    results.append({'name': name, 'legal_sets_and_depth_2_perft_agree': True, 'played_repetition_verified': illegal, 'legal_ep_control_verified': not illegal})
receipt = {
    'schema': 'ngn-ep-independent-receipt-comparison-v1',
    'source_head': ngn['source_head'],
    'input_sha256': {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in (ngn_path, sf_path)},
    'all_six_fixtures_agree_with_independent_oracle': True,
    'fixtures': results,
}
print(json.dumps(receipt, indent=2))
