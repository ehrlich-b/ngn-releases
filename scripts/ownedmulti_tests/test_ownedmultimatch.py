import copy
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import unittest

HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
import ownedmultimatch as match
sys.path.insert(0, str(HERE.parent / 'candidate-match'))
import smp_match_stage as witness


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


class LayoutTests(unittest.TestCase):
    def spec(self):
        role = {'name': 'A', 'network': {'path': 'net', 'sha256': '1' * 64},
                'options': [['Threads', 2], ['EvalBackend', 'ngn-k4-768-v1'], ['OwnBook', False]]}
        return {'schema': 'ngn-owned-multicore-spec-v1', 'tc': '10+0.1', 'concurrency': 2,
                'cpus': [4, 6, 8, 10], 'cpu_teams': [[4, 6], [8, 10]], 'strict': True,
                'pairs': 100, 'minimum_full_width_fraction': .8, 'roles': [role, dict(role, name='B')]}

    def test_layout_rejects_overlap_width_overcommit_and_uncontrolled_options(self):
        match.validate_layout(self.spec())
        for mutation in ['overlap', 'width', 'duplicate', 'strict', 'concurrency', 'fixed-nodes']:
            value = copy.deepcopy(self.spec())
            if mutation == 'overlap': value['cpu_teams'][1] = [6, 8]
            if mutation == 'width': value['roles'][0]['options'][0][1] = 4
            if mutation == 'duplicate': value['roles'][0]['options'].append(['Threads', 2])
            if mutation == 'strict': value['strict'] = False
            if mutation == 'concurrency': value['concurrency'] = 3
            if mutation == 'fixed-nodes': value.pop('tc'); value['nodes'] = 20000
            with self.subTest(mutation=mutation), self.assertRaises(match.base.CandidateMatchError):
                match.validate_layout(value)

    def test_hidden_thread_mask_drift_is_rejected(self):
        role = {'engine': '/engine', 'gomaxprocs': '2', 'allowed_cpu_masks': ['12,14']}
        row = {'exe': '/engine', 'gomaxprocs': '2', 'cpus_allowed_list': '12,14',
               'tasks': [{'tid': 1, 'cpus_allowed_list': '12,14'}, {'tid': 2, 'cpus_allowed_list': '14'}]}
        self.assertTrue(any('thread 2' in message for message in witness.observation_violations(role, row)))
        row['tasks'][1]['cpus_allowed_list'] = '12,14'
        self.assertFalse(witness.observation_violations(role, row))

    def test_exact_counter_external_mode_and_mixed_receipts(self):
        spec = self.spec()
        spec.update(schema='ngn-owned-multicore-counter-spec-v1', purpose='anchor', strict=False)
        spec['roles'][1] = {'name': 'Counter', 'engine': {'sha256': match.base.COUNTER55_SHA256},
            'network': None, 'options': [['Threads', 1], ['Hash', 128], ['ExperimentSettings', False]]}
        match.validate_layout(spec)
        changed = copy.deepcopy(spec); changed['roles'][1]['engine']['sha256'] = '0' * 64
        with self.assertRaises(match.base.CandidateMatchError): match.validate_layout(changed)
        changed = copy.deepcopy(spec); changed['roles'][1]['options'][0][1] = 2
        with self.assertRaises(match.base.CandidateMatchError): match.validate_layout(changed)
        changed = copy.deepcopy(spec); changed['strict'] = True
        with self.assertRaises(match.base.CandidateMatchError): match.validate_layout(changed)
        def line(body): return '[Engine] [00:00:00] < 1> ' + body
        trace = [line('A <--- setoption name Threads value 2'),
                 line('A ---> info string threads configured 2 effective 2'),
                 line('A <--- go wtime 10000 btime 10000'),
                 line('A ---> info string threads configured 2 effective 2'),
                 line('A ---> bestmove e2e4'),
                 line('Counter <--- go wtime 10000 btime 10000'),
                 line('Counter ---> bestmove e7e5')]
        roles = [{'name': 'A', 'ngn': True, 'options': [['Threads', 2]]},
                 {'name': 'Counter', 'ngn': False, 'options': [['Threads', 1]]}]
        self.assertTrue(match.audit_width_receipts(trace, roles, .8)['pass'])

    def test_aa_binds_cpu_teams_instrument_and_participating_width(self):
        with tempfile.TemporaryDirectory() as raw:
            path = pathlib.Path(raw) / 'control.json'
            spec = self.spec(); spec.update(purpose='gate', opening_plies=6)
            hashes = {'instrument': '1' * 64}
            control = {'pass': True, 'purpose': 'aa', 'instrument': hashes,
                       'configuration': match.configuration(spec), 'role_identity': match.base.role_identity(spec['roles'][0])}
            path.write_text(json.dumps(control)); spec['control_report'] = {'path': str(path), 'sha256': digest(path)}
            match.admit_control(spec, hashes)
            changed = copy.deepcopy(spec); changed['cpu_teams'] = [[4, 8], [6, 10]]
            with self.assertRaises(match.base.CandidateMatchError): match.admit_control(changed, hashes)
            with self.assertRaises(match.base.CandidateMatchError): match.admit_control(spec, {'instrument': '2' * 64})
            changed = copy.deepcopy(spec)
            for role in changed['roles']: role['options'][0][1] = 1
            with self.assertRaises(match.base.CandidateMatchError): match.admit_control(changed, hashes)

    def test_effective_width_receipts_reject_missing_duplicate_foreign_and_false_width(self):
        def line(body): return '[Engine] [00:00:00] < 1> ' + body
        roles = [{'name': 'A', 'options': [['Threads', 2]]}, {'name': 'B', 'options': [['Threads', 1]]}]
        lines = [line('A <--- setoption name Threads value 2'), line('A ---> info string threads configured 2 effective 2'),
                 line('B <--- setoption name Threads value 1'), line('B ---> info string threads configured 1 effective 1'),
                 line('A <--- go wtime 10000 btime 10000'), line('A ---> info string threads configured 2 effective 2'),
                 line('A ---> bestmove e2e4'), line('B <--- go wtime 10000 btime 10000'), line('B ---> bestmove e7e5')]
        self.assertTrue(match.audit_width_receipts(lines, roles, .8)['pass'])
        for mutation in ['missing', 'duplicate', 'foreign', 'wrong-config', 'impossible', 'all-primary', 'missing-option-ack', 'duplicate-option-ack']:
            changed = list(lines)
            if mutation == 'missing': changed.pop(5)
            if mutation == 'duplicate': changed.insert(6, lines[5])
            if mutation == 'foreign': changed[5] = line('Other ---> info string threads configured 2 effective 2')
            if mutation == 'wrong-config': changed[5] = line('A ---> info string threads configured 4 effective 2')
            if mutation == 'impossible': changed[5] = line('A ---> info string threads configured 2 effective 3')
            if mutation == 'all-primary': changed[5] = line('A ---> info string threads configured 2 effective 1')
            if mutation == 'missing-option-ack': changed.pop(1)
            if mutation == 'duplicate-option-ack': changed.insert(2, lines[1])
            with self.subTest(mutation=mutation), self.assertRaises(match.base.CandidateMatchError):
                match.audit_width_receipts(changed, roles, .8)


@unittest.skipUnless(hasattr(os, 'sched_getaffinity') and {12, 14}.issubset(os.sched_getaffinity(0)), 'WSL fixture CPUs required')
class ProcessTests(unittest.TestCase):
    def test_launcher_expands_anchor_and_rejects_unknown_anchor_or_hash(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw); launcher = root / 'smp_role_exec.py'
            shutil.copy2(HERE.parent / 'candidate-match/smp_role_exec.py', launcher)
            engine = pathlib.Path('/usr/bin/python3').resolve()
            config = {'schema': 'ngn-smp-role-exec-v1', 'engine': str(engine), 'engine_sha256': digest(engine),
                      'gomaxprocs': '2', 'expected_cwd': str(root), 'cpu_teams': [[12, 14]]}
            (root / 'role-config.json').write_text(json.dumps(config))
            code = 'import os,json;print(json.dumps({"width":os.environ["GOMAXPROCS"],"cpus":sorted(os.sched_getaffinity(0))}))'
            result = subprocess.run([sys.executable, str(launcher)], cwd=root, input=code,
                preexec_fn=lambda: os.sched_setaffinity(0, {12}), text=True, capture_output=True, timeout=5)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(result.stdout), {'width': '2', 'cpus': [12, 14]})
            result = subprocess.run([sys.executable, str(launcher)], cwd=root,
                preexec_fn=lambda: os.sched_setaffinity(0, {14}), text=True, capture_output=True, timeout=5)
            self.assertEqual(result.returncode, 111)
            self.assertIn('inherited affinity', result.stderr)
            config['engine_sha256'] = '0' * 64
            (root / 'role-config.json').write_text(json.dumps(config))
            result = subprocess.run([sys.executable, str(launcher)], cwd=root,
                preexec_fn=lambda: os.sched_setaffinity(0, {12}), text=True, capture_output=True, timeout=5)
            self.assertEqual(result.returncode, 111)
            self.assertIn('hash', result.stderr)

    def run_stage(self, variant):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw); roles = []
            for name in ['a', 'b']:
                cwd = root / name; cwd.mkdir(); engine = root / ('engine-' + name)
                shutil.copy2('/usr/bin/sleep', engine)
                launcher = cwd / 'fixture_parent.py'; shutil.copy2(HERE / 'fixture_parent.py', launcher)
                roles.append({'id': name, 'engine': str(engine), 'engine_sha256': digest(engine), 'cwd': str(cwd),
                              'gomaxprocs': '2', 'allowed_cpu_masks': ['12,14'], 'launcher': str(launcher)})
            config = {'schema': 'ngn-smp-match-stage-v1', 'command': [sys.executable, str(HERE / 'fixture_parent.py'), variant,
                roles[0]['engine'], roles[0]['cwd'], roles[1]['engine'], roles[1]['cwd']], 'cwd': str(root),
                'environment': {'PATH': '/usr/bin:/bin', 'LANG': 'C'}, 'roles': roles, 'sample_interval_seconds': 0.02,
                'match_stdout': str(root / 'stdout'), 'match_stderr': str(root / 'stderr'), 'witness': str(root / 'witness.json')}
            path = root / 'config.json'; path.write_text(json.dumps(config))
            result = subprocess.run([sys.executable, str(HERE.parent / 'candidate-match/smp_match_stage.py'), '--config', str(path)],
                                    text=True, capture_output=True, timeout=10)
            return result, json.loads((root / 'witness.json').read_text())

    def test_live_multicore_child_witness(self):
        result, receipt = self.run_stage('good')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(receipt['state'], 'COMPLETE')
        self.assertEqual(receipt['schema'], 'ngn-smp-child-process-witness-v1')
        self.assertTrue(receipt['observations'])
        self.assertTrue(all(row['tasks'] and row['cpus_allowed_list'] == '12,14' for row in receipt['observations']))

    def test_wrong_live_width_or_affinity_is_rejected(self):
        for variant, word in [('bad-env', 'GOMAXPROCS'), ('bad-mask', 'CPU mask')]:
            with self.subTest(variant=variant):
                result, receipt = self.run_stage(variant)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(receipt['state'], 'FAILED')
                self.assertTrue(any(word in item for item in receipt['violations']))


if __name__ == '__main__':
    unittest.main()
