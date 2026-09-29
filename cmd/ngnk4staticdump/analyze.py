#!/usr/bin/env python3
"""Analyze a receipt-bound K4 static prediction table without changing it."""

import argparse
import csv
import hashlib
import json
from pathlib import Path

import numpy as np
from scipy.special import expit
from scipy.stats import spearmanr


FIELDS = (
    "id", "teacher_cp", "hce_cp", "k4_cp", "rodent_cp", "side",
    "output_head", "white_king_bucket", "black_king_bucket",
    "piece_count", "source_ply",
)
MODELS = ("hce", "k4", "rodent")


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def load(csv_path: Path, receipt_path: Path) -> tuple[dict, dict]:
    receipt = json.loads(receipt_path.read_text())
    if receipt["schema"] != "ngn-k4-static-predictions-v1":
        raise ValueError("wrong predictions receipt schema")
    expected = receipt["predictions"]
    if csv_path.stat().st_size != expected["bytes"] or sha256(csv_path) != expected["sha256"]:
        raise ValueError("predictions CSV does not match its receipt")
    columns = {name: [] for name in FIELDS}
    with csv_path.open(newline="") as source:
        reader = csv.DictReader(source)
        if tuple(reader.fieldnames or ()) != FIELDS:
            raise ValueError("wrong predictions columns")
        for row in reader:
            for name in FIELDS:
                columns[name].append(row[name])
    if len(columns["id"]) != receipt["rows"] or len(set(columns["id"])) != receipt["rows"]:
        raise ValueError("prediction count or IDs do not match receipt")
    numeric = set(FIELDS) - {"id", "side"}
    for name in numeric:
        columns[name] = np.asarray(columns[name], dtype=np.int32)
    columns["side"] = np.asarray(columns["side"])
    if not np.isin(columns["side"], ["w", "b"]).all():
        raise ValueError("invalid side in predictions")
    if not np.isin(columns["output_head"], np.arange(8)).all():
        raise ValueError("invalid output head")
    return columns, receipt


def huber_fit(teacher: np.ndarray, predicted: np.ndarray) -> dict:
    if len(teacher) < 3 or np.std(teacher) < 1e-9:
        return {"intercept_cp": None, "slope": None}
    design = np.column_stack((np.ones(len(teacher)), teacher / 400.0))
    coeff = np.linalg.lstsq(design, predicted, rcond=None)[0]
    for _ in range(8):
        residual = predicted - design @ coeff
        median = np.median(residual)
        scale = max(25.0, 1.4826 * np.median(np.abs(residual - median)))
        cutoff = 1.345 * scale
        weight = np.minimum(1.0, cutoff / np.maximum(np.abs(residual), 1e-12))
        root = np.sqrt(weight)
        coeff = np.linalg.lstsq(design * root[:, None], predicted * root, rcond=None)[0]
    return {"intercept_cp": float(coeff[0]), "slope": float(coeff[1] / 400.0)}


def metrics(teacher: np.ndarray, predicted: np.ndarray) -> dict:
    if len(teacher) == 0:
        return {"n": 0}
    difference = predicted - teacher
    answer = {
        "n": int(len(teacher)),
        "mae_cp": float(np.mean(np.abs(difference))),
        "rmse_cp": float(np.sqrt(np.mean(difference * difference))),
        "bias_cp": float(np.mean(difference)),
        "wdl_mse": float(np.mean((expit(predicted / 400) - expit(teacher / 400)) ** 2)),
        "pearson": float(np.corrcoef(teacher, predicted)[0, 1])
            if len(teacher) > 1 and np.std(teacher) > 0 and np.std(predicted) > 0 else None,
        "spearman": float(spearmanr(teacher, predicted).statistic)
            if len(teacher) > 1 and np.std(teacher) > 0 and np.std(predicted) > 0 else None,
    }
    for threshold in (50, 100):
        eligible = np.abs(teacher) >= threshold
        answer[f"sign_n_{threshold}"] = int(np.count_nonzero(eligible))
        answer[f"sign_accuracy_{threshold}"] = (
            float(np.mean(np.sign(predicted[eligible]) == np.sign(teacher[eligible])))
            if np.any(eligible) else None
        )
    answer["huber_predicted_on_teacher"] = huber_fit(teacher, predicted)
    return answer


def paired_bootstrap(teacher: np.ndarray, k4: np.ndarray, other: np.ndarray, seed: int) -> dict:
    if len(teacher) < 20:
        return {"mae_delta_cp": None, "position_bootstrap_95ci_cp": None}
    delta = np.abs(k4 - teacher) - np.abs(other - teacher)
    random = np.random.default_rng(seed)
    draws = np.empty(400)
    for index in range(len(draws)):
        draws[index] = float(np.mean(delta[random.integers(0, len(delta), len(delta))]))
    return {
        "mae_delta_cp": float(np.mean(delta)),
        "position_bootstrap_95ci_cp": [float(x) for x in np.percentile(draws, [2.5, 97.5])],
    }


def group_report(data: dict, mask: np.ndarray, bootstrap: bool, seed: int) -> dict:
    teacher = data["teacher_cp"][mask].astype(np.float64)
    answer = {"n": int(len(teacher))}
    for name in MODELS:
        answer[name] = metrics(teacher, data[f"{name}_cp"][mask].astype(np.float64))
    if bootstrap:
        k4 = data["k4_cp"][mask].astype(np.float64)
        answer["k4_minus_hce"] = paired_bootstrap(
            teacher, k4, data["hce_cp"][mask].astype(np.float64), seed
        )
        answer["k4_minus_rodent"] = paired_bootstrap(
            teacher, k4, data["rodent_cp"][mask].astype(np.float64), seed + 1
        )
    return answer


def score_band(score: np.ndarray) -> np.ndarray:
    edges = np.array([-800, -400, -200, -100, -50, 50, 100, 200, 400, 800])
    return np.searchsorted(edges, score, side="right")


def analyze(data: dict, receipt: dict) -> dict:
    n = receipt["rows"]
    if n != 100_000:
        raise ValueError(f"expected 100000 calibration rows, got {n}")
    groups = {}
    all_rows = np.ones(n, dtype=bool)
    groups["overall"] = {"all": group_report(data, all_rows, True, 260921)}
    groups["side"] = {
        side: group_report(data, data["side"] == side, False, 260922)
        for side in ("w", "b")
    }
    groups["output_head"] = {
        str(head): group_report(data, data["output_head"] == head, True, 260930 + head * 2)
        for head in range(8)
    }
    pairs = 4 * data["white_king_bucket"] + data["black_king_bucket"]
    groups["king_bucket_pair"] = {
        f"{pair // 4}-{pair % 4}": group_report(data, pairs == pair, False, 260950 + pair)
        for pair in range(16)
    }
    groups["piece_count"] = {
        str(count): group_report(data, data["piece_count"] == count, False, 260980 + count)
        for count in range(2, 33)
    }
    ply_edges = np.array([20, 40, 80, 120])
    ply_bins = np.searchsorted(ply_edges, data["source_ply"], side="right")
    groups["source_ply_band"] = {
        label: group_report(data, ply_bins == index, False, 261020 + index)
        for index, label in enumerate(("0-19", "20-39", "40-79", "80-119", "120+"))
    }
    bands = score_band(data["teacher_cp"])
    band_names = ("<-800", "-800:-401", "-400:-201", "-200:-101", "-100:-51",
                  "-50:49", "50:99", "100:199", "200:399", "400:799", "800+")
    groups["teacher_score_band"] = {
        label: group_report(data, bands == index, False, 261040 + index)
        for index, label in enumerate(band_names)
    }
    teacher = data["teacher_cp"]
    k4_error = np.abs(data["k4_cp"] - teacher)
    excess = k4_error - np.abs(data["hce_cp"] - teacher)
    worst = {}
    for name, value in (("k4_absolute_error", k4_error), ("k4_excess_error_over_hce", excess)):
        indices = np.argsort(value)[-20:][::-1]
        worst[name] = [
            {"id": data["id"][int(i)], "teacher_cp": int(teacher[i]),
             "k4_cp": int(data["k4_cp"][i]), "hce_cp": int(data["hce_cp"][i]),
             "rodent_cp": int(data["rodent_cp"][i]), "output_head": int(data["output_head"][i]),
             "white_king_bucket": int(data["white_king_bucket"][i]),
             "black_king_bucket": int(data["black_king_bucket"][i])}
            for i in indices
        ]
    return {
        "schema": "ngn-k4-static-calibration-analysis-v1",
        "predictions_sha256": receipt["predictions"]["sha256"],
        "method": {
            "huber": "8 IRLS steps, predicted_cp ~ intercept + slope * teacher_cp; Huber cutoff 1.345 * max(25 cp, MAD scale)",
            "bootstrap": "400 paired position resamples, deterministic seeds; chain correlation makes intervals optimistic",
            "wdl": "natural sigmoid(score_cp / 400), matching the frozen training target",
        },
        "groups": groups,
        "worst": worst,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--csv", required=True, type=Path)
    parser.add_argument("--receipt", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    data, receipt = load(args.csv, args.receipt)
    result = analyze(data, receipt)
    with args.output.open("x") as destination:
        json.dump(result, destination, indent=2, allow_nan=False)
        destination.write("\n")
    overall = result["groups"]["overall"]["all"]
    print(json.dumps({name: overall[name] for name in MODELS} | {
        "k4_minus_hce": overall["k4_minus_hce"],
        "k4_minus_rodent": overall["k4_minus_rodent"],
    }, indent=2))


if __name__ == "__main__":
    main()
