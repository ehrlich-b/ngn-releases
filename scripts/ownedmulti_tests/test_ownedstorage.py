import copy
import gzip
import hashlib
import json
from pathlib import Path
import shutil
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import ownedmatch as match
import ownedsmp


def item(path):
    return {'path': str(path), 'sha256': match.sha256(path)}


class StorageTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.out = Path(self.temp.name)
        self.spec = {'trace_storage': 'ram-gzip', 'maximum_trace_bytes': 1 << 20}

    def trace(self, data=b'full\r\ntrace\n' * 10000):
        storage = match.ownedstorage.TraceStorage(self.out, self.spec)
        with storage:
            storage.path.write_bytes(data)
        return storage, data

    def test_trace_round_trip_preserves_every_byte_and_audit_lines(self):
        storage, data = self.trace()
        lines, digest = match.ownedstorage.read_trace(self.out, self.spec)
        self.assertEqual(lines, data.decode().splitlines())
        self.assertEqual(digest, hashlib.sha256(data).hexdigest())
        self.assertEqual(gzip.decompress((self.out / 'fastchess.log.gz').read_bytes()), data)
        self.assertFalse(storage.directory.exists())
        self.assertLess((self.out / 'fastchess.log.gz').stat().st_size, len(data) // 10)

    def test_failed_stage_archives_partial_trace_and_cannot_be_scored(self):
        storage = match.ownedstorage.TraceStorage(self.out, self.spec)
        with self.assertRaisesRegex(RuntimeError, 'original stage failure'):
            with storage:
                storage.path.write_bytes(b'complete partial trace\n')
                raise RuntimeError('original stage failure')
        self.assertEqual(gzip.decompress((self.out / 'fastchess.log.gz').read_bytes()), b'complete partial trace\n')
        self.assertFalse(storage.directory.exists())
        with self.assertRaises(match.CandidateMatchError):
            match.ownedstorage.read_trace(self.out, self.spec)

    def test_archive_failure_preserves_original_stage_error_and_path_receipt(self):
        storage = match.ownedstorage.TraceStorage(self.out, self.spec)
        self.addCleanup(shutil.rmtree, storage.directory)
        with self.assertRaisesRegex(RuntimeError, 'original'):
            with storage:
                raise RuntimeError('original')
        receipt = json.loads((self.out / 'trace-storage.json').read_text())
        self.assertEqual(receipt['state'], 'FAILED')
        self.assertEqual(receipt['raw_path'], str(storage.path))

    def test_compressed_and_uncompressed_corruption_are_rejected(self):
        self.trace()
        receipt_path = self.out / 'trace-storage.json'
        receipt = json.loads(receipt_path.read_text())
        for key, value in [('raw_sha256', '0' * 64), ('raw_bytes', 1), ('compressed_sha256', '0' * 64)]:
            changed = dict(receipt, **{key: value})
            receipt_path.write_text(json.dumps(changed))
            with self.subTest(key=key), self.assertRaises(match.CandidateMatchError):
                match.ownedstorage.read_trace(self.out, self.spec)
        receipt_path.write_text(json.dumps(receipt))
        with self.assertRaises(match.CandidateMatchError):
            match.ownedstorage.read_trace(self.out, dict(self.spec, maximum_trace_bytes=8))

    def test_trace_mode_and_budget_are_bounded(self):
        for spec in [{'trace_storage': 'other'}, *[dict(self.spec, maximum_trace_bytes=x) for x in (None, True, 0, (2 << 30) + 1)]]:
            with self.subTest(spec=spec), self.assertRaises(match.CandidateMatchError):
                match.ownedstorage.TraceStorage(self.out, spec)

    def test_readonly_pool_shares_exact_tools_and_copies_openings(self):
        original = self.out / 'original'; original.mkdir()
        pool = self.out / 'pool'; pool.mkdir()
        spec = {'input_pool': {}}
        for key in ('fastchess', 'auditor', 'stockfish', 'openings_pgn', 'opening_prefixes'):
            path = original / key; path.write_bytes(key.encode())
            spec[key] = item(path)
            if key in ('fastchess', 'auditor', 'stockfish'):
                shared = pool / key; shutil.copy2(path, shared); shared.chmod(0o555)
                spec['input_pool'][key] = item(shared)
        out = self.out / 'cell'; out.mkdir()
        staged = match.ownedstorage.stage_inputs(spec, out, match.quickmatch.checked)
        for key in spec['input_pool']:
            self.assertEqual(staged[key].stat().st_ino, Path(spec['input_pool'][key]['path']).stat().st_ino)
        self.assertNotEqual(staged['openings_pgn'].stat().st_ino, Path(spec['openings_pgn']['path']).stat().st_ino)
        for mutation in ('writable', 'hash', 'bytes', 'unexpected-key'):
            changed = copy.deepcopy(spec)
            shared = pool / 'stockfish'
            if mutation == 'writable': shared.chmod(0o755)
            if mutation == 'hash': changed['input_pool']['stockfish']['sha256'] = '0' * 64
            if mutation == 'bytes': shared.chmod(0o755); shared.write_bytes(b'drift'); shared.chmod(0o555)
            if mutation == 'unexpected-key': changed['input_pool']['engine'] = item(shared)
            bad = self.out / mutation; bad.mkdir()
            with self.subTest(mutation=mutation), self.assertRaises(match.CandidateMatchError):
                match.ownedstorage.stage_inputs(changed, bad, match.quickmatch.checked)
            shared.chmod(0o555)
            if mutation == 'bytes':
                shared.chmod(0o755); shared.write_bytes(b'stockfish'); shared.chmod(0o555)

    def test_aa_binds_storage_configuration(self):
        role = {'engine': {'sha256': '1' * 64}, 'network': None, 'options': [['Threads', 1]]}
        spec = dict(self.spec, purpose='anchor', tc='10+0.1', concurrency=2, cpus=[12, 14], strict=True,
                    input_pool={}, roles=[role, role])
        control = {'purpose': 'aa', 'pass': True, 'instrument': match.instrument(),
                   'configuration': match.configuration(spec), 'role_identity': match.role_identity(role)}
        path = self.out / 'control.json'; path.write_text(json.dumps(control))
        spec['control_report'] = item(path)
        match.admit_control(spec, match.instrument())
        for key, value in [('trace_storage', 'disk'), ('maximum_trace_bytes', 2 << 20), ('input_pool', {'different': {}})]:
            with self.subTest(key=key), self.assertRaises(match.CandidateMatchError):
                match.admit_control(dict(spec, **{key: value}), match.instrument())

    def test_initial_width_two_requires_the_complete_accepted_same_role(self):
        def saved(name, data):
            path = self.out / name; path.write_text(json.dumps(data)); return item(path)
        role = {'engine': {'sha256': '1' * 64}, 'network': {'sha256': '2' * 64},
                'options': [['Threads', 2], ['Hash', 256]]}
        audit = saved('audit.json', {'status': 'PASS', 'games': 800, 'pairs': 400})
        operational = saved('operational.json', {'pass': True})
        report = saved('report.json', {'games': 800, 'pairs': 400, 'elo_interval_95': [1, 10]})
        verdict = {'state': 'COMPLETE', 'decision': 'SELECT_WIDTH', 'selected_width': 2,
                   'candidate_width': 2, 'baseline_width': 1, 'report': report}
        control = saved('gate-control.json', {'pass': True, 'purpose': 'gate', 'report_sha256': report['sha256'], 'role_identity': match.role_identity(role),
                        'audit_sha256': audit['sha256'], 'operational_sha256': operational['sha256']})
        plan = {'initial_width': 2, 'role': role, 'initial_selection': {
            'verdict': saved('verdict.json', verdict), 'control': control, 'audit': audit,
            'operational': operational, 'spec': saved('spec.json', {'purpose': 'gate', 'roles': [role]})}}
        self.assertEqual(ownedsmp.initial_width(plan), 2)
        self.assertEqual(ownedsmp.initial_width({}), 1)
        with self.assertRaises(match.CandidateMatchError):
            ownedsmp.initial_width(dict(plan, role=dict(role, network={'sha256': '3' * 64})))
        plan['initial_selection']['verdict'] = saved('rejected.json', dict(verdict, decision='RETAIN_BASELINE_WIDTH'))
        with self.assertRaises(match.CandidateMatchError):
            ownedsmp.initial_width(plan)


if __name__ == '__main__':
    unittest.main()
