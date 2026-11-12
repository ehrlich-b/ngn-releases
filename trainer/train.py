"""Train NGNN1/2/3/5/6 on NGNP1 shards or a direct archive; checkpoint/export each epoch."""

import argparse
import hashlib
from concurrent.futures import ThreadPoolExecutor
import json
import math
from pathlib import Path
import time

import numpy as np
import torch

from trainer.export import export_state
from trainer.model import NGNN, blended_loss, decode_gpu, validate_buckets, validate_hidden, warm_start_ngnn3
from trainer.ngnp import Shards
from trainer.ngnp_split import ContentShards
from trainer.archive import Archive
from trainer.king_buckets import load_map


def epoch_batches(shards: Shards, batch: int, positions: int, seed: int):
    """Superbatches can span multiple shard passes; never discard the final tail."""
    seen, cycle = 0, 0
    while seen < positions:
        for raw in shards.batches(batch, seed + cycle):
            take = min(len(raw), positions - seen)
            if isinstance(shards, Archive):
                shards.record_exposure(take)
            yield raw[:take]
            seen += take
            if seen == positions:
                return
        cycle += 1


def prefetched(batches):
    """Overlap one CPU batch with GPU execution, with bounded memory/one thread."""
    iterator = iter(batches)
    with ThreadPoolExecutor(max_workers=1) as pool:
        future = pool.submit(next, iterator, None)
        while (raw := future.result()) is not None:
            future = pool.submit(next, iterator, None)
            yield raw


@torch.no_grad()
def validation_metrics(model, shards, batch, device, lam, k):
    """Visit every held-out record, including the tail, without updating weights."""
    model.eval()
    lambdas = (lam, 0.75, 0.0)
    total = torch.zeros(3, device=device, dtype=torch.float64)
    seen = 0
    for raw_numpy in shards.batches(batch, 0, shuffle=False):
        raw = torch.from_numpy(np.ascontiguousarray(raw_numpy)).to(device)
        features, stm, score, result, bucket = decode_gpu(
            raw, model.buckets, model.king_map, model.king_buckets, model.threat_weights is not None)
        evaluation = model(features, stm, bucket)
        for i, objective_lambda in enumerate(lambdas):
            total[i] += blended_loss(evaluation, score, result, objective_lambda, k).double() * len(raw_numpy)
        seen += len(raw_numpy)
    model.train()
    losses = (total / seen).tolist()
    if not all(math.isfinite(loss) for loss in losses):
        raise RuntimeError("Non-finite validation loss; checkpoint not written")
    return dict(zip(("validation_loss", "validation_loss_lambda_075", "validation_loss_result"), losses))


def validation_loss(model, shards, batch, device, lam, k):
    return validation_metrics(model, shards, batch, device, lam, k)["validation_loss"]


def learning_rate(args, step: int, total: int) -> float:
    if args.decay == "step":
        return args.lr * args.gamma ** (step // args.decay_steps)
    if args.decay == "cosine":
        fraction = min(step / max(total - 1, 1), 1)
        return args.lr * (args.min_lr_ratio + (1 - args.min_lr_ratio) * 0.5 * (1 + math.cos(math.pi * fraction)))
    return args.lr


def write_history(path, history):
    temporary = path.with_suffix(".json.tmp")
    temporary.write_text(json.dumps(history, indent=2, allow_nan=False) + "\n")
    temporary.replace(path)


def parser() -> argparse.ArgumentParser:
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("--data", nargs="+", required=True, help="One or more quoted shard globs")
    cli.add_argument("--data-format", choices=("ngnp", "archive"), default="ngnp")
    cli.add_argument("--archive-holdout-modulus", type=int, default=1024,
                     help="Archive board-content hash holdout fraction 1/N; N is a power of two")
    cli.add_argument("--archive-split-seed", type=int,
                     help="Archive content partition seed; default is --seed")
    cli.add_argument("--ngnp-holdout-modulus", type=int, default=0,
                     help="Optional NGNP occupied-board holdout fraction 1/N; 0 keeps shard validation")
    cli.add_argument("--ngnp-split-seed", type=int,
                     help="NGNP content partition seed; default is --seed")
    cli.add_argument("--validation-data", nargs="+", help="Disjoint held-out shard globs; never optimized")
    cli.add_argument("--record-counts", type=Path,
                     help="JSON mapping of every training shard path to its frozen prefix record count")
    cli.add_argument("--out", type=Path, required=True)
    cli.add_argument("--hidden", type=int, default=256)
    cli.add_argument("--buckets", type=int, choices=(1, 2, 4, 8), default=1)
    cli.add_argument("--format", type=int, choices=(1, 2, 3, 5, 6), help="Default: 1 for NB=1, otherwise 2")
    cli.add_argument("--residual-source", type=Path,
                     help="NGNN6: owned NGNN3 KB8/B8 zero-lift source (required)")
    cli.add_argument("--kb-map", default="default", help="NGNN3/5: default or JSON file with 64 bucket IDs")
    cli.add_argument("--king-factorizer", action="store_true",
                     help="NGNN3/5: learn shared feature rows plus king-bucket residuals; fold at export")
    cli.add_argument("--fake-quantize", action="store_true",
                     help="NGNN3: export-exact forward arithmetic with straight-through gradients")
    cli.add_argument("--batch", type=int, default=16384)
    cli.add_argument("--lr", type=float, default=0.001)
    cli.add_argument("--epochs", "--superbatches", dest="epochs", type=int, default=10)
    cli.add_argument("--positions-per-epoch", "--positions-per-superbatch", type=int, default=0,
                     help="0 = one filtered data pass; larger values cycle the data")
    cli.add_argument("--decay", choices=("cosine", "step", "none"), default="cosine")
    cli.add_argument("--decay-steps", type=int, default=1000)
    cli.add_argument("--gamma", type=float, default=0.5)
    cli.add_argument("--min-lr-ratio", type=float, default=0.1)
    cli.add_argument("--wdl", type=float, default=0.75, help="Score sigmoid blend lambda")
    cli.add_argument("--k", type=float, default=400)
    cli.add_argument("--weight-decay", type=float, default=0.01)
    cli.add_argument("--drop-flags", type=lambda s: int(s, 0), default=0x0F)
    cli.add_argument("--min-ply", type=int, default=0)
    cli.add_argument("--block-records", type=int, default=262144)
    cli.add_argument("--seed", type=int, default=20261005)
    cli.add_argument("--device", default="cuda")
    cli.add_argument("--threads", type=int, default=4)
    cli.add_argument("--precision", choices=("fp32", "bf16"), default="bf16")
    restart = cli.add_mutually_exclusive_group()
    restart.add_argument("--resume", type=Path)
    restart.add_argument("--warm-start", type=Path, help="Initialize NGNN5 from NGNN3; reset optimizer and schedule")
    cli.add_argument("--keep-best-only", action="store_true",
                     help="Retain latest and best checkpoints/exports instead of every epoch")
    return cli


def main() -> None:
    args = parser().parse_args()
    validate_hidden(args.hidden)
    validate_buckets(args.buckets)
    args.format = args.format or (2 if args.buckets > 1 else 1)
    if args.format == 1 and args.buckets != 1:
        raise ValueError("NGNN1 requires --buckets 1")
    if args.format not in (3, 5, 6) and args.kb_map != "default":
        raise ValueError("--kb-map requires --format 3 or 5")
    residual_model = None
    if args.format == 6:
        from trainer.residual_model import ResidualNGNN
        if (args.residual_source is None or args.king_factorizer or args.kb_map != "default"
                or args.buckets != 8 or args.precision != "fp32"):
            raise ValueError("NGNN6 requires --residual-source, --buckets 8, --precision fp32 and no map/factorizer override")
        source = args.residual_source.read_bytes()
        residual_model = ResidualNGNN.from_ngnn3(source)
        if args.hidden != residual_model.hidden:
            raise ValueError("NGNN6 hidden width differs from its source")
        args.residual_source_sha256 = hashlib.sha256(source).hexdigest()
    elif args.residual_source is not None:
        raise ValueError("--residual-source requires --format 6")
    king_map = (residual_model.king_map.tolist() if residual_model is not None else
                load_map(args.kb_map) if args.format in (3, 5, 6) else None)
    if args.king_factorizer and (king_map is None or max(king_map) < 1):
        raise ValueError("King factorizer requires NGNN3 with at least two king buckets")
    if args.fake_quantize and args.format != 3:
        raise ValueError("Fake quantization requires --format 3")
    if (args.batch < 1 or args.epochs < 1 or args.threads < 1 or args.lr <= 0 or args.k <= 0
            or not 0 <= args.wdl <= 1 or args.positions_per_epoch < 0 or args.weight_decay < 0
            or args.decay_steps < 1 or not 0 < args.gamma <= 1 or not 0 <= args.min_lr_ratio <= 1):
        raise ValueError("Invalid training options")
    torch.set_num_threads(args.threads)
    torch.set_num_interop_threads(1)
    torch.set_float32_matmul_precision("high")
    # Checkpoint continuation must preserve floating point updates exactly.
    if torch.device(args.device).type == "cpu":
        torch.use_deterministic_algorithms(True)
    torch.manual_seed(args.seed)
    device = torch.device(args.device)
    if device.type == "cuda" and not torch.cuda.is_available():
        raise RuntimeError("CUDA is unavailable; use --device cpu explicitly for tests")
    use_amp = args.precision == "bf16" and device.type == "cuda"
    record_counts = json.loads(args.record_counts.read_text()) if args.record_counts else None
    if args.data_format == "archive":
        if (args.record_counts or args.validation_data or args.min_ply or args.drop_flags != 0x0F
                or args.ngnp_holdout_modulus or args.ngnp_split_seed is not None):
            raise ValueError("Archive uses its own content holdout and lacks NGNP flags/ply; "
                             "--record-counts, --validation-data, NGNP split options and custom filters are unsupported")
        args.archive_split_seed = args.seed if args.archive_split_seed is None else args.archive_split_seed
        shards = Archive(args.data, args.block_records, args.archive_split_seed, args.archive_holdout_modulus)
        validation = shards.holdout()
    else:
        if args.ngnp_holdout_modulus:
            if args.validation_data:
                raise ValueError("NGNP content holdout cannot be combined with --validation-data")
            args.ngnp_split_seed = args.seed if args.ngnp_split_seed is None else args.ngnp_split_seed
            shards = ContentShards(args.data, args.drop_flags, args.min_ply, args.block_records,
                                   args.format in (3, 5, 6), record_counts,
                                   args.ngnp_holdout_modulus, args.ngnp_split_seed)
            validation = shards.holdout()
        else:
            if args.ngnp_split_seed is not None:
                raise ValueError("--ngnp-split-seed requires --ngnp-holdout-modulus")
            shards = Shards(args.data, args.drop_flags, args.min_ply, args.block_records, args.format in (3, 5, 6), record_counts)
            validation = (Shards(args.validation_data, args.drop_flags, args.min_ply, args.block_records, args.format in (3, 5, 6))
                          if args.validation_data else None)
    if record_counts is not None:
        record_counts = {str(Path(p).resolve()): n for p, n in record_counts.items()}
    if args.data_format == "ngnp" and not args.ngnp_holdout_modulus and validation and set(map(lambda p: Path(p).resolve(), shards.paths)) & set(
            map(lambda p: Path(p).resolve(), validation.paths)):
        raise ValueError("Training and validation shards overlap")
    positions = args.positions_per_epoch or shards.count
    # epoch_batches may have partial batches at pass boundaries.
    steps_per_pass = math.ceil(shards.count / args.batch)
    passes, remainder = divmod(positions, shards.count)
    steps_per_epoch = passes * steps_per_pass + math.ceil(remainder / args.batch)
    total_steps = args.epochs * steps_per_epoch
    model = (residual_model or NGNN(args.hidden, args.buckets, king_map, args.king_factorizer,
                                    threats=args.format == 5, fake_quantize=args.fake_quantize)).to(device)
    if args.warm_start:
        warm_start_ngnn3(model, torch.load(args.warm_start, map_location=device, weights_only=True))
    decayed = [model.w1, model.o]
    if getattr(model, "threat_weights", None) is not None:
        decayed.append(model.threat_weights)
    if model.w1_shared is not None:
        decayed.append(model.w1_shared)
    groups = [
        {"params": decayed, "weight_decay": args.weight_decay},
        {"params": [model.b1, model.ob], "weight_decay": 0.0},
    ]
    if residual_model is not None:
        groups = model.optimizer_groups(args.weight_decay)
    optimizer = torch.optim.AdamW(groups, lr=args.lr, fused=device.type == "cuda")
    first_epoch, step, history = 0, 0, []
    if args.resume:
        checkpoint = torch.load(args.resume, map_location=device, weights_only=True)
        saved = checkpoint["config"]
        # Restoring with changed objective, data, batching, or schedule is not a continuation.
        same = ("hidden", "batch", "lr", "decay", "decay_steps", "gamma", "min_lr_ratio", "wdl", "k",
                "weight_decay", "drop_flags", "min_ply", "block_records", "positions_per_epoch",
                "seed", "precision", "data")
        differences = [key for key in same if saved[key] != getattr(args, key)]
        for key, default in (("buckets", 1), ("format", 1), ("king_factorizer", False),
                             ("fake_quantize", False)):
            if saved.get(key, default) != getattr(args, key):
                differences.append(key)
        if saved.get("data_format", "ngnp") != args.data_format:
            differences.append("data_format")
        if args.data_format == "archive":
            for key in ("archive_holdout_modulus", "archive_split_seed"):
                if saved.get(key) != getattr(args, key):
                    differences.append(key)
            if saved.get("archive_split") != shards.split_receipt:
                differences.append("archive_split")
        if args.data_format == "ngnp":
            for key, default in (("ngnp_holdout_modulus", 0), ("ngnp_split_seed", None)):
                if saved.get(key, default) != getattr(args, key):
                    differences.append(key)
            if args.ngnp_holdout_modulus and saved.get("ngnp_split") != shards.split_receipt:
                differences.append("ngnp_split")
        if args.format == 6 and saved.get("residual_source_sha256") != args.residual_source_sha256:
            differences.append("residual_source_sha256")
        if args.format in (3, 5, 6):
            if saved.get("king_map") != list(king_map):
                differences.append("king_map")
        if saved.get("validation_data") != args.validation_data:
            differences.append("validation_data")
        if saved.get("shard_record_counts") != record_counts:
            differences.append("shard_record_counts")
        if differences:
            raise ValueError("Resume options differ: " + ", ".join(differences))
        if checkpoint["filtered_records"] != shards.count or checkpoint["raw_records"] != shards.raw_count:
            raise ValueError("Resume data record counts differ")
        if saved["epochs"] != args.epochs and args.decay == "cosine":
            raise ValueError("Cosine resume requires the original total --epochs")
        model.load_state_dict(checkpoint["model"])
        optimizer.load_state_dict(checkpoint["optimizer"])
        torch.set_rng_state(checkpoint["torch_rng"].cpu())
        if device.type == "cuda" and checkpoint["cuda_rng"]:
            torch.cuda.set_rng_state_all([state.cpu() for state in checkpoint["cuda_rng"]])
        first_epoch, step, history = checkpoint["epoch"], checkpoint["step"], checkpoint["history"]
        if isinstance(shards, Archive) and history:
            previous_exposure = history[-1].get("source_presentations_cumulative", {})
            shards.exposure_counts = [previous_exposure.get(path, 0) for path in shards.source_paths]
    config = vars(args).copy()
    if king_map is not None:
        config["king_map"] = list(king_map)
    config["out"], config["resume"] = str(args.out), str(args.resume) if args.resume else None
    config["residual_source"] = str(args.residual_source) if args.residual_source else None
    config["warm_start"] = str(args.warm_start) if args.warm_start else None
    config["record_counts"] = str(args.record_counts) if args.record_counts else None
    config["shard_record_counts"] = record_counts
    if args.data_format == "archive":
        config["archive_split"] = shards.split_receipt
    if args.data_format == "ngnp" and args.ngnp_holdout_modulus:
        config["ngnp_split"] = shards.split_receipt
    args.out.mkdir(parents=True, exist_ok=True)
    (args.out / "config.json").write_text(json.dumps(config, indent=2) + "\n")
    # A crash can leave the final atomic checkpoint ahead of its history file.
    # Restore this metadata even when resume has no further epochs to execute.
    write_history(args.out / "history.json", history)

    def objective(raw):
        features, stm, score, result, bucket = decode_gpu(
            raw, args.buckets, model.king_map, model.king_buckets, model.threat_weights is not None)
        return blended_loss(model(features, stm, bucket), score, result, args.wdl, args.k)

    print(json.dumps({"device": str(device), "torch": torch.__version__, "raw_records": shards.raw_count,
                      "filtered_records": shards.count, "positions_per_epoch": positions,
                      "validation_records": validation.count if validation else 0,
                      "steps_per_epoch": steps_per_epoch, "config": config}), flush=True)
    model.train()
    for epoch in range(first_epoch, args.epochs):
        if device.type == "cuda":
            torch.cuda.synchronize()
        started = time.perf_counter()
        losses = torch.zeros((), device=device)
        seen, updates = 0, 0
        warmup_positions, steady_started = 0, None
        for raw_numpy in prefetched(epoch_batches(shards, args.batch, positions, args.seed + epoch * 1000003)):
            # Input is only 32 bytes/position; never transfer dense features over PCIe.
            raw = torch.from_numpy(np.ascontiguousarray(raw_numpy))
            if device.type == "cuda":
                raw = raw.pin_memory()
            raw = raw.to(device, non_blocking=True)
            lr = learning_rate(args, step, total_steps)
            for group in optimizer.param_groups:
                group["lr"] = lr
            optimizer.zero_grad(set_to_none=True)
            with torch.autocast(device_type=device.type, dtype=torch.bfloat16, enabled=use_amp):
                loss = objective(raw)
            loss.backward()
            optimizer.step()
            model.clip_quantized()
            losses += loss.detach() * len(raw_numpy)
            seen += len(raw_numpy)
            step, updates = step + 1, updates + 1
            if updates == 10:
                if device.type == "cuda":
                    torch.cuda.synchronize()
                steady_started = time.perf_counter()
                warmup_positions = seen
        if device.type == "cuda":
            torch.cuda.synchronize()
        finished = time.perf_counter()
        mean_loss = losses.item() / seen
        if not math.isfinite(mean_loss):
            raise RuntimeError("Non-finite training loss; checkpoint not written")
        row = {"epoch": epoch + 1, "step": step, "positions": seen, "updates": updates,
               "loss": mean_loss, "lr": lr, "seconds": finished - started,
               "positions_per_second": seen / (finished - started),
               "steady_positions_per_second": ((seen - warmup_positions) / (finished - steady_started)
                                               if steady_started and seen > warmup_positions else None)}
        if isinstance(shards, Archive):
            row["source_presentations_cumulative"] = dict(zip(shards.source_paths, shards.exposure_counts))
        if validation:
            row.update(validation_metrics(model, validation, args.batch, device, args.wdl, args.k))
        selection_key = "validation_loss" if validation else "loss"
        improved = not history or row[selection_key] < min(previous[selection_key] for previous in history)
        history.append(row)
        print(json.dumps(row), flush=True)
        checkpoint = {"format": "NGN-trainer-1", "model": model.state_dict(), "optimizer": optimizer.state_dict(),
                      "epoch": epoch + 1, "step": step, "history": history, "config": config,
                      "raw_records": shards.raw_count, "filtered_records": shards.count,
                      "torch_rng": torch.get_rng_state(),
                      "cuda_rng": torch.cuda.get_rng_state_all() if device.type == "cuda" else []}
        stem = "latest" if args.keep_best_only else f"epoch-{epoch + 1:04d}"
        destination = args.out / f"{stem}.pt"
        temporary = destination.with_suffix(".pt.tmp")
        torch.save(checkpoint, temporary)
        temporary.replace(destination)
        export_state(model.state_dict(), args.out / f"{stem}.nnue", args.format)
        if args.keep_best_only and improved:
            best = args.out / "best.pt"
            temporary = best.with_suffix(".pt.tmp")
            torch.save(checkpoint, temporary)
            temporary.replace(best)
            export_state(model.state_dict(), args.out / "best.nnue", args.format)
        write_history(args.out / "history.json", history)


if __name__ == "__main__":
    main()
