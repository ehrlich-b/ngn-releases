#!/usr/bin/env python3
"""Separate, fingerprinted instrument for exact Counter draw-PV reporting."""
from __future__ import annotations

import os
import sys
from pathlib import Path

import ownedmatch as base

dependency = Path(os.environ['NGN_COUNTER_DRAW_DEPS']).resolve()
sys.path.insert(0, str(dependency))
import chess
from counter_draw_compat import filter_draw_warnings

CORE_SHA = '1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b'
if chess.__version__ != '1.11.2' or base.sha256(Path(chess.__file__)) != CORE_SHA:
    raise RuntimeError('independent chess validator differs from the frozen dependency')

original_instrument = base.instrument
original_trace = base.operational_trace


def instrument():
    return dict(original_instrument(), **{
        Path(__file__).name: base.sha256(Path(__file__)),
        'counter_draw_compat.py': base.sha256(Path(__file__).with_name('counter_draw_compat.py')),
        'python_chess_core_1.11.2': CORE_SHA,
    })


def operational_trace(lines, trace_roles, roles):
    filtered, accepted = filter_draw_warnings(lines, roles, base.TRACE_RE)
    report = original_trace(filtered, trace_roles, roles)
    report['validated_exact_counter55_post_draw_pvs'] = accepted
    report['unfiltered_trace_lines'] = len(lines)
    return report


base.instrument = instrument
base.operational_trace = operational_trace

if __name__ == '__main__':
    base.main()
