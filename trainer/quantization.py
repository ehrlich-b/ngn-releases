"""Export-exact NGNN3 fake quantization with straight-through derivatives.

Parameters stay float32. Integer-valued float64 intermediates preserve the
exporter's widening-before-rounding and exact integer sums (including on CPU
under autocast). Rounding and signed truncation have identity derivatives;
SCReLU and the final cp clamp retain their ordinary clipping derivatives.
"""

import torch

from trainer.export import QA, QB, SCALE


class _Round(torch.autograd.Function):
    @staticmethod
    def forward(ctx, value):
        return torch.round(value)  # Nearest integer, ties to even.

    @staticmethod
    def backward(ctx, gradient):
        return gradient


class _Truncate(torch.autograd.Function):
    @staticmethod
    def forward(ctx, value):
        return torch.trunc(value)  # Signed division truncates toward zero.

    @staticmethod
    def backward(ctx, gradient):
        return gradient


def fake_quantize(value: torch.Tensor, multiplier: int, dtype: torch.dtype) -> torch.Tensor:
    """Return export integer units with an STE, rejecting unexportable values.

    Bounds are checked after rounding, exactly as in export.quantize. Do not
    silently clamp parameters here: clip_quantized already supplies the training
    bounds, and export rejects nonfinite or overflowing parameters.
    """
    if value.dtype != torch.float32:
        raise ValueError("Fake quantization requires float32 parameters")
    rounded = _Round.apply(value.to(torch.float64) * multiplier)
    bounds = torch.iinfo(dtype)
    if not torch.all(torch.isfinite(rounded) & (rounded >= bounds.min) & (rounded <= bounds.max)):
        raise ValueError("Parameter cannot be represented after quantization")
    return rounded


def trunc_div(value: torch.Tensor, divisor: int) -> torch.Tensor:
    """Go-style signed division in forward; derivative 1/divisor in backward."""
    if divisor <= 0:
        raise ValueError("Divisor must be positive")
    return _Truncate.apply(value / divisor)


def quantized_forward(model, features: torch.Tensor, stm: torch.Tensor,
                      bucket: torch.Tensor | None = None) -> torch.Tensor:
    """Evaluate the unchanged NGNN3 export arithmetic with surrogate gradients."""
    if model.king_map is None:
        raise ValueError("Fake quantization requires NGNN3 king inputs")
    if model.buckets > 1 and bucket is None:
        raise ValueError("Material bucket indices are required")
    # Match export's float32 factorizer merge before widening and rounding once.
    # Disable mixed precision so autocast cannot change that merge or the sums.
    with torch.autocast(device_type=features.device.type, enabled=False):
        w1 = fake_quantize(model.effective_w1(), QA, torch.int16)
        b1 = fake_quantize(model.b1, QA, torch.int16)
        if torch.any(model.o.detach().to(torch.float64).abs() > 1.980001):
            raise ValueError("Output float weights must be clipped to [-1.98,1.98]")
        o = fake_quantize(model.o, QB, torch.int16)
        ob = fake_quantize(model.ob, QA * QB, torch.int32)
        acc = features.to(torch.float64) @ w1 + b1
        white, black = acc.clamp(0, QA).square().chunk(2, dim=0)
        us = torch.where((stm == 0)[:, None], white, black)
        them = torch.where((stm == 0)[:, None], black, white)
        weights = o if model.buckets == 1 else o[bucket]
        bias = ob if model.buckets == 1 else ob[bucket]
        total = (us * weights[..., :model.hidden] + them * weights[..., model.hidden:]).sum(dim=1)
        value = trunc_div(total, QA) + bias
        cp = trunc_div(value * SCALE, QA * QB)
        return cp.clamp(-24999, 24999).float()
