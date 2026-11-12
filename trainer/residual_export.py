"""Byte-exact NGNN3 KB8/B8 -> NGNN6 lift; no float requantization.

The default head uses PCG64(seed=20261104), drawing A in [-8,8] and
HB in [-1024,1024], in that order as int64 then casting to wire types.
R and RB are both zero. Only the NGN-owned input supplied by the caller is
used. See docs/ngnn6.md for the fixed wire layout and integer scales.
"""

import argparse
import hashlib
import json
from pathlib import Path
import struct
import zlib

import numpy as np

SEED = 20261104


def source_width(data: bytes) -> int:
    if len(data) < 100:
        raise ValueError("Truncated NGNN3")
    magic, version, h, kb, nb, qa, qb, scale = struct.unpack_from("<4s7I", data)
    if (magic, version, kb, nb, qa, qb, scale) != (b"NGN3", 3, 8, 8, 255, 64, 400):
        raise ValueError("Expected NGNN3 KB8/B8 with fixed scales")
    if h not in (32, 768) or any(b >= 8 for b in data[32:96]):
        raise ValueError("Unsupported width or king map")
    if len(data) != 12322 * h + 132:
        raise ValueError("Wrong NGNN3 length")
    if zlib.crc32(data[:-4]) != struct.unpack_from("<I", data, len(data)-4)[0]:
        raise ValueError("NGNN3 CRC mismatch")
    return h


def _integer_array(value, shape, dtype):
    value = np.asarray(value)
    limits = np.iinfo(dtype)
    if (value.shape != shape or not np.issubdtype(value.dtype, np.integer)
            or np.any(value < limits.min) or np.any(value > limits.max)):
        raise ValueError(f"Expected integer {dtype} array of shape {shape}")
    return value.astype(dtype)


def _affine_bound(weights, biases):
    w = weights.astype(np.int64)
    lo = biases.astype(np.int64) + 255 * np.minimum(w, 0).sum(axis=1)
    hi = biases.astype(np.int64) + 255 * np.maximum(w, 0).sum(axis=1)
    if np.any(lo < -(2**31)) or np.any(hi > 2**31-1):
        raise ValueError("Residual affine range exceeds int32")


def residual_bytes(source: bytes, a, hb, r, rb) -> bytes:
    """Export supplied integer residual arrays, retaining the source payload."""
    h = source_width(source)
    a = _integer_array(a, (16, 2*h), "<i2")
    hb = _integer_array(hb, (16,), "<i4")
    r = _integer_array(r, (8, 16), "<i2")
    rb = _integer_array(rb, (8,), "<i4")
    _affine_bound(a, hb)
    _affine_bound(r, rb)
    header = b"NGN6" + struct.pack("<I", 6) + source[8:96]
    header += struct.pack("<6I", 16, 64, 255, 64, 400, 0)
    data = header + source[96:-4] + a.tobytes() + hb.tobytes() + r.tobytes() + rb.tobytes()
    return data + struct.pack("<I", zlib.crc32(data))


def lift_bytes(source: bytes, seed: int = SEED) -> bytes:
    h = source_width(source)
    rng = np.random.Generator(np.random.PCG64(seed))
    a = rng.integers(-8, 9, size=(16, 2*h), dtype=np.int64)
    hb = rng.integers(-1024, 1025, size=16, dtype=np.int64)
    return residual_bytes(source, a, hb, np.zeros((8, 16), dtype=np.int16),
                          np.zeros(8, dtype=np.int32))


def receipt(source: bytes, output: bytes, seed: int = SEED) -> dict:
    h = source_width(source)
    if output != lift_bytes(source, seed):
        raise ValueError("Output is not the deterministic zero lift")
    def metadata(data, fmt, header):
        return {"format": fmt, "sha256": hashlib.sha256(data).hexdigest(),
                "file_bytes": len(data), "payload_bytes": len(data)-header-4,
                "crc32": f"{zlib.crc32(data[:-4]):08x}"}
    return {"input": metadata(source, "NGNN3", 96), "output": metadata(output, "NGNN6", 120),
            "hidden": h, "seed": seed, "rng": "PCG64; int64 draws A then HB",
            "preserved_payload_bytes": len(source)-100, "preserved_king_map_bytes": 64,
            "zero_projection": True,
            "claim": "Integer identity only; no loss, speed or strength claim. A trained nonzero model must pass the same independent oracle."}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--seed", type=int, default=SEED)
    parser.add_argument("--receipt", type=Path)
    args = parser.parse_args()
    source = args.source.read_bytes()
    output = lift_bytes(source, args.seed)
    args.output.write_bytes(output)
    result = json.dumps(receipt(source, output, args.seed), indent=2) + "\n"
    if args.receipt:
        args.receipt.write_text(result)
    print(result, end="")


if __name__ == "__main__":
    main()
