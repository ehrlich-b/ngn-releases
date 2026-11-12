"""Reproduce a small CPU-trained NGNN5 parity fixture from owned records."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import sys

import numpy as np
import torch

from trainer.king_buckets import DEFAULT_MAP
from trainer.model import NGNN
from trainer.ngnp import RECORD_DTYPE, encode_record, parse_fen
from trainer.refeval import Network
from trainer.threats import reference_indices


def generate(output):
    output.mkdir(parents=True, exist_ok=True)
    torch.set_num_threads(2)
    torch.manual_seed(1105)
    fens = json.loads(Path('testdata/ngnn5/positions.json').read_text())['scores']
    records = np.asarray([encode_record(parse_fen(fen), (i-10)*40, i % 3)
                          for i, fen in enumerate(fens)], dtype=RECORD_DTYPE)
    shard = output / 'records.ngnp'
    records.tofile(shard)
    base = NGNN(16, 8, DEFAULT_MAP)
    checkpoint = output / 'ngnn3.pt'
    torch.save({'model': base.state_dict(),
                'config': {'format': 3, 'hidden': 16, 'buckets': 8}}, checkpoint)
    command = [sys.executable, '-m', 'trainer.train', '--data', str(shard),
               '--out', str(output / 'trained'), '--format', '5', '--hidden', '16',
               '--buckets', '8', '--warm-start', str(checkpoint), '--device', 'cpu',
               '--precision', 'fp32', '--threads', '2', '--batch', '10', '--epochs', '3',
               '--decay', 'none', '--lr', '0.02', '--seed', '1105']
    subprocess.run(command, check=True)
    trained = output / 'trained/epoch-0003.nnue'
    net = Network(trained)
    if not net.threat_weights:
        raise RuntimeError('Training did not produce nonzero quantized threats')
    payload = {'format': 'NGNN5', 'hidden': 16, 'king_buckets': 8, 'buckets': 8,
               'network_sha256': hashlib.sha256(trained.read_bytes()).hexdigest(),
               'records_sha256': hashlib.sha256(shard.read_bytes()).hexdigest(),
               'training': {'device': 'cpu', 'epochs': 3, 'updates': 6, 'seed': 1105},
               'cases': [{'fen': fen, 'eval_cp': net.evaluate_fen(fen),
                          'bucket': net.bucket(parse_fen(fen)),
                          'threat_rows': [reference_indices(parse_fen(fen), p, DEFAULT_MAP)
                                          for p in (0, 1)]} for fen in fens]}
    (output / 'evals.json').write_text(json.dumps(payload, indent=2) + '\n')
    print(json.dumps({'python': sys.executable, 'torch': torch.__version__,
                      'nonzero_threat_rows': len(net.threat_weights),
                      'network_sha256': payload['network_sha256']}))


if __name__ == '__main__':
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument('--out', type=Path, required=True)
    generate(cli.parse_args().out)
