"""Directed occupied attacks in NGNN5's perspective-relative row order."""
from functools import lru_cache


THREAT_FEATURES = 42564


def geometry(kind, source, target):
    dr, df = target // 8 - source // 8, target % 8 - source % 8
    ar, af = abs(dr), abs(df)
    if source == target:
        return False
    return (0 < source // 8 < 7 and dr == 1 and af == 1,
            ar * af == 2, ar == af, dr == 0 or df == 0,
            dr == 0 or df == 0 or ar == af, ar <= 1 and af <= 1)[kind]


@lru_cache(maxsize=4)
def tables(device):
    import torch

    layout = torch.full((6, 64, 12, 64), -1, dtype=torch.int64)
    row = 0
    for kind in range(6):
        for source in range(64):
            for victim in range(12):
                for target in range(64):
                    if (geometry(kind, source, target)
                            and not (victim % 6 == 0 and target // 8 in (0, 7))
                            and not (kind == 5 and victim % 6 == 5)):
                        layout[kind, source, victim, target] = row
                        row += 1
    assert row == THREAT_FEATURES
    between = torch.zeros((64, 64), dtype=torch.int64)
    for source in range(64):
        for target in range(64):
            if not geometry(4, source, target):
                continue
            dr, df = target // 8 - source // 8, target % 8 - source % 8
            step = (8 if dr > 0 else -8 if dr < 0 else 0) + (1 if df > 0 else -1 if df < 0 else 0)
            mask = 0
            square = source + step
            while square != target:
                mask |= 1 << square
                square += step
            between[source, target] = mask
    return layout.to(device), between.to(device)


def decode_threats(board, occupied, occupancy, king_map):
    """Return packed row indices and bag offsets, without a dense threat matrix.

    At most 32 occupied squares participate. Every attacker is assigned to its
    own color's bag; slider rays include the first occupied target (defenses too).
    All batch-dependent operations stay on the input device.
    """
    import torch

    batch = board.shape[0]
    layout, between = tables(board.device)
    squares = torch.arange(64, device=board.device).expand(batch, -1)
    packed = torch.where(occupied.bool(), squares, 64).sort(dim=1).values[:, :32]
    present = packed < 64
    packed = packed.clamp_max(63)
    codes = board.gather(1, packed)
    color = codes // 6
    kings = torch.stack([((board == k) & occupied.bool()).long().argmax(1)
                         for k in (5, 11)], dim=1)
    kings[:, 1] ^= 56
    mirrors = torch.where((kings & 7) >= 4, 7, 0)
    buckets = king_map[kings ^ mirrors]
    flip = color * 56 + mirrors.gather(1, color)
    source = packed ^ flip
    target = packed[:, None, :] ^ flip[:, :, None]
    victim = codes[:, None, :] % 6 + (color[:, None, :] != color[:, :, None]).long() * 6
    rows = layout[codes[:, :, None] % 6, source[:, :, None], victim, target]
    blocked = (between[packed[:, :, None], packed[:, None, :]] & occupancy.reshape(batch, 1, 1)) != 0
    active = (rows >= 0) & present[:, :, None] & present[:, None, :]
    active &= ~blocked | ~((codes % 6 >= 2) & (codes % 6 <= 4))[:, :, None]
    rows = rows + buckets.gather(1, color)[:, :, None] * THREAT_FEATURES
    owners = torch.arange(batch, device=board.device)[:, None].expand(-1, 32) + color * batch
    owners = owners[:, :, None].expand(-1, -1, 32)[active]
    order = owners.argsort(stable=True)
    indices = rows[active][order]
    counts = torch.bincount(owners, minlength=2 * batch)
    offsets = torch.cat((counts.new_zeros(1), counts.cumsum(0)))
    return indices, offsets


@lru_cache(maxsize=1)
def scalar_layout():
    """Map valid directed attacks to rows without requiring tensor libraries."""
    layout = {}
    for kind in range(6):
        for source in range(64):
            for victim in range(12):
                for target in range(64):
                    if (geometry(kind, source, target)
                            and not (victim % 6 == 0 and target // 8 in (0, 7))
                            and not (kind == 5 and victim % 6 == 5)):
                        layout[kind, source, victim, target] = len(layout)
    assert len(layout) == THREAT_FEATURES
    return layout


def reference_indices(position, perspective, mapping):
    """Scalar board/ray reference, independent of the packed tensor decoder."""
    layout = scalar_layout()
    board = dict(zip(map(int, position.squares), map(int, position.pieces)))
    king = next(s for s, c in board.items() if c == 5 + 6 * perspective) ^ (56 * perspective)
    flip = 56 * perspective ^ (7 if king % 8 >= 4 else 0)
    bucket = mapping[king ^ (7 if king % 8 >= 4 else 0)]
    rows = []
    for source, code in board.items():
        if code // 6 != perspective:
            continue
        for target, victim in board.items():
            row = layout.get((code % 6, source ^ flip,
                              victim % 6 + 6 * (victim // 6 != perspective), target ^ flip), -1)
            if row < 0:
                continue
            if code % 6 in (2, 3, 4):
                dr, df = target // 8 - source // 8, target % 8 - source % 8
                step = (8 if dr > 0 else -8 if dr < 0 else 0) + (1 if df > 0 else -1 if df < 0 else 0)
                if any(s in board for s in range(source + step, target, step)):
                    continue
            rows.append(bucket * THREAT_FEATURES + row)
    return sorted(rows)
