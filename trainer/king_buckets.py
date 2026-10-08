"""NGN's own king layout; square numbers are rank-major from a1."""

import json
from pathlib import Path

import numpy as np


DEFAULT_MAP = tuple(
    (min(file, 7 - file) // 2 + 2 * rank if rank < 2 else
     4 if rank == 2 else 5 if rank == 3 else 6 if rank < 6 else 7)
    for rank in range(8) for file in range(8)
)


def validate_map(mapping) -> tuple[int, ...]:
    if hasattr(mapping, "detach"):
        mapping = mapping.detach().cpu().numpy()
    values = np.asarray(mapping)
    if (values.shape != (64,) or not np.issubdtype(values.dtype, np.integer)
            or np.any(values < 0) or np.any(values > 7)):
        raise ValueError("King map must contain 64 integer bucket IDs in [0,7]")
    return tuple(int(value) for value in values)


def load_map(name: str = "default") -> tuple[int, ...]:
    return DEFAULT_MAP if name == "default" else validate_map(json.loads(Path(name).read_text()))


def feature_indices(position, perspective: int, mapping) -> np.ndarray:
    """CPU reference, independent of the GPU decoder and engine implementation."""
    if perspective not in (0, 1):
        raise ValueError("Perspective must be white=0 or black=1")
    kings = position.squares[position.pieces == 5 + 6 * perspective]
    if len(kings) != 1:
        raise ValueError("NGNN3 requires exactly one own king")
    king = int(kings[0]) ^ (56 * perspective)
    mirror = 7 if king % 8 >= 4 else 0
    bucket = mapping[king ^ mirror]
    relative = (position.pieces // 6 != perspective).astype(np.int64)
    squares = position.squares ^ (56 * perspective) ^ mirror
    return bucket * 768 + relative * 384 + (position.pieces % 6) * 64 + squares
