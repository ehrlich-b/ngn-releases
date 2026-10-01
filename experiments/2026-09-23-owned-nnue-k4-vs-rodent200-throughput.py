#!/usr/bin/env python3
"""Summarize per-role search cost from the audited Fastchess PGN."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import statistics
import tarfile
from pathlib import Path

ARCHIVE_SHA256 = '7262da90d008d39456778628b7aaeddbfc829d7955c9f7f5995d463ca19c4507'
PGN_SHA256 = 'ced360152afd4faa361d7a0a166e719527f805755f4743b00a5bc36f77818e51'
MOVE = re.compile(r'\b([a-h][1-8][a-h][1-8][qrbn]?) \{([^}]*)\}')


def summary(rows: list[dict]) -> dict:
    return {
        'searches': len(rows),
        'total_nodes': sum(row['nodes'] for row in rows),
        'total_search_seconds': sum(row['seconds'] for row in rows),
        'nodes_per_search_second': sum(row['nodes'] for row in rows) / sum(row['seconds'] for row in rows),
        'median_reported_nps': statistics.median(row['nps'] for row in rows),
        'median_depth': statistics.median(row['depth'] for row in rows),
        'median_search_seconds': statistics.median(row['seconds'] for row in rows),
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--archive', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if args.output.exists():
        raise ValueError('refusing existing output')
    if hashlib.sha256(args.archive.read_bytes()).hexdigest() != ARCHIVE_SHA256:
        raise ValueError('match archive SHA-256 mismatch')
    with tarfile.open(args.archive) as archive:
        pgn = archive.extractfile('games.pgn').read()
    if hashlib.sha256(pgn).hexdigest() != PGN_SHA256:
        raise ValueError('match PGN SHA-256 mismatch')
    games = ['[Event "Fastchess Tournament"]' + text for text in pgn.decode().split('[Event "Fastchess Tournament"]') if text.strip()]
    if len(games) != 200:
        raise ValueError(f'expected 200 games, found {len(games)}')
    rows = {'k4': [], 'rodent': []}
    book_plies = 0
    for game in games:
        white = re.search(r'^\[White "([^"]+)"\]$', game, re.MULTILINE).group(1)
        black = re.search(r'^\[Black "([^"]+)"\]$', game, re.MULTILINE).group(1)
        if {'k4' if 'Owned-K4' in white else 'rodent', 'k4' if 'Owned-K4' in black else 'rodent'} != {'k4', 'rodent'}:
            raise ValueError('game roles differ from frozen pair')
        moves = MOVE.findall(game.split('\n\n', 1)[1])
        for ply, (_, comment) in enumerate(moves, 1):
            if comment == 'book':
                book_plies += 1
                continue
            role_name = white if ply % 2 else black
            role = 'k4' if 'Owned-K4' in role_name else 'rodent'
            score_depth = re.match(r'[^/]+/(\d+) ', comment)
            seconds = re.search(r' ([0-9.]+)s,', comment)
            nodes = re.search(r'\bn=(\d+)', comment)
            nps = re.search(r'\bnps=(\d+)', comment)
            if not all((score_depth, seconds, nodes, nps)):
                raise ValueError(f'missing search telemetry at ply {ply}: {comment[:80]}')
            rows[role].append({
                'depth': int(score_depth.group(1)),
                'seconds': float(seconds.group(1)),
                'nodes': int(nodes.group(1)),
                'nps': int(nps.group(1)),
            })
    results = {role: summary(values) for role, values in rows.items()}
    results['k4_to_rodent_nodes_per_search_second'] = (
        results['k4']['nodes_per_search_second'] / results['rodent']['nodes_per_search_second']
    )
    report = {
        'schema': 'ngn-k4-vs-rodent200-throughput-v1',
        'archive_sha256': ARCHIVE_SHA256,
        'pgn_sha256': PGN_SHA256,
        'games': len(games),
        'book_plies': book_plies,
        'roles': results,
    }
    args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
