#!/usr/bin/env python3
"""Exact Maelstrom artifact: independently verify post-draw PV diagnostics."""
import re
import sys
from pathlib import Path

import ownedmatch as base

DEPENDENCY = Path('/home/ehrli/nnue-owned-continuation-20260928/counterdraw-deps/chess-1.11.2')
sys.path.insert(0, str(DEPENDENCY))
import chess
from counter_draw_compat import DrawPVChecker, filter_draw_warnings

CORE_SHA = '1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b'
if chess.__version__ != '1.11.2' or base.sha256(Path(chess.__file__)) != CORE_SHA:
    raise RuntimeError('independent chess validator differs from the pinned dependency')

INFO = re.compile(r'^info depth (\d+) nodes (\d+) time (\d+) score cp (-?\d+) nps (\d+) pv (.+)$')


class MaelstromPVChecker(DrawPVChecker):
    def verify(self, history, info, kind, warning_move):
        match = INFO.fullmatch(info)
        if not match:
            raise ValueError('unrecognized exact Maelstrom info grammar')
        depth, nodes, elapsed, score, nps, pv = match.groups()
        canonical = f'info depth {depth} score cp {score} nodes {nodes} time {elapsed} nps {nps} pv {pv}'
        return super().verify(history, canonical, kind, warning_move)


def filter_maelstrom_warnings(lines, roles):
    return filter_draw_warnings(lines, roles, base.TRACE_RE,
                                artifact_sha256=base.MAELSTROM330_SHA256, checker=MaelstromPVChecker())


original_instrument = base.instrument
original_trace = base.operational_trace


def instrument():
    return dict(original_instrument(), **{
        Path(__file__).name: base.sha256(Path(__file__)),
        'counter_draw_compat.py': base.sha256(Path(__file__).with_name('counter_draw_compat.py')),
        'python_chess_core_1.11.2': CORE_SHA,
    })


def operational_trace(lines, trace_roles, roles):
    filtered, accepted = filter_maelstrom_warnings(lines, roles)
    report = original_trace(filtered, trace_roles, roles)
    report['independently_verified_exact_maelstrom330_post_draw_pvs'] = accepted
    report['unfiltered_trace_lines'] = len(lines)
    return report


base.instrument = instrument
base.operational_trace = operational_trace

if __name__ == '__main__':
    base.main()
