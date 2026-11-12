"""Wire-format regression independent of the training framework."""
import struct
import unittest
import zlib
import numpy as np
from trainer.export import export_bytes, HEADER5, THREAT_FEATURES


class ExportThreatTest(unittest.TestCase):
    def test_sparse_threat_rows_match_source(self):
        h = 16
        threats = np.zeros((THREAT_FEATURES, h), dtype=np.float32)
        threats[3, 2] = 7 / 255
        threats[-1, 9] = -8 / 255
        state = dict(king_map=[0] * 64, w1=np.zeros((768, h)), b1=np.zeros(h),
                     o=np.zeros(2*h), ob=np.array(0), threat_weights=threats)
        for fmt in (None, 5):
            data = export_bytes(state, fmt)
            header = HEADER5.unpack_from(data)
            self.assertEqual((header[0], header[-3]), (b'NGN5', 2))
            offset = HEADER5.size + 768*h*2
            actual = np.zeros_like(threats, dtype=np.int8)
            for _ in range(header[-3]):
                row = struct.unpack_from('<I', data, offset)[0]
                offset += 4
                actual[row] = np.frombuffer(data, dtype=np.int8, count=h, offset=offset)
                offset += h
            np.testing.assert_array_equal(actual, np.rint(threats * 255).astype(np.int8))
            self.assertEqual(struct.unpack_from('<I', data, len(data)-4)[0], zlib.crc32(data[:-4]))
        with self.assertRaises(ValueError):
            export_bytes(state, 3)


if __name__ == '__main__':
    unittest.main()
