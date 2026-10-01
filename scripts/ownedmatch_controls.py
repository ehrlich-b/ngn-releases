#!/usr/bin/env python3
"""Exercise Counter compatibility boundaries against an audited match trace."""
import argparse
import copy
import io
import json
from pathlib import Path
from types import SimpleNamespace

import ownedmatch as match


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    match.require_wsl()
    run = args.run.resolve()
    assert json.loads((run / 'STATE.json').read_text())['state'] == 'COMPLETE'
    spec = json.loads((run / 'spec.json').read_text())
    witness = json.loads((run / 'child-processes.json').read_text())
    roles, trace_roles = [], []
    for index, role in enumerate(spec['roles']):
        roles.append({'name': role['name'], 'sha256': role['engine']['sha256']})
        trace_roles.append({'display_name': role['name'],
            'resolved_options': [{'name': name, 'value': match.value_text(str(run / f'role{index}' / 'network') if value == '$NETWORK' else value)}
                                 for name, value in role['options']],
            'expected_refreshes': 2 * spec['pairs'],
            'expected_processes': len(witness['observed_instances'][str(index)])})
    lines = (run / 'fastchess.log').read_text().splitlines()
    positive = match.operational_trace(lines, trace_roles, roles)
    records = positive['allowed_exact_counter55_startup_banners']
    version = next(row for row in records if row['kind'] == 'version')
    weights = next(row for row in records if row['kind'] == 'embedded_weights')
    position = version['line'] - 1
    original = lines[position]
    mutations = {}
    for label, text in [('wrong_revision', original.replace('63c487ca724c620f71c129d62129c6fb9109c872', '0' * 40)),
                        ('wrong_runtime', original.replace('go1.21.0', 'go1.22.0')),
                        ('warning_banner', original.replace('[Engine]', '[WARN]')),
                        ('illegal_pv', original.replace(version['payload'], 'Warning; Illegal PV move g2g1'))]:
        assert text != original, label
        changed = lines.copy()
        changed[position] = text
        mutations[label] = (changed, roles)
    mutations['duplicate_banner'] = (lines + [original], roles)
    mutations['missing_weights'] = ([line for index, line in enumerate(lines) if index != weights['line'] - 1], roles)
    ngn_name = next(role['name'] for role in roles if role['sha256'] != match.COUNTER55_SHA256)
    mutations['banner_from_ngn'] = (lines + [original.replace('<stderr>' + version['role'], '<stderr>' + ngn_name)], roles)
    # Preserve spacing used by fastchess rather than accepting a no-op mutation.
    if mutations['banner_from_ngn'][0][-1] == original:
        mutations['banner_from_ngn'][0][-1] = original.replace(version['role'], ngn_name, 1)
    assert mutations['banner_from_ngn'][0][-1] != original
    wrong_roles = copy.deepcopy(roles)
    next(role for role in wrong_roles if role['sha256'] == match.COUNTER55_SHA256)['sha256'] = '0' * 64
    mutations['wrong_executable_hash'] = (lines, wrong_roles)
    rejected = {}
    for label, (changed, changed_roles) in mutations.items():
        try:
            match.operational_trace(changed, trace_roles, changed_roles)
        except match.CandidateMatchError as error:
            rejected[label] = str(error)[:300]
        else:
            raise AssertionError('invalid trace accepted: ' + label)
    startup = [version['payload'], weights['payload']]
    protocol_cases = [('exact_release', startup, True, False),
                      ('duplicate', startup + [startup[0]], True, True),
                      ('foreign_engine', startup, False, True),
                      ('fatal_stderr', startup + ['panic: fixture'], True, True)]
    protocols = {}
    for label, payload, eligible, expect_error in protocol_cases:
        process = SimpleNamespace(stdout=io.BytesIO(b''), stderr=io.BytesIO(('\n'.join(payload) + '\n').encode()))
        transcript = io.StringIO()
        protocol = match.CheckedProtocol(process, transcript, eligible)
        for thread in protocol.threads:
            thread.join(timeout=5)
            assert not thread.is_alive()
        assert bool(protocol.stderr_lines) == expect_error, label
        assert len(transcript.getvalue().splitlines()) == len(payload), label
        protocols[label] = {'unexpected_stderr': len(protocol.stderr_lines), 'preserved_lines': len(payload)}
    report = {'schema': 'ngn-owned-counter-compatibility-controls-v1', 'pass': True,
              'instrument': match.instrument(), 'positive_run': str(run),
              'trace_sha256': match.sha256(run / 'fastchess.log'),
              'positive_startup_records': len(records), 'rejected_mutations': rejected, 'protocol_cases': protocols}
    match.atomic_json(args.output, report)
    print(json.dumps(report))


if __name__ == '__main__':
    main()
