"""Independent integer reference: int64 accumulators, Go-style signed division."""

import argparse
from pathlib import Path
import struct
import zlib

import numpy as np

from trainer.ngnp import Position, decode_record, feature_indices, parse_fen
from trainer.king_buckets import feature_indices as king_features


def trunc_div(numerator: int, denominator: int) -> int:
    if denominator <= 0:
        raise ValueError("Divisor must be positive")
    return numerator // denominator if numerator >= 0 else -((-numerator) // denominator)


class Network:
    def __init__(self, path: str | Path):
        data = Path(path).read_bytes()
        if len(data) < 32:
            raise ValueError("Truncated NGN file")
        magic, version, hidden = struct.unpack_from("<4s2I", data)
        king_buckets, self.king_map = 1, None
        if (magic, version) == (b"NGNN", 1):
            offset, buckets = 24, 1
            qa, qb, scale = struct.unpack_from("<3I", data, 12)
        elif (magic, version) == (b"NGN2", 2):
            offset = 28
            buckets, qa, qb, scale = struct.unpack_from("<4I", data, 12)
        elif (magic, version) == (b"NGN3", 3):
            if len(data) < 96:
                raise ValueError("Truncated NGNN3 header")
            offset = 96
            king_buckets, buckets, qa, qb, scale = struct.unpack_from("<5I", data, 12)
            self.king_map = tuple(data[32:96])
            if not 1 <= king_buckets <= 8 or any(b >= king_buckets for b in self.king_map):
                raise ValueError("Invalid NGNN3 king layout")
        else:
            raise ValueError("Unsupported NGNN magic/version")
        if buckets not in (1, 2, 4, 8):
            raise ValueError("Invalid NGNN2 bucket count")
        if not 16 <= hidden <= 2048 or hidden % 16:
            raise ValueError("Invalid NGNN1 hidden size")
        if (qa, qb, scale) != (255, 64, 400):
            raise ValueError("Invalid NGNN1 quantization constants")
        features = king_buckets * 768
        count = (features + 1) * hidden + buckets * 2 * hidden
        expected = offset + count * 2 + buckets * 4 + 4
        if len(data) != expected:
            raise ValueError("Wrong NGNN1 payload length")
        stored_crc = struct.unpack_from("<I", data, len(data) - 4)[0]
        if zlib.crc32(data[:-4]) != stored_crc:
            raise ValueError("NGNN1 CRC mismatch")
        self.hidden, self.qa, self.qb, self.scale = hidden, qa, qb, scale
        self.version, self.buckets = version, buckets
        self.king_buckets = king_buckets
        # Widen before any arithmetic; valid int16 endpoints cannot overflow.
        weights = np.frombuffer(data, dtype="<i2", count=count, offset=offset).astype(np.int64)
        self.w1 = weights[:features * hidden].reshape(features, hidden)
        self.b1 = weights[features * hidden:(features + 1) * hidden]
        self.o = weights[(features + 1) * hidden:]
        biases = np.frombuffer(data, dtype="<i4", count=buckets, offset=offset + count * 2).astype(np.int64)
        self.ob = int(biases[0]) if buckets == 1 else biases
        if buckets > 1:
            self.o = self.o.reshape(buckets, 2 * hidden)

    def bucket(self, position: Position) -> int:
        return max(0, min(self.buckets - 1, (len(position.squares) - 2) * self.buckets // 32))

    def evaluate(self, position: Position) -> int:
        indices = [feature_indices(position, p) if self.king_map is None else
                   king_features(position, p, self.king_map) for p in (0, 1)]
        acc = [self.b1 + self.w1[index].sum(axis=0, dtype=np.int64) for index in indices]
        us = np.clip(acc[position.stm], 0, self.qa) ** 2
        them = np.clip(acc[1 - position.stm], 0, self.qa) ** 2
        bucket = self.bucket(position)
        weights = self.o if self.buckets == 1 else self.o[bucket]
        bias = self.ob if self.buckets == 1 else int(self.ob[bucket])
        total = int(us @ weights[:self.hidden] + them @ weights[self.hidden:])
        value = trunc_div(total, self.qa) + bias
        cp = trunc_div(value * self.scale, self.qa * self.qb)
        # Preserve the legacy reference's unbounded diagnostic scores.
        return max(-24999, min(24999, cp)) if self.version == 3 else cp

    def evaluate_fen(self, fen: str) -> int:
        return self.evaluate(parse_fen(fen))

    def evaluate_record(self, record: np.void) -> int:
        return self.evaluate(decode_record(record))


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("network")
    parser.add_argument("fens", nargs="*", help="Quoted FENs")
    parser.add_argument("--fen-file", type=Path, help="One FEN per line")
    args = parser.parse_args()
    fens = args.fens + (args.fen_file.read_text().splitlines() if args.fen_file else [])
    if not fens:
        import sys
        fens = sys.stdin.read().splitlines()
    network = Network(args.network)
    for fen in fens:
        if fen.strip():
            print(f"{network.evaluate_fen(fen)}\t{fen}")


if __name__ == "__main__":
    main()
