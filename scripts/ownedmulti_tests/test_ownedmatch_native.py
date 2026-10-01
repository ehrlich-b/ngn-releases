import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import ownedmatch as match

RELEASE = Path('/home/ehrli/nnue-owned-continuation-20260928/release-build-0.2.0-rc.1/ngn')
NETWORK = Path('/home/ehrli/nnue-owned-k4-20260920/night-20260927/runs/wdl25-e10.nnue')


class NativeAdmissionTests(unittest.TestCase):
    def role(self):
        return {'name': 'Maelstrom-3.3.0',
                'engine': {'path': '/maelstrom', 'sha256': match.MAELSTROM330_SHA256},
                'network': None, 'options': [['Hash', 128], ['Ponder', False]]}

    def test_single_worker_release_is_admitted_without_sending_threads(self):
        match.validate_one_worker_role(self.role())
        for mutation in ('hash', 'threads', 'ponder', 'duplicate', 'network', 'hash-range', 'hash-type'):
            role = copy.deepcopy(self.role())
            if mutation == 'hash': role['engine']['sha256'] = '0' * 64
            if mutation == 'threads': role['options'].append(['Threads', 1])
            if mutation == 'ponder': role['options'][1][1] = True
            if mutation == 'duplicate': role['options'].append(['Hash', 128])
            if mutation == 'network': role['network'] = {'path': '/network', 'sha256': '1' * 64}
            if mutation == 'hash-range': role['options'][0][1] = 4097
            if mutation == 'hash-type': role['options'][0][1] = True
            with self.subTest(mutation=mutation), self.assertRaises(match.CandidateMatchError):
                match.validate_one_worker_role(role)

    def test_other_artifacts_still_require_an_explicit_one_worker_option(self):
        role = {'engine': {'sha256': '0' * 64}, 'options': [['Threads', 1]]}
        match.validate_one_worker_role(role)
        for options in ([], [['Threads', 2]], [['Threads', True]], [['Threads', 1], ['Threads', 1]]):
            with self.subTest(options=options), self.assertRaises(match.CandidateMatchError):
                match.validate_one_worker_role(dict(role, options=options))

    def test_protocol_identity_and_absent_threads_are_verified(self):
        lines = ['id name Maelstrom v3.3.0', 'id author Saigautam Bonam',
                 'option name Hash type spin default 256 min 1 max 4096',
                 'option name Ponder type check default false', 'uciok']
        match.validate_maelstrom_handshake(lines, match.parse_advertised_options(lines))
        for mutation in ('version', 'author', 'duplicate-id', 'threads', 'hash-domain', 'missing-ponder'):
            changed = list(lines)
            if mutation == 'version': changed[0] = 'id name Maelstrom v3.2.0'
            if mutation == 'author': changed[1] = 'id author Unknown'
            if mutation == 'duplicate-id': changed.append(lines[0])
            if mutation == 'threads': changed.append('option name Threads type spin default 1 min 1 max 8')
            if mutation == 'hash-domain': changed[2] = 'option name Hash type spin default 256 min 2 max 4096'
            if mutation == 'missing-ponder': changed.pop(3)
            with self.subTest(mutation=mutation), self.assertRaises(match.CandidateMatchError):
                match.validate_maelstrom_handshake(changed, match.parse_advertised_options(changed))

    def test_new_anchor_cannot_reuse_a_different_aa_instrument_or_clock(self):
        with tempfile.TemporaryDirectory() as raw:
            path = Path(raw) / 'control.json'
            role = {'engine': {'sha256': '1' * 64}, 'network': None, 'options': [['Threads', 1]]}
            spec = {'purpose': 'anchor', 'tc': '10+0.1', 'concurrency': 2,
                    'cpus': [12, 14], 'strict': True, 'roles': [role, self.role()]}
            hashes = {'ownedmatch.py': '2' * 64}
            control = {'purpose': 'aa', 'pass': True, 'instrument': hashes,
                       'configuration': match.configuration(spec), 'role_identity': match.role_identity(role)}
            path.write_text(json.dumps(control))
            spec['control_report'] = {'path': str(path), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
            match.admit_control(spec, hashes)
            with self.assertRaises(match.CandidateMatchError):
                match.admit_control(spec, {'ownedmatch.py': '3' * 64})
            with self.assertRaises(match.CandidateMatchError):
                match.admit_control(dict(spec, tc='60+0.6'), hashes)


@unittest.skipUnless(RELEASE.is_file() and NETWORK.is_file() and hasattr(os, 'sched_getaffinity'),
                     'frozen release on WSL required')
class ReleaseStartupTests(unittest.TestCase):
    def test_staged_owned_release_starts_before_evalfile_can_be_set(self):
        role = {'engine': {'path': str(RELEASE),
                           'sha256': 'de86da92c75ada0f5449ae47d66d55879e221054731190e0c7c885d02123937f'},
                'network': {'path': str(NETWORK),
                            'sha256': '1ec8fc1737ddfdd5f8b6ff0b4e26778085fb563f7c5e82c1fc5d3ea454b6ec29'},
                'options': [['Threads', 1], ['OwnBook', False], ['EvalFile', '$NETWORK'],
                            ['EvalBackend', 'ngn-k4-768-v1'], ['Hash', 128],
                            ['Move Overhead', 100], ['K4EvalScale', 60]]}
        with tempfile.TemporaryDirectory() as raw:
            cwd = Path(raw)
            binary, options = match.stage_role_inputs(role, cwd)
            launcher = cwd / 'role_exec.py'
            shutil.copy2(match.HELPERS / 'role_exec.py', launcher)
            launcher.chmod(0o555)
            (cwd / 'role-config.json').write_text(json.dumps({
                'schema': 'ngn-candidate-role-exec-v1', 'engine': str(binary),
                'engine_sha256': role['engine']['sha256'], 'gomaxprocs': '1', 'expected_cwd': str(cwd)}))
            cpu = min(os.sched_getaffinity(0))
            receipt = match.preflight(launcher, cwd, options, role, cwd / 'positive.jsonl', cpu)
            self.assertTrue(receipt['pass'])
            self.assertTrue(receipt['ngn'])
            (cwd / 'ngn.nnue').unlink()
            with self.assertRaisesRegex(match.CandidateMatchError, 'evaluator startup configuration failed'):
                match.preflight(launcher, cwd, options, role, cwd / 'missing-network.jsonl', cpu)


if __name__ == '__main__':
    unittest.main()
