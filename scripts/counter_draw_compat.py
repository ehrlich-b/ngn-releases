"""Validate the exact Counter release's PV continuations beyond claimed draws.

The exception is restricted to a witnessed Counter info record at the exact
root history. Every historical/PV move must be legal and the reported warning
must identify the first move after the independently established draw boundary.
Played moves, clocks, stderr and all other warning classes remain audited.
"""
from __future__ import annotations

import re

import chess

COUNTER_SHA = '6c48fb52934d49d3774633e32f0b4fb4796b1e2c63925f24167ef0f0c0d761c8'
WARNING = re.compile(r'^Warning; PV continues after (fifty-move rule|threefold repetition) - move ([a-h][1-8][a-h][1-8][nbrq]?) from (.+)$')
INFO = re.compile(r'^info depth \d+ score cp -?\d+ nodes \d+ time \d+ nps \d+ pv (.+)$')


class DrawPVChecker:
    def __init__(self):
        self.roots = {}
        self.checked = {}

    @staticmethod
    def draw(board):
        reasons = []
        if board.halfmove_clock >= 100 and any(board.legal_moves):
            reasons.append('fifty-move rule')
        if board.is_repetition(3):
            reasons.append('threefold repetition')
        if board.is_insufficient_material():
            reasons.append('insufficient material')
        return reasons

    def verify(self, history, info, kind, warning_move):
        match = INFO.fullmatch(info)
        if not match:
            raise ValueError('unrecognized Counter info grammar')
        key = (tuple(history), info, kind, warning_move)
        if key in self.checked:
            return self.checked[key]
        root_key = tuple(history)
        if root_key not in self.roots:
            board = chess.Board()
            for token in history:
                board.push_uci(token)
            if self.draw(board):
                raise ValueError('root is already terminal under the host draw rules')
            self.roots[root_key] = board
        board = self.roots[root_key].copy(stack=True)
        moves = match.group(1).split()
        boundary = None
        for index, token in enumerate(moves):
            reasons = self.draw(board)
            if reasons and boundary is None:
                boundary = {'index': index, 'reasons': reasons, 'fen': board.fen()}
            board.push_uci(token)  # Also validates every move in the post-draw tail.
        if boundary is None or kind not in boundary['reasons'] or moves[boundary['index']] != warning_move:
            raise ValueError('warning does not match the first independently drawn PV boundary')
        result = {'kind': kind, 'first_post_draw_index': boundary['index'], 'first_post_draw_move': warning_move,
                  'boundary_fen': boundary['fen'], 'pv_plies': len(moves), 'history_plies': len(history),
                  'every_move_legal': True}
        self.checked[key] = result
        return result


def filter_draw_warnings(lines, roles, trace_re, *, artifact_sha256=COUNTER_SHA, checker=None):
    allowed = {role['name'] for role in roles if role['sha256'] == artifact_sha256}
    checker = checker if checker is not None else DrawPVChecker()
    live = {}
    filtered, accepted = [], []
    index = 0
    while index < len(lines):
        line = lines[index]
        trace = trace_re.fullmatch(line)
        body = trace.group('body').strip() if trace else ''
        thread = trace.group('thread').strip() if trace else ''
        if ' <--- ' in body:
            actor, payload = body.split(' <--- ', 1)
            if actor in allowed and payload.startswith('position startpos moves '):
                live[(actor, thread)] = {'history': tuple(payload.removeprefix('position startpos moves ').split()), 'infos': set()}
        if ' ---> ' in body:
            actor, payload = body.split(' ---> ', 1)
            if actor in allowed and (actor, thread) in live and payload.startswith('info depth '):
                live[(actor, thread)]['infos'].add(payload)
        warning = WARNING.fullmatch(body.removeprefix('fastchess --- '))
        if warning:
            kind, move, actor = warning.groups()
            if actor not in allowed or not trace or not body.startswith('fastchess --- ') or trace.group('level').strip().upper() != 'WARN':
                raise ValueError('draw warning is not from the exact witnessed Counter role')
            if index + 3 >= len(lines) or not lines[index+1].startswith('Info; ') or lines[index+2] != 'Position; startpos' or not lines[index+3].startswith('Moves; '):
                raise ValueError('malformed draw-warning diagnostic block')
            info = lines[index+1].removeprefix('Info; ')
            history = lines[index+3].removeprefix('Moves; ').split()
            if not any(name == actor and row['history'] == tuple(history) and info in row['infos']
                       for (name, _), row in live.items()):
                raise ValueError('warning is not bound to a recorded Counter info/root command')
            result = checker.verify(history, info, kind, move)
            accepted.append(dict(result, line=index+1, role=actor, info=info))
            index += 4
            continue
        filtered.append(line)
        index += 1
    return filtered, accepted
