"""Exact lift, malformed wire controls, records and nonzero integer oracle."""
import hashlib
import json
from pathlib import Path
import runpy
import struct
import unittest
import zlib

import numpy as np

from trainer.ngnp import RECORD_DTYPE, decode_record, parse_fen
from trainer.refeval import Network as LegacyReference
from trainer.residual_export import lift_bytes, residual_bytes
from trainer.residual_refeval import Network, trunc_div

ROOT = Path(__file__).resolve().parents[2]
FIXTURE = ROOT / "experiments/wave2-1104/residual-convert"


def crc(data):
    return data[:-4]+struct.pack("<I", zlib.crc32(data[:-4]))


class ResidualExportTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.source = (FIXTURE/"source-h32.ngn3").read_bytes()
        cls.zero = (FIXTURE/"zero-h32.ngn6").read_bytes()
        cls.nonzero = (FIXTURE/"nonzero-h32.ngn6").read_bytes()
        cls.fixture = json.loads((FIXTURE/"parity.json").read_text())

    def test_byte_preservation_and_repeat(self):
        self.assertEqual(self.zero, lift_bytes(self.source))
        self.assertEqual(lift_bytes(self.source), lift_bytes(self.source))
        self.assertNotEqual(self.zero, lift_bytes(self.source, 20261105))
        self.assertEqual(self.source[8:96], self.zero[8:96])
        self.assertEqual(self.source[96:-4], self.zero[120:120+len(self.source)-100])
        for name, raw in (("input", self.source), ("output", self.zero), ("nonzero", self.nonzero)):
            self.assertEqual(hashlib.sha256(raw).hexdigest(), self.fixture[name]["sha256"])
            self.assertEqual(zlib.crc32(raw[:-4]), struct.unpack_from("<I", raw, len(raw)-4)[0])
        n = Network(self.zero)
        self.assertTrue(np.any(n.a))
        self.assertTrue(np.any(n.hb))
        self.assertFalse(np.any(n.r))
        self.assertFalse(np.any(n.rb))

    def test_committed_packet_regenerates(self):
        generate = runpy.run_path(str(FIXTURE/"generate.py"))
        for name, raw in generate["build_packet"]().items():
            self.assertEqual(raw, (FIXTURE/name).read_bytes(), name)

    def test_32_records_and_nonzero_route_control(self):
        base, zero, nonzero = (Network(raw) for raw in (self.source, self.zero, self.nonzero))
        legacy = LegacyReference(FIXTURE/"source-h32.ngn3")
        records = np.fromfile(FIXTURE/"records.ngnp", dtype=RECORD_DTYPE)
        self.assertEqual(len(records), 32)
        coverage, changed = set(), 0
        for record, c in zip(records, self.fixture["cases"]):
            p = decode_record(record)
            k = int(p.squares[p.pieces == 5][0])
            coverage.add((base.bucket(p), k % 8 >= 4, p.stm))
            self.assertEqual(zero.evaluate_record(record), c["zero_cp"])
            self.assertEqual(base.evaluate_record(record), c["zero_cp"])
            self.assertEqual(legacy.evaluate_record(record), c["zero_cp"])
            self.assertEqual(nonzero.evaluate_record(record), c["nonzero_cp"])
            self.assertEqual(nonzero.accumulators(p).reshape(-1).tolist(), c["acc"])
            changed += c["nonzero_cp"] != c["zero_cp"]
        self.assertEqual(len(coverage), 32)
        self.assertGreaterEqual(changed, 24, "nonzero fixture must reject an NGNN3 route")
        self.assertEqual(changed, self.fixture["nonzero_differs"])

    def test_invalid_source(self):
        for offset, value in ((0, 0), (4, 6), (8, 48), (12, 4), (16, 4), (20, 254), (24, 65), (28, 401)):
            raw = bytearray(self.source)
            struct.pack_into("<I", raw, offset, value)
            with self.subTest(offset=offset), self.assertRaises(ValueError):
                lift_bytes(crc(raw))
        raw = bytearray(self.source)
        raw[32] = 8
        for invalid in (crc(raw), self.source[:-1], self.source+b"x", self.source[:96],
                        self.source[:-4]+b"\0\0\0\0"):
            with self.assertRaises(ValueError):
                lift_bytes(invalid)

    def test_oracle_invalid_format_and_bounds(self):
        for offset, value in ((4, 3), (8, 48), (12, 4), (16, 4), (20, 256),
                              (96, 8), (100, 65), (104, 256), (108, 63), (112, 401), (116, 1)):
            raw = bytearray(self.zero)
            struct.pack_into("<I", raw, offset, value)
            with self.subTest(offset=offset), self.assertRaises(ValueError):
                Network(crc(raw))
        for raw in (self.zero[:-1], self.zero+b"x", self.zero[:-4]+b"\0\0\0\0"):
            with self.assertRaises(ValueError):
                Network(raw)
        n = Network(self.zero)
        for bad in (np.zeros((16, 63), dtype=np.int16), np.full((16, 64), 32768), n.a*1.0):
            with self.assertRaises(ValueError):
                residual_bytes(self.source, bad, n.hb, n.r, n.rb)
        hb = np.full(16, 2147483647, dtype=np.int64)
        with self.assertRaises(ValueError):
            residual_bytes(self.source, np.ones((16, 64), dtype=np.int16), hb, n.r, n.rb)
        raw = bytearray(self.zero)
        struct.pack_into("<i", raw, 120+12322*32+32+16*64*2, 2147483647)
        # Positive unit weights plus endpoint bias violate the full input-box proof.
        struct.pack_into("<h", raw, 120+12322*32+32, 1)
        with self.assertRaises(ValueError):
            Network(crc(raw))

    def test_signed_truncation_and_clamp_order(self):
        self.assertEqual(trunc_div(np.array([-65, -64, -63, 63, 64, 65]), 64).tolist(), [-1, -1, 0, 0, 1, 1])
        p = parse_fen("7k/8/8/8/8/8/8/K7 w - - 0 1")
        raw = bytearray(self.zero)
        # Zero linear weights; linear bias yields +30000 cp. Residual -10000
        # gives +20000 before clamp, rather than +14999 after a premature clamp.
        linear_offset = 120+2*(6144+1)*32
        raw[linear_offset:linear_offset+8*64*2] = bytes(8*64*2)
        struct.pack_into("<i", raw, linear_offset+8*64*2, 1224000)
        raw[-4-32:-4] = struct.pack("<8i", -408000, 0, 0, 0, 0, 0, 0, 0)
        n = Network(crc(raw))
        self.assertEqual(n.components(p), (30000, -10000))
        self.assertEqual(n.evaluate(p), 20000)
        struct.pack_into("<i", raw, linear_offset+8*64*2, -1)
        raw[-4-32:-4] = bytes(32)
        n = Network(crc(raw))
        self.assertEqual(n.evaluate(p), 0, "negative division must truncate, never floor")


if __name__ == "__main__":
    unittest.main()
