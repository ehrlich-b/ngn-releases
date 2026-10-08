"""Quantize float parameters and write little-endian NGNN1/2/3 + IEEE CRC32."""

import argparse
from pathlib import Path
import struct
import zlib

import numpy as np

from trainer.king_buckets import validate_map

QA, QB, SCALE = 255, 64, 400
HEADER = struct.Struct("<4s5I")
HEADER2 = struct.Struct("<4s6I")
HEADER3 = struct.Struct("<4s7I64s")


def quantize(value, multiplier: int, dtype: str) -> np.ndarray:
    if hasattr(value, "detach"):
        value = value.detach().cpu().numpy()
    value = np.asarray(value, dtype=np.float64)
    rounded = np.rint(value * multiplier)  # Nearest integer, ties to even.
    bounds = np.iinfo(np.dtype(dtype))
    if not np.all(np.isfinite(rounded)) or np.any((rounded < bounds.min) | (rounded > bounds.max)):
        raise ValueError("Parameter cannot be represented after quantization")
    return rounded.astype(dtype)


def export_state(state: dict, path: str | Path, format: int | None = None) -> None:
    w1 = quantize(state["w1"], QA, "<i2")
    king_map = validate_map(state["king_map"]) if "king_map" in state else None
    king_buckets = max(king_map) + 1 if king_map is not None else 1
    hidden = w1.shape[1] if w1.ndim == 2 else 0
    if w1.shape != (768 * king_buckets, hidden) or not 16 <= hidden <= 2048 or hidden % 16:
        raise ValueError("Expected W1 [KB*768,H], H a multiple of 16 in [16,2048]")
    b1 = quantize(state["b1"], QA, "<i2")
    o = quantize(state["o"], QB, "<i2")
    ob = quantize(state["ob"], QA * QB, "<i4")
    buckets = 1 if o.ndim == 1 else o.shape[0] if o.ndim == 2 else 0
    expected_o = (2 * hidden,) if o.ndim == 1 else (buckets, 2 * hidden)
    expected_ob = () if o.ndim == 1 else (buckets,)
    if buckets not in (1, 2, 4, 8) or b1.shape != (hidden,) or o.shape != expected_o or ob.shape != expected_ob:
        raise ValueError("Invalid NGN parameter shape or bucket count")
    format = (3 if king_map is not None else 2 if buckets > 1 else 1) if format is None else format
    if (format not in (1, 2, 3) or (format == 1 and buckets != 1)
            or (format == 3) != (king_map is not None)):
        raise ValueError("NGNN1 requires one output bucket; only NGNN3 accepts a king map")
    original_o = state["o"]
    if hasattr(original_o, "detach"):
        original_o = original_o.detach().cpu().numpy()
    if np.any(np.abs(np.asarray(original_o, dtype=np.float64)) > 1.980001):
        raise ValueError("Output float weights must be clipped to [-1.98,1.98]")
    if format == 3:
        data = HEADER3.pack(b"NGN3", 3, hidden, king_buckets, buckets, QA, QB, SCALE, bytes(king_map))
    else:
        data = (HEADER.pack(b"NGNN", 1, hidden, QA, QB, SCALE) if format == 1 else
                HEADER2.pack(b"NGN2", 2, hidden, buckets, QA, QB, SCALE))
    data += w1.tobytes() + b1.tobytes() + o.tobytes() + ob.tobytes()
    data += struct.pack("<I", zlib.crc32(data))
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(path.name + ".tmp")
    temporary.write_bytes(data)
    temporary.replace(path)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("checkpoint")
    parser.add_argument("output")
    parser.add_argument("--format", type=int, choices=(1, 2, 3), help="Default: checkpoint format or infer from parameters")
    args = parser.parse_args()
    import torch
    checkpoint = torch.load(args.checkpoint, map_location="cpu", weights_only=True)
    export_state(checkpoint["model"], args.output, args.format or checkpoint.get("config", {}).get("format"))


if __name__ == "__main__":
    main()
