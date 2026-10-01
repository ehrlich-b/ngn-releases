#!/usr/bin/env python3
"""Deterministic paired fixed-game report with honest degenerate handling."""
from __future__ import annotations

import math
import random
from collections import Counter
from typing import Iterable

BOOTSTRAP_RESAMPLES = 100_000
BOOTSTRAP_SEED = 0x4E474E4558543236


def elo(score: float) -> float | str:
    if score <= 0:
        return "-infinity"
    if score >= 1:
        return "infinity"
    return -400.0 * math.log10(1.0 / score - 1.0)


def percentile(sorted_values: list[float], quantile: float) -> float:
    if not sorted_values:
        raise ValueError("empty percentile input")
    index = quantile * (len(sorted_values) - 1)
    lower = int(math.floor(index))
    upper = int(math.ceil(index))
    if lower == upper:
        return sorted_values[lower]
    fraction = index - lower
    return sorted_values[lower] * (1.0 - fraction) + sorted_values[upper] * fraction


def report(pair_half_points: Iterable[int]) -> dict:
    values = list(pair_half_points)
    if len(values) != 50 or any(
        isinstance(value, bool) or not isinstance(value, int) or value not in range(5)
        for value in values
    ):
        raise ValueError("external cell requires exactly 50 pair half-point outcomes in [0,4]")
    scores = [value / 4.0 for value in values]
    mean = sum(scores) / len(scores)
    rng = random.Random(BOOTSTRAP_SEED)
    bootstrap = sorted(
        sum(scores[rng.randrange(len(scores))] for _ in scores) / len(scores)
        for _ in range(BOOTSTRAP_RESAMPLES)
    )
    lower = percentile(bootstrap, 0.025)
    upper = percentile(bootstrap, 0.975)
    degenerate = len(set(scores)) == 1
    result = {
        "pairs": len(scores),
        "penta_0_to_4": [Counter(values)[index] for index in range(5)],
        "score": mean,
        "elo": elo(mean),
        "bootstrap": {
            "method": "deterministic-paired-nonparametric-percentile",
            "resamples": BOOTSTRAP_RESAMPLES,
            "seed": BOOTSTRAP_SEED,
            "score_interval_95": [lower, upper],
            "elo_interval_95": [elo(lower), elo(upper)],
            "degenerate": degenerate,
        },
    }
    if degenerate:
        radius = math.sqrt(math.log(40.0) / (2.0 * len(scores)))
        conservative = [max(0.0, mean - radius), min(1.0, mean + radius)]
        result["hoeffding_when_degenerate"] = {
            "method": "bounded-pair-mean-Hoeffding-95",
            "radius": radius,
            "score_interval_95": conservative,
            "elo_interval_95": [elo(conservative[0]), elo(conservative[1])],
            "interpretation": "conservative bound; zero-width bootstrap is not certainty",
        }
    return result
