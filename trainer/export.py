"""Quantize float parameters and write little-endian NGNN1/2/3/5 + IEEE CRC32."""

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
HEADER5 = struct.Struct("<4s7I64s4I")
THREAT_FEATURES = 42564


def quantize(value, multiplier: int, dtype: str) -> np.ndarray:
    if hasattr(value, "detach"):
        value = value.detach().cpu().numpy()
    value = np.asarray(value, dtype=np.float64)
    rounded = np.rint(value * multiplier)  # Nearest integer, ties to even.
    bounds = np.iinfo(np.dtype(dtype))
    if not np.all(np.isfinite(rounded)) or np.any((rounded < bounds.min) | (rounded > bounds.max)):
        raise ValueError("Parameter cannot be represented after quantization")
    return rounded.astype(dtype)


def export_bytes(state: dict, format: int | None = None, *, _ngnn6: bool = False) -> bytes:
    if format == 6 or (format is None and "residual_scales" in state):
        from trainer.residual_model import export_model_bytes
        return export_model_bytes(state)
    king_map = validate_map(state["king_map"]) if "king_map" in state else None
    king_buckets = max(king_map) + 1 if king_map is not None else 1
    if _ngnn6:
        king_buckets = 8  # Fixed wire allocation, even when some map IDs are unused.
    transformer = state["w1"]
    if "w1_shared" in state:
        shared = state["w1_shared"]
        if hasattr(transformer, "detach"):
            transformer = transformer.detach().cpu().numpy()
        if hasattr(shared, "detach"):
            shared = shared.detach().cpu().numpy()
        transformer, shared = np.asarray(transformer), np.asarray(shared)
        if (king_map is None or king_buckets < 2 or transformer.ndim != 2
                or transformer.shape[0] != king_buckets * 768
                or shared.shape != (768, transformer.shape[1])
                or transformer.dtype != np.float32 or shared.dtype != np.float32):
            raise ValueError("King factorizer requires float32 W1 [KB*768,H] and shared [768,H]")
        # Match the model's float32 sum, then quantize once. Quantizing the two
        # components separately changes rounding and can overflow int16.
        transformer = (transformer.reshape(king_buckets, *shared.shape) + shared).reshape(transformer.shape)
    w1 = quantize(transformer, QA, "<i2")
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
    format = (5 if "threat_weights" in state else 3 if king_map is not None else 2 if buckets > 1 else 1) if format is None else format
    if (format not in (1, 2, 3, 5) or (format == 1 and buckets != 1)
            or (format in (3, 5)) != (king_map is not None)):
        raise ValueError("NGNN1 requires one output bucket; NGNN3/5 require a king map")
    if format != 5 and "threat_weights" in state:
        raise ValueError("Threat weights require NGNN5 export")
    original_o = state["o"]
    if hasattr(original_o, "detach"):
        original_o = original_o.detach().cpu().numpy()
    if not _ngnn6 and np.any(np.abs(np.asarray(original_o, dtype=np.float64)) > 1.980001):
        raise ValueError("Output float weights must be clipped to [-1.98,1.98]")
    threat_data = b""
    if format == 5:
        if "threat_weights" not in state:
            raise ValueError("NGNN5 requires threat weights")
        threats = quantize(state["threat_weights"], QA, "i1")
        if threats.shape != (king_buckets * THREAT_FEATURES, hidden):
            raise ValueError("Expected threat weights [KB*42564,H]")
        rows = np.flatnonzero(np.any(threats != 0, axis=1))
        data = HEADER5.pack(b"NGN5", 5, hidden, king_buckets, buckets, QA, QB, SCALE,
                            bytes(king_map), THREAT_FEATURES, len(rows), 1, 0)
        threat_data = b"".join(struct.pack("<I", int(row)) + threats[row].tobytes() for row in rows)
    elif format == 3:
        data = HEADER3.pack(b"NGN3", 3, hidden, king_buckets, buckets, QA, QB, SCALE, bytes(king_map))
    else:
        data = (HEADER.pack(b"NGNN", 1, hidden, QA, QB, SCALE) if format == 1 else
                HEADER2.pack(b"NGN2", 2, hidden, buckets, QA, QB, SCALE))
    data += w1.tobytes() + threat_data + b1.tobytes() + o.tobytes() + ob.tobytes()
    data += struct.pack("<I", zlib.crc32(data))
    return data


def export_state(state: dict, path: str | Path, format: int | None = None) -> None:
    data = export_bytes(state, format)
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(path.name + ".tmp")
    temporary.write_bytes(data)
    temporary.replace(path)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("checkpoint")
    parser.add_argument("output")
    parser.add_argument("--format", type=int, choices=(1, 2, 3, 5, 6), help="Default: checkpoint format or infer from parameters")
    args = parser.parse_args()
    import torch
    checkpoint = torch.load(args.checkpoint, map_location="cpu", weights_only=True)
    export_state(checkpoint["model"], args.output, args.format or checkpoint.get("config", {}).get("format"))


if __name__ == "__main__":
    main()
