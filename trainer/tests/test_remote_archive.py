"""Exact local/remote record order controls using NGN-owned synthetic boards."""

import hashlib
from concurrent.futures import ThreadPoolExecutor
import json
from pathlib import Path
import tempfile
import threading
import unittest
from urllib.error import HTTPError
from urllib.request import urlopen
from unittest.mock import patch

import numpy as np

from trainer.archive import ARCHIVE_DTYPE, Archive
from trainer.archive_server import make_server
from trainer.remote_archive import RemoteArray
from trainer.tests.synthetic import random_records
from trainer.tests.test_archive import archive_fixture


class RemoteArchiveTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)
        self.path = self.root / "data.bf"
        source = random_records(257, np.random.default_rng(319))
        rows = np.asarray([archive_fixture(record) for record in source], dtype=ARCHIVE_DTYPE)
        rows.tofile(self.path)
        self.pin = dict(bytes=self.path.stat().st_size,
                        mtime_ns=self.path.stat().st_mtime_ns,
                        sha256=hashlib.sha256(self.path.read_bytes()).hexdigest())
        (self.root / "files.json").write_text(json.dumps({"data.bf": self.pin}))
        self.server = make_server(self.root, "test-token", ("127.0.0.1", 0))
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.descriptor = self.root / "data.archive.json"
        self.descriptor.write_text(json.dumps(dict(bytes=self.pin["bytes"], sha256=self.pin["sha256"],
            url=f"http://127.0.0.1:{self.server.server_port}/test-token/data.bf")))

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()
        self.directory.cleanup()

    def test_exact_partition_batch_boundaries_shuffle_and_cache(self):
        local = Archive([str(self.path)], block_records=31, seed=992, holdout_modulus=4)
        remote = Archive([str(self.descriptor)], block_records=31, seed=992, holdout_modulus=4)
        self.assertEqual(local.count, remote.count)
        self.assertEqual(local.raw_count, remote.raw_count)
        for batch in (1, 19, 64, 300):
            for seed in (17, 144):
                for shuffle in (False, True):
                    for a, b in ((local, remote), (local.holdout(), remote.holdout())):
                        left = list(a.batches(batch, seed, shuffle))
                        right = list(b.batches(batch, seed, shuffle))
                        self.assertEqual([len(x) for x in left], [len(x) for x in right])
                        for expected, actual in zip(left, right):
                            np.testing.assert_array_equal(expected, actual)
        # Cached validation must survive loss of the serving host.
        with patch("trainer.remote_archive.urlopen", side_effect=AssertionError("remote reread")):
            np.testing.assert_array_equal(np.concatenate(list(local.holdout().batches(19, 3))),
                                          np.concatenate(list(remote.holdout().batches(19, 3))))

    def test_range_read_preserves_duplicate_indexes_and_rejects_changed_file(self):
        remote = RemoteArray(self.descriptor, ARCHIVE_DTYPE)
        local = np.memmap(self.path, dtype=ARCHIVE_DTYPE, mode="r")
        indexes = np.array([30, 3, 17, 3], dtype=np.int64)
        np.testing.assert_array_equal(remote[indexes], local[indexes])
        np.testing.assert_array_equal(remote[250:300], local[250:300])
        self.assertEqual(len(remote[0:0]), 0)
        with self.assertRaises(IndexError):
            remote[np.array([257])]
        self.path.write_bytes(self.path.read_bytes() + bytes(32))
        with self.assertRaises(HTTPError) as failed:
            remote[0:1]
        self.assertEqual(failed.exception.code, 412)

    def test_wrong_digest_and_partial_response_fail_before_decoding(self):
        remote = RemoteArray(self.descriptor, ARCHIVE_DTYPE)
        remote.etag = '"' + "0" * 64 + '"'
        with self.assertRaises(HTTPError):
            remote[0:1]
        remote = RemoteArray(self.descriptor, ARCHIVE_DTYPE)

        class Response:
            status = 206
            headers = {"ETag": remote.etag, "Content-Range": f"bytes 0-31/{remote.size}",
                       "Content-Length": "32"}
            def __enter__(self):
                return self
            def __exit__(self, *_args):
                pass
            def read(self, _count):
                return bytes(31)

        with patch("trainer.remote_archive.urlopen", return_value=Response()):
            with self.assertRaisesRegex(ValueError, "Incomplete"):
                remote[0:1]

    def test_descriptor_publication_wakes_one_blocking_request(self):
        url = f"http://127.0.0.1:{self.server.server_port}/test-token/descriptor.json"
        def read():
            with urlopen(url, timeout=5) as response:
                return json.loads(response.read())
        with ThreadPoolExecutor(max_workers=1) as executor:
            future = executor.submit(read)
            temporary = self.root / "t74.archive.json.tmp"
            temporary.write_bytes(self.descriptor.read_bytes())
            temporary.replace(self.root / "t74.archive.json")
            self.assertEqual(future.result(timeout=5), json.loads(self.descriptor.read_text()))


if __name__ == "__main__":
    unittest.main()
