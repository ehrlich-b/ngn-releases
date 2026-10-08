"""Report held-out synthetic loss and assert an H=64 smoke learned material."""

import argparse
import json

import numpy as np
import torch

from trainer.model import NGNN, blended_loss, decode_gpu
from trainer.refeval import Network
from trainer.tests.synthetic import random_records


def main() -> None:
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("checkpoint")
    cli.add_argument("network")
    args = cli.parse_args()
    torch.set_num_threads(4)
    saved = torch.load(args.checkpoint, map_location="cpu", weights_only=True)
    model = NGNN(saved["config"]["hidden"])
    model.load_state_dict(saved["model"])
    records = random_records(8192, np.random.default_rng(701))
    features, stm, score, result = decode_gpu(torch.from_numpy(records.view(np.uint8).reshape(-1, 32)))
    with torch.no_grad():
        evaluation = model(features, stm)
        heldout = blended_loss(evaluation, score, result, saved["config"]["wdl"], saved["config"]["k"]).item()
        correlation = float(np.corrcoef(evaluation.numpy(), score.numpy())[0, 1])
    network = Network(args.network)
    fens = [
        "8/8/8/8/8/8/4Q3/4K2k w - - 0 1",
        "8/8/8/8/8/8/4Q3/4K2k b - - 0 1",
        "rnb1kbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
        "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNB1KBNR w KQkq - 0 1",
        "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
    ]
    values = [network.evaluate_fen(fen) for fen in fens]
    output = {"heldout_loss": heldout, "material_correlation": correlation,
              "cases": [{"fen": fen, "eval_cp": value} for fen, value in zip(fens, values)]}
    print(json.dumps(output, indent=2), flush=True)
    history = saved["history"]
    assert history[-1]["loss"] < history[0]["loss"] * 0.5, "Smoke loss did not halve"
    # Sampled WDL labels impose an irreducible MSE floor near 0.009 here.
    assert heldout < 0.012 and correlation > 0.9, "Held-out material fit is weak"
    assert values[0] > 300 and values[1] < -300, "Bare queen material was not learned"
    assert values[2] > 300 and values[3] < -300, "Queen advantage sign was not learned"


if __name__ == "__main__":
    main()
