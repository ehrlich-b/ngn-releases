"""Lossless match traces and one private, frozen copy of large audit tools."""
import gzip
import hashlib
import os
import shutil
import tempfile
from pathlib import Path

from common import CandidateMatchError, atomic_json, sha256


def stage_inputs(spec, out, checked):
    pool = spec.get('input_pool', {})
    if not isinstance(pool, dict) or set(pool) - {'fastchess', 'auditor', 'stockfish'}:
        raise CandidateMatchError('invalid frozen tool pool')
    frozen = {}
    for key in ('fastchess', 'auditor', 'stockfish', 'openings_pgn', 'opening_prefixes'):
        original = checked(spec[key])
        target = out / (key + original.suffix)
        if key in pool:
            if pool[key]['sha256'] != spec[key]['sha256']:
                raise CandidateMatchError('shared tool hash differs from original')
            try:
                shared = checked(pool[key])
            except SystemExit as error:
                raise CandidateMatchError(str(error)) from error
            if shared.stat().st_mode & 0o222:
                raise CandidateMatchError('shared tool must match the original and be read-only')
            os.link(shared, target)
        else:
            shutil.copy2(original, target)
        frozen[key] = target
    return frozen


class TraceStorage:
    """Buffer a trace on tmpfs; archive all bytes after the supervised child exits.

    There is no background compressor or additional child to supervise. Failed
    stages also archive their complete partial trace. An archive failure leaves
    the RAM file and its path receipt intact for recovery, and rejects the cell.
    """
    def __init__(self, out, spec):
        self.out = out
        self.mode = spec.get('trace_storage', 'disk')
        self.directory = None
        if self.mode not in ('disk', 'ram-gzip'):
            raise CandidateMatchError('invalid trace storage mode')
        self.maximum = spec.get('maximum_trace_bytes')
        if self.mode == 'ram-gzip':
            if type(self.maximum) is not int or not 0 < self.maximum <= 2 << 30:
                raise CandidateMatchError('RAM trace requires a bounded byte budget')
            capacity = os.statvfs('/dev/shm')
            if capacity.f_bavail * capacity.f_frsize < self.maximum:
                raise CandidateMatchError('insufficient tmpfs trace capacity')
            self.directory = Path(tempfile.mkdtemp(prefix='ngn-owned-trace-', dir='/dev/shm'))
            self.path = self.directory / 'fastchess.log'
        else:
            self.path = out / 'fastchess.log'
        atomic_json(out / 'trace-storage.json', {'state': 'WRITING', 'mode': self.mode,
                                               'raw_path': str(self.path)})

    def __enter__(self):
        return self

    def __exit__(self, kind, error, traceback):
        if self.mode == 'disk':
            return False
        try:
            digest = hashlib.sha256()
            count = 0
            target = self.out / 'fastchess.log.gz'
            with self.path.open('rb') as source, target.open('xb') as dest:
                os.chmod(target, 0o600)
                with gzip.GzipFile(filename='', fileobj=dest, mode='wb', compresslevel=1, mtime=0) as archive:
                    while block := source.read(1 << 20):
                        archive.write(block)
                        digest.update(block)
                        count += len(block)
            atomic_json(self.out / 'trace-storage.json', {
                'state': 'ARCHIVED', 'mode': self.mode, 'raw_path': str(self.path),
                'raw_bytes': count, 'raw_sha256': digest.hexdigest(),
                'compressed_bytes': target.stat().st_size, 'compressed_sha256': sha256(target),
                'stage_failed': error is not None,
            })
            self.path.unlink()
            self.directory.rmdir()
        except Exception as archive_error:
            atomic_json(self.out / 'trace-storage.json', {
                'state': 'FAILED', 'mode': self.mode, 'raw_path': str(self.path),
                'error': str(archive_error), 'stage_failed': error is not None,
            })
            if error is None:
                raise
            error.add_note('trace archive failed: ' + str(archive_error))
        return False


def read_trace(out, spec):
    if spec.get('trace_storage', 'disk') == 'disk':
        path = out / 'fastchess.log'
        return path.read_text().splitlines(), sha256(path)
    import json
    receipt = json.loads((out / 'trace-storage.json').read_text())
    path = out / 'fastchess.log.gz'
    if receipt['state'] != 'ARCHIVED' or receipt['mode'] != 'ram-gzip' or receipt['stage_failed'] or \
       sha256(path) != receipt['compressed_sha256'] or path.stat().st_size != receipt['compressed_bytes']:
        raise CandidateMatchError('trace archive receipt failed')
    digest, count = hashlib.sha256(), 0
    with gzip.open(path, 'rb') as source:
        while block := source.read(1 << 20):
            digest.update(block)
            count += len(block)
            if count > spec['maximum_trace_bytes']:
                raise CandidateMatchError('trace exceeds declared byte budget')
    if count != receipt['raw_bytes'] or digest.hexdigest() != receipt['raw_sha256']:
        raise CandidateMatchError('uncompressed trace hash or size differs')
    with gzip.open(path, 'rt') as source:
        return source.read().splitlines(), digest.hexdigest()
