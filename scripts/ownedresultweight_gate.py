#!/usr/bin/env python3
"""Run the frozen WDL15 playing gate after the full fresh external anchor."""
import argparse
import datetime
import json
import os
import subprocess
import time
from pathlib import Path

import ownednight
from ownednight import atomic_json, require_wsl, sha256


def checked(path):
    path = Path(path)
    return {'path': str(path), 'sha256': sha256(path)}


def run(root, plan_path):
    require_wsl()
    plan = json.loads(plan_path.read_text()); out = root / 'wdl15-gate'; out.mkdir(exist_ok=False)
    deadline = datetime.datetime.fromisoformat(plan['gate_deadline_utc'].replace('Z', '+00:00')).timestamp()
    atomic_json(out / 'STATE.json', {'state': 'WAITING_EXTERNAL_AND_TRAINING', 'plan': checked(plan_path), 'started': time.time()})
    while True:
        external = json.loads((root / 'external-fresh/STATE.json').read_text())
        training = json.loads((root / 'wdl15/STATE.json').read_text())
        if external['state'] in ['FAILED', 'SKIPPED'] or training['state'] == 'FAILED':
            raise RuntimeError('prerequisite did not produce an accepted terminal')
        if external['state'] == 'COMPLETE' and training['state'] == 'COMPLETE':
            unit = json.loads((root / 'external-fresh/unit.json').read_text())['unit']
            status = subprocess.run(['systemctl', '--user', 'show', unit, '-p', 'ActiveState', '--value'], capture_output=True, text=True, check=True)
            if status.stdout.strip() != 'active':
                break
        if time.time() >= deadline - plan['gate_minimum_remaining_seconds']:
            atomic_json(out / 'STATE.json', {'state': 'SKIPPED', 'reason': 'fixed-cell admission deadline', 'completed': time.time()}); return
        time.sleep(min(600, max(1, deadline - plan['gate_minimum_remaining_seconds'] - time.time())))
    if ownednight.capacity() < 25 or time.time() >= deadline - plan['gate_minimum_remaining_seconds']:
        atomic_json(out / 'STATE.json', {'state': 'SKIPPED', 'reason': 'capacity/time admission', 'completed': time.time()}); return
    baseline = json.loads((root / 'external-fresh/selection.json').read_text())['baseline']
    ready = json.loads((root / 'wdl15/ready.json').read_text())
    if ready['state'] != 'COMPLETE' or ready['plan_sha256'] != sha256(plan_path):
        raise RuntimeError('training readiness differs')
    if sha256(Path(ready['network']['path'])) != ready['network']['sha256']:
        raise RuntimeError('WDL15 network differs')
    if ready['parity_sha256'] != sha256(root / 'wdl15/parity.stdout') or not json.loads((root / 'wdl15/parity.stdout').read_text())['pass']:
        raise RuntimeError('WDL15 export parity differs')
    if baseline['engine']['sha256'] != plan['engine_sha256'] or sha256(Path(baseline['engine']['path'])) != plan['engine_sha256']:
        raise RuntimeError('frozen engine differs')
    if baseline['network']['sha256'] not in plan['allowed_baseline_networks'] or sha256(Path(baseline['network']['path'])) != baseline['network']['sha256']:
        raise RuntimeError('frozen baseline differs')
    spec = json.loads((root / 'specs/reference.json').read_text())
    for key in ['tc', 'concurrency', 'cpus', 'strict', 'opening_plies']:
        if spec[key] != plan['configuration'][key]:
            raise RuntimeError('gate configuration differs')
    candidate = dict(baseline, name='Owned-WDL15', network=ready['network'])
    atomic_json(out / 'selection.json', {'candidate': candidate, 'baseline': baseline, 'plan': checked(plan_path)})
    master = root / 'openings/master.txt'
    if sha256(master) != plan['opening_master_sha256']:
        raise RuntimeError('frozen opening source differs')
    prefixes = master.read_text().splitlines(); receipt = json.loads((root / 'specs/openings-receipt.json').read_text()); converter = Path(receipt['converter_path'])
    if sha256(converter) != receipt['converter_sha256']:
        raise RuntimeError('opening converter differs')
    for cell in plan['matches']:
        name = cell['name']; first, last = cell['opening_lines']; text = out / (name + '.txt'); pgn = out / (name + '.pgn')
        with text.open('x') as handle:
            handle.write('\n'.join(prefixes[first - 1:last]) + '\n')
        subprocess.run([str(converter), str(text), str(pgn), str(last - first + 1)], check=True, timeout=30)
        current = dict(spec, purpose='aa' if name.endswith('-aa') else 'gate', pairs=cell['games'] // 2,
                       roles=[dict(candidate, name='WDL15-A'), dict(candidate, name='WDL15-B')] if name.endswith('-aa') else [candidate, baseline],
                       openings_pgn=checked(pgn), opening_prefixes=checked(text), limit_seconds=min(7200, int(deadline - time.time() - 300)))
        if not name.endswith('-aa'):
            current['control_report'] = checked(out / 'wdl15-aa/control.json')
        path = out / (name + '.json'); atomic_json(path, current)
        atomic_json(out / 'STATE.json', {'state': 'RUNNING', 'name': name, 'started': time.time()})
        ownednight.run_bounded(['python3', str(root / 'source/scripts/ownedmatch.py'), '--spec', str(path), '--out', str(out / name)],
                               out / (name + '-run'), deadline, dict(os.environ, PATH='/usr/bin:/bin'), '4,6,8,10,12,14')
        if json.loads((out / name / 'STATE.json').read_text())['state'] != 'COMPLETE':
            raise RuntimeError('cell lacks an accepted audited terminal')
    report = json.loads((out / 'wdl15-gate/report.json').read_text())
    atomic_json(out / 'verdict.json', {'state': 'COMPLETE', 'decision': 'PROMOTE_WDL15' if report['elo_interval_95'][0] > 0 else 'RETAIN_SELECTED',
                'rule': plan['matches'][1]['decision'], 'baseline': baseline['network'], 'candidate': ready['network'],
                'report': checked(out / 'wdl15-gate/report.json')})
    atomic_json(out / 'STATE.json', {'state': 'COMPLETE', 'completed': time.time()})


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__); parser.add_argument('--root', type=Path, required=True); parser.add_argument('--plan', type=Path, required=True)
    args = parser.parse_args()
    try:
        run(args.root.resolve(), args.plan.resolve())
    except Exception as error:
        atomic_json(args.root / 'wdl15-gate/STATE.json', {'state': 'FAILED', 'error': str(error), 'completed': time.time()}); raise
