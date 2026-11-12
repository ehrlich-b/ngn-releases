"""Two deterministic CPU updates of eight owned packed records each."""
from pathlib import Path

import numpy as np
import torch

from trainer.model import decode_gpu
from trainer.residual_model import ResidualNGNN

ROOT = Path(__file__).resolve().parents[2]
PACKET = ROOT / "experiments/wave2-1104/residual-trainer"
SOURCE = ROOT / "experiments/wave2-1104/residual-convert/source-h32.ngn3"


def inputs(model):
    raw = np.frombuffer((PACKET / "records.ngnp").read_bytes(), dtype=np.uint8).copy().reshape(16, 32)
    return decode_gpu(torch.from_numpy(raw), 8, model.king_map, 8)


def train_fixture():
    torch.set_num_threads(2)
    torch.manual_seed(20261105)
    model = ResidualNGNN.from_ngnn3(SOURCE.read_bytes())
    features, stm, _, _, bucket = inputs(model)
    with torch.no_grad():
        initial = model(features, stm, bucket)
    groups = model.optimizer_groups(0.0)
    # Isolate the new head in this tiny diagnostic; all parameters remain in
    # explicit optimizer groups, while inherited groups have a zero step size.
    groups[0]['lr'] = groups[1]['lr'] = 0.0
    optimizer = torch.optim.AdamW(groups, lr=0.02)
    gradients, losses = [], []
    for step in range(2):
        indices = torch.arange(step * 8, (step + 1) * 8)
        perspective = torch.cat((indices, indices + 16))
        optimizer.zero_grad(set_to_none=True)
        evaluation = model(features[perspective], stm[indices], bucket[indices])
        loss = ((evaluation - initial[indices] - 200) / 400).square().mean()
        loss.backward()
        gradients.append({name: float(getattr(model, name).grad.abs().sum())
                          for name in ("a", "hb", "r", "rb")})
        losses.append(float(loss.detach()))
        optimizer.step()
        model.clip_quantized()
    return model, {"seed": 20261105, "batches": 2, "batch_size": 8, "lr": 0.02,
                   "optimizer": "AdamW; explicit inherited/residual weights/biases; inherited lr=0; weight_decay=0",
                   "objective": "MSE cp/400; target initial cp + 200", "losses": losses,
                   "gradients": gradients, "threads": torch.get_num_threads(),
                   "device": str(model.w1.device), "initial_cp": initial.tolist()}
