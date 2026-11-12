"""Bounded NGNN3 integer diagnostics on an already selected validation stream.

Float predictions are pre-export, STM-relative cp in exactly the record order.
This module needs NumPy, not torch, and never chooses a split or checkpoint.
"""

import argparse
from dataclasses import dataclass
from itertools import islice
import json
import math
from pathlib import Path

import numpy as np

from trainer.ngnp import RECORD_DTYPE, decode_record
from trainer.refeval import Network


def sigmoid(value: float) -> float:
    # Avoid exp overflow for saturated engine scores or extreme teacher labels.
    exp = math.exp(-abs(value))
    return 1 / (1 + exp) if value >= 0 else exp / (1 + exp)


@dataclass
class _Totals:
    count: int = 0
    float_count: int = 0
    integer_loss_sum: float = 0.0
    paired_integer_loss_sum: float = 0.0
    float_loss_sum: float = 0.0
    loss_difference_sum: float = 0.0
    cp_error_sum: float = 0.0
    cp_absolute_error_sum: float = 0.0
    cp_squared_error_sum: float = 0.0
    cp_max_absolute_error: float = 0.0

    def add(self, integer_cp, float_cp, target, k):
        integer_loss = (sigmoid(integer_cp / k) - target) ** 2
        self.count += 1
        self.integer_loss_sum += integer_loss
        if float_cp is None:
            return
        float_loss = (sigmoid(float_cp / k) - target) ** 2
        error = integer_cp - float_cp
        self.float_count += 1
        self.paired_integer_loss_sum += integer_loss
        self.float_loss_sum += float_loss
        self.loss_difference_sum += integer_loss - float_loss
        self.cp_error_sum += error
        self.cp_absolute_error_sum += abs(error)
        self.cp_squared_error_sum += error ** 2
        self.cp_max_absolute_error = max(self.cp_max_absolute_error, abs(error))

    def report(self):
        n = self.float_count
        return {
            "count": self.count,
            "float_count": n,
            "missing_float_count": self.count - n,
            "integer_loss_sum": self.integer_loss_sum if self.count else None,
            "integer_loss": self.integer_loss_sum / self.count if self.count else None,
            "paired_integer_loss_sum": self.paired_integer_loss_sum if n else None,
            "paired_integer_loss": self.paired_integer_loss_sum / n if n else None,
            "float_loss_sum": self.float_loss_sum if n else None,
            "float_loss": self.float_loss_sum / n if n else None,
            "loss_difference_sum": self.loss_difference_sum if n else None,
            "loss_difference": self.loss_difference_sum / n if n else None,
            "cp_error_sum": self.cp_error_sum if n else None,
            "cp_error": self.cp_error_sum / n if n else None,
            "cp_absolute_error_sum": self.cp_absolute_error_sum if n else None,
            "cp_mean_absolute_error": self.cp_absolute_error_sum / n if n else None,
            "cp_squared_error_sum": self.cp_squared_error_sum if n else None,
            "cp_root_mean_squared_error": math.sqrt(self.cp_squared_error_sum / n) if n else None,
            "cp_max_absolute_error": self.cp_max_absolute_error if n else None,
        }


def evaluate_stream(network: Network, records, *, max_records: int,
                    float_predictions=None, lam: float = 0.75, k: float = 400) -> dict:
    """Evaluate at most max_records from an iterable of NumPy NGNP1 records.

    Supply the existing validation partition/order, with the same flag/ply
    filters and objective as float validation. No shuffling or filtering occurs
    here. Float values may be None; an absent/exhausted prediction stream is
    unknown, never an integer-derived replacement. Only paired rows contribute
    to float/cp/difference metrics; integer loss includes every processed row.
    Memory is O(network size + output buckets), independent of record count.
    The cap does not consume a lookahead record or prediction.
    """
    if network.version != 3:
        raise ValueError("Quantized validation requires an NGNN3 export")
    if type(max_records) is not int or max_records < 1:
        raise ValueError("max_records must be a positive integer")
    if not math.isfinite(lam) or not 0 <= lam <= 1 or not math.isfinite(k) or k <= 0:
        raise ValueError("Require finite lambda in [0,1] and positive finite k")
    predictions = iter(()) if float_predictions is None else iter(float_predictions)
    overall = _Totals()
    buckets = [_Totals() for _ in range(network.buckets)]
    for record in islice(records, max_records):
        position = decode_record(record)
        integer_cp = network.evaluate(position)
        float_cp = next(predictions, None)
        if float_cp is not None:
            float_cp = float(float_cp)
            if not math.isfinite(float_cp):
                raise ValueError("Float predictions must be finite cp or None")
        # NGNP labels are White-relative; predictions and loss are STM-relative.
        score = int(record["score"]) * (1 - 2 * position.stm)
        result = int(record["result"]) / 2
        if position.stm:
            result = 1 - result
        target = lam * sigmoid(score / k) + (1 - lam) * result
        overall.add(integer_cp, float_cp, target, k)
        buckets[network.bucket(position)].add(integer_cp, float_cp, target, k)
    return {
        "schema": "ngnn3-quantized-validation-1",
        "diagnostic_only": True,
        "network": {"format": 3, "hidden": network.hidden,
                    "king_buckets": network.king_buckets, "output_buckets": network.buckets,
                    "qa": network.qa, "qb": network.qb, "score_scale": network.scale},
        "objective": {"lambda": lam, "k": k, "prediction_perspective": "stm",
                      "cp_error": "integer minus float",
                      "loss_difference": "integer minus float on paired records"},
        "max_records": max_records,
        "overall": overall.report(),
        "buckets": [dict(bucket=b, **totals.report()) for b, totals in enumerate(buckets)],
    }


def record_stream(path: Path):
    """Read one raw NGNP1 record at a time, without indexing the entire shard."""
    with Path(path).open("rb") as source:
        while raw := source.read(RECORD_DTYPE.itemsize):
            if len(raw) != RECORD_DTYPE.itemsize:
                raise ValueError("Truncated NGNP1 record")
            yield np.frombuffer(raw, dtype=RECORD_DTYPE)[0]


def prediction_stream(path: Path):
    """One JSON number or null per line; blank lines cannot change alignment."""
    with Path(path).open() as source:
        for line in source:
            value = json.loads(line)
            if value is not None and type(value) not in (int, float):
                raise ValueError("Prediction lines must contain a cp number or null")
            yield value


def main() -> None:
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("--network", type=Path, required=True)
    cli.add_argument("--records", type=Path, required=True,
                     help="Already selected held-out NGNP1 records in validation order")
    cli.add_argument("--float-predictions", type=Path,
                     help="JSONL STM cp from the same float checkpoint; null means unknown")
    cli.add_argument("--max-records", type=int, required=True)
    cli.add_argument("--lambda", dest="lam", type=float, default=0.75)
    cli.add_argument("--k", type=float, default=400)
    cli.add_argument("--out", type=Path, help="Default: write JSON to stdout")
    args = cli.parse_args()
    records = record_stream(args.records)
    predictions = prediction_stream(args.float_predictions) if args.float_predictions else None
    try:
        report = evaluate_stream(Network(args.network), records, max_records=args.max_records,
                                 float_predictions=predictions, lam=args.lam, k=args.k)
    finally:
        records.close()
        if predictions is not None:
            predictions.close()
    output = json.dumps(report, indent=2, allow_nan=False) + "\n"
    if args.out:
        args.out.write_text(output)
    else:
        print(output, end="")


if __name__ == "__main__":
    main()
