"""NGNN6 QAT with the specified integer forward and straight-through gradients.

Parameters use the NGNN3 float convention: W/B in units of 1/255,
linear and residual weights in units of 1/64, biases in units of 1/(255*64).
The residual consumes unsquared activations. Rounding is nearest, ties even;
signed divisions truncate toward zero. Float64 holds every integer intermediate
exactly, including the H768 linear sum. Rounding/truncation have identity
surrogates for backward; clipping retains its ordinary derivative.
"""
import struct
import zlib

import numpy as np
import torch
from torch import nn

from trainer.king_buckets import validate_map
from trainer.model import NGNN, QA, QB, SCALE
from trainer.residual_export import lift_bytes, residual_bytes, source_width

RESIDUAL_SCALES = (255, 64, 400, 16, 64, 255, 64, 400)


class _IntegerForward(torch.autograd.Function):
    @staticmethod
    def forward(ctx, value, operation):
        return operation(value)

    @staticmethod
    def backward(ctx, gradient):
        return gradient, None


def _ste(value, operation):
    return _IntegerForward.apply(value, operation)


def _quantized(value, scale):
    return _ste(value.double() * scale, torch.round)


class ResidualNGNN(NGNN):
    """Eight output rows over a shared 16-unit residual bottleneck."""
    def __init__(self, hidden, king_map):
        if hidden not in (32, 768):
            raise ValueError("NGNN6 requires H32/H768")
        super().__init__(hidden, 8, validate_map(king_map))
        # NGNN6 allocates all eight king buckets, including unused map IDs.
        if self.king_buckets != 8:
            self.king_buckets = 8
            self.w1 = nn.Parameter(torch.empty(768 * self.king_buckets, hidden))
            nn.init.normal_(self.w1, mean=0, std=0.02)
        self.a = nn.Parameter(torch.zeros(16, 2 * hidden))
        # Float64 head biases retain every int32 wire value at warm start.
        self.ob = nn.Parameter(self.ob.double())
        self.hb = nn.Parameter(torch.zeros(16, dtype=torch.float64))
        self.r = nn.Parameter(torch.zeros(8, 16))
        self.rb = nn.Parameter(torch.zeros(8, dtype=torch.float64))
        self.register_buffer("residual_scales", torch.tensor(RESIDUAL_SCALES))

    @classmethod
    def from_ngnn3(cls, source):
        """Use the passed deterministic zero lift, retaining all inherited values."""
        return cls.from_zero_lift(lift_bytes(source))

    @classmethod
    def from_zero_lift(cls, data):
        if len(data) < 124 or data[:8] != b"NGN6\x06\x00\x00\x00":
            raise ValueError("Expected NGNN6 zero lift")
        source = b"NGN3" + struct.pack("<I", 3) + data[8:96]
        h = struct.unpack_from("<I", data, 8)[0]
        end = 120 + 12322 * h + 32
        source += data[120:end]
        source += struct.pack("<I", zlib.crc32(source))
        source_width(source)
        if data != lift_bytes(source):
            raise ValueError("Expected the deterministic passed zero lift")
        model = cls(h, tuple(source[32:96]))
        offset = 120
        with torch.no_grad():
            for name, shape, dtype, scale in (
                    ("w1", (6144, h), "<i2", QA), ("b1", (h,), "<i2", QA),
                    ("o", (8, 2*h), "<i2", QB), ("ob", (8,), "<i4", QA*QB),
                    ("a", (16, 2*h), "<i2", QB), ("hb", (16,), "<i4", QA*QB),
                    ("r", (8, 16), "<i2", QB), ("rb", (8,), "<i4", QA*QB)):
                count = int(np.prod(shape))
                values = np.frombuffer(data, dtype=dtype, count=count, offset=offset).copy()
                offset += count * np.dtype(dtype).itemsize
                parameter = getattr(model, name)
                parameter.copy_(torch.from_numpy(values.reshape(shape)).to(parameter.dtype) / scale)
        return model

    def optimizer_groups(self, weight_decay=0.01):
        if not np.isfinite(weight_decay) or weight_decay < 0:
            raise ValueError("Weight decay must be finite and nonnegative")
        return [
            {"params": [self.w1, self.o], "weight_decay": weight_decay},
            {"params": [self.b1, self.ob], "weight_decay": 0.0},
            {"params": [self.a, self.r], "weight_decay": weight_decay},
            {"params": [self.hb, self.rb], "weight_decay": 0.0},
        ]

    def forward(self, features, stm, bucket=None):
        if bucket is None:
            raise ValueError("Material bucket indices are required")
        acc = features.double() @ _quantized(self.w1, QA) + _quantized(self.b1, QA)
        white, black = acc.clamp(0, QA).chunk(2, dim=0)
        us = torch.where((stm == 0)[:, None], white, black)
        them = torch.where((stm == 0)[:, None], black, white)
        x = torch.cat((us, them), dim=1)
        linear = (x.square() * _quantized(self.o, QB)[bucket]).sum(dim=1)
        linear = _ste(linear / QA, torch.trunc) + _quantized(self.ob, QA*QB)[bucket]
        linear = _ste(linear * SCALE / (QA*QB), torch.trunc)
        z = x @ _quantized(self.a, QB).T + _quantized(self.hb, QA*QB)
        z = _ste(z / QB, torch.trunc).clamp(0, QA)
        residual = (z * _quantized(self.r, QB)[bucket]).sum(dim=1)
        residual = residual + _quantized(self.rb, QA*QB)[bucket]
        residual = _ste(residual * SCALE / (QA*QB), torch.trunc)
        return (linear + residual).clamp(-24999, 24999)

    @torch.no_grad()
    def clip_quantized(self):
        self.w1.clamp_(-32768 / QA, 32767 / QA)
        self.b1.clamp_(-32768 / QA, 32767 / QA)
        # NGNN6 inherits the complete int16 linear head, including accepted
        # H768 values outside the older float trainer's [-1.98,1.98] range.
        self.o.clamp_(-32768 / QB, 32767 / QB)
        self.ob.clamp_(-2147483648 / (QA*QB), 2147483647 / (QA*QB))
        for weights, biases in ((self.a, self.hb), (self.r, self.rb)):
            weights.clamp_(-32768 / QB, 32767 / QB)
            # Bias must leave room for every possible clipped input, using
            # the same rounded rows that export validates (not sampled data).
            q = torch.round(weights.double() * QB)
            low = (-2147483648 - QA * q.clamp_max(0).sum(dim=1)) / (QA*QB)
            high = (2147483647 - QA * q.clamp_min(0).sum(dim=1)) / (QA*QB)
            if torch.any(low > high):
                raise ValueError("Residual weights have no valid int32 affine interval")
            # Round bounds inward if the parameter dtype cannot retain them.
            lo, hi = low.to(biases.dtype), high.to(biases.dtype)
            lo = torch.where(lo.double() < low, torch.nextafter(lo, torch.full_like(lo, float('inf'))), lo)
            hi = torch.where(hi.double() > high, torch.nextafter(hi, torch.full_like(hi, -float('inf'))), hi)
            biases.clamp_(lo, hi)


def export_model_bytes(state):
    """Validate fixed scales, quantize once and emit the exact NGNN6 layout."""
    from trainer.export import export_bytes, quantize
    scales = state.get("residual_scales")
    if hasattr(scales, "detach"):
        scales = scales.detach().cpu().numpy()
    if not np.array_equal(scales, RESIDUAL_SCALES):
        raise ValueError("Invalid NGNN6 fixed scales")
    source = export_bytes(state, 3, _ngnn6=True)
    return residual_bytes(source, quantize(state["a"], QB, "<i2"),
                          quantize(state["hb"], QA*QB, "<i4"),
                          quantize(state["r"], QB, "<i2"),
                          quantize(state["rb"], QA*QB, "<i4"))
