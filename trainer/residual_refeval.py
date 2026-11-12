"""Independent NumPy integer NGNN3/6 oracle derived from docs/ngnn6.md.

No exporter, legacy reference evaluator, engine features or output kernels are
used. Every numeric array is widened to int64 before arithmetic.
"""

import argparse
from pathlib import Path
import struct
import zlib

import numpy as np

from trainer.ngnp import decode_record, parse_fen


def trunc_div(value, divisor):
    """Signed division toward zero, without a floating point intermediate."""
    value = np.asarray(value, dtype=np.int64)
    return np.where(value < 0, -((-value) // divisor), value // divisor)


class Network:
    def __init__(self, data: bytes):
        if len(data) < 100:
            raise ValueError("Truncated header")
        magic, version, h, kb, nb, qa, qb, scale = struct.unpack_from("<4s7I", data)
        if (magic, version) not in ((b"NGN3", 3), (b"NGN6", 6)):
            raise ValueError("Expected NGNN3 or NGNN6")
        if h not in (32, 768) or (kb, nb, qa, qb, scale) != (8, 8, 255, 64, 400):
            raise ValueError("Unsupported dimensions or scales")
        self.king_map = tuple(data[32:96])
        if any(b > 7 for b in self.king_map):
            raise ValueError("Invalid king map")
        header = 96 if version == 3 else 120
        expected = 12322*h+132 if version == 3 else 12386*h+508
        if len(data) != expected:
            raise ValueError("Wrong file length")
        if version == 6 and struct.unpack_from("<6I", data, 96) != (16, 64, 255, 64, 400, 0):
            raise ValueError("Invalid residual dimensions/scales/flags")
        if zlib.crc32(data[:-4]) != struct.unpack_from("<I", data, len(data)-4)[0]:
            raise ValueError("CRC mismatch")
        self.hidden, self.version = h, version
        offset = header
        def read(dtype, shape):
            nonlocal offset
            count = int(np.prod(shape))
            values = np.frombuffer(data, dtype=dtype, count=count, offset=offset).astype(np.int64)
            offset += count * np.dtype(dtype).itemsize
            return values.reshape(shape)
        self.w = read("<i2", (6144, h))
        self.bias = read("<i2", (h,))
        self.linear = read("<i2", (8, 2*h))
        self.lb = read("<i4", (8,))
        if version == 6:
            self.a = read("<i2", (16, 2*h))
            self.hb = read("<i4", (16,))
            self.r = read("<i2", (8, 16))
            self.rb = read("<i4", (8,))
            for weights, biases in ((self.a, self.hb), (self.r, self.rb)):
                lower = biases + np.minimum(weights, 0).sum(axis=1) * 255
                upper = biases + np.maximum(weights, 0).sum(axis=1) * 255
                if np.any(lower < -2147483648) or np.any(upper > 2147483647):
                    raise ValueError("Affine bound exceeds int32")
        assert offset == len(data)-4

    def accumulators(self, position):
        accum = []
        for perspective in (0, 1):
            kings = position.squares[position.pieces == 5+6*perspective]
            # Match the specified diagnostic fallback for a missing king.
            king = int(kings[0]) if len(kings) else 0
            oriented = king ^ (56 if perspective else 0)
            mirror = 7 if oriented % 8 >= 4 else 0
            bucket = self.king_map[oriented ^ mirror]
            indices = []
            for square, piece in zip(position.squares, position.pieces):
                relative = 0 if int(piece)//6 == perspective else 1
                transformed = int(square) ^ (56 if perspective else 0) ^ mirror
                indices.append(bucket*768 + relative*384 + (int(piece)%6)*64 + transformed)
            accum.append(self.bias + self.w[indices].sum(axis=0, dtype=np.int64))
        return np.asarray(accum, dtype=np.int64)

    def bucket(self, position):
        return min(7, max(0, int(trunc_div((len(position.squares)-2)*8, 32))))

    def components(self, position):
        acc = self.accumulators(position)
        x = np.clip(np.concatenate((acc[position.stm], acc[1-position.stm])), 0, 255)
        bucket = self.bucket(position)
        total = np.sum(x*x*self.linear[bucket], dtype=np.int64)
        linear = int(trunc_div((trunc_div(total, 255)+self.lb[bucket])*400, 255*64))
        residual = 0
        if self.version == 6:
            z = np.clip(trunc_div(self.a @ x + self.hb, 64), 0, 255)
            residual = int(trunc_div((self.rb[bucket] + self.r[bucket] @ z)*400, 255*64))
        return linear, residual

    def evaluate(self, position):
        linear, residual = self.components(position)
        return min(24999, max(-24999, linear+residual))

    def evaluate_fen(self, fen):
        return self.evaluate(parse_fen(fen))

    def evaluate_record(self, record):
        return self.evaluate(decode_record(record))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("network", type=Path)
    parser.add_argument("fens", nargs="+")
    args = parser.parse_args()
    network = Network(args.network.read_bytes())
    for fen in args.fens:
        print(f"{network.evaluate_fen(fen)}\t{fen}")


if __name__ == "__main__":
    main()
