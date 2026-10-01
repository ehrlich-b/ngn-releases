import copy
import gzip
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import ownedmatch_maelstromdraw as wrapper
import counter_draw_compat


class MaelstromDrawTests(unittest.TestCase):
    def setUp(self):
        self.roles = [{'name': 'Maelstrom', 'sha256': wrapper.base.MAELSTROM330_SHA256}]
        self.history = 'g1f3 g8f6 f3g1 f6g8 g1f3 g8f6 f3g1'
        self.info = 'info depth 5 nodes 99 time 1 score cp 0 nps 99000 pv f6g8 e2e4'
        self.lines = ['[Engine] [00:00:00] <1> Maelstrom <--- position startpos moves ' + self.history,
                      '[Engine] [00:00:00] <1> Maelstrom ---> ' + self.info,
                      '[WARN  ] [00:00:00] <1> fastchess --- Warning; PV continues after threefold repetition - move e2e4 from Maelstrom',
                      'Info; ' + self.info, 'Position; startpos', 'Moves; ' + self.history]

    def test_independently_drawn_boundary_is_accepted_for_the_exact_artifact(self):
        filtered, accepted = wrapper.filter_maelstrom_warnings(self.lines, self.roles)
        self.assertEqual(filtered, self.lines[:2])
        self.assertEqual(len(accepted), 1)
        self.assertEqual(accepted[0]['first_post_draw_index'], 1)
        self.assertTrue(accepted[0]['every_move_legal'])

    def test_foreign_unbound_illegal_and_false_draws_are_rejected(self):
        mutations = []
        roles = copy.deepcopy(self.roles); roles[0]['sha256'] = '0' * 64
        mutations.append(('artifact', self.lines, roles))
        for name in ('actor', 'info-binding', 'root-binding', 'prefix', 'tail', 'boundary', 'draw-kind', 'grammar', 'severity', 'position', 'missing-block', 'terminal-root'):
            lines = list(self.lines)
            if name == 'actor': lines[2] = lines[2].replace('from Maelstrom', 'from Owned')
            if name == 'info-binding': lines[3] = lines[3].replace('score cp 0', 'score cp 1')
            if name == 'root-binding': lines[5] += ' f6g8'
            if name in ('prefix', 'tail'):
                bad = self.info.replace('f6g8' if name == 'prefix' else 'e2e4', 'a1a8')
                lines[1] = lines[1].split(' ---> ')[0] + ' ---> ' + bad; lines[3] = 'Info; ' + bad
            if name == 'boundary': lines[2] = lines[2].replace('move e2e4', 'move d2d4')
            if name == 'draw-kind': lines[2] = lines[2].replace('threefold repetition', 'fifty-move rule')
            if name == 'grammar':
                bad = self.info.replace('score cp 0', 'score mate 0')
                lines[1] = lines[1].split(' ---> ')[0] + ' ---> ' + bad; lines[3] = 'Info; ' + bad
            if name == 'severity': lines[2] = lines[2].replace('[WARN  ]', '[INFO  ]')
            if name == 'position': lines[4] = 'Position; fen invalid'
            if name == 'missing-block': lines.pop()
            if name == 'terminal-root':
                lines[0] += ' f6g8'; lines[5] += ' f6g8'
            mutations.append((name, lines, self.roles))
        for name, lines, roles in mutations:
            with self.subTest(name=name), self.assertRaises(ValueError):
                wrapper.filter_maelstrom_warnings(lines, roles)

    def test_other_warning_classes_remain_visible(self):
        lines = list(self.lines)
        lines[2] = lines[2].replace('PV continues after threefold repetition', 'Illegal PV move')
        filtered, accepted = wrapper.filter_maelstrom_warnings(lines, self.roles)
        self.assertEqual(filtered, lines)
        self.assertEqual(accepted, [])

    def test_default_counter_filter_still_rejects_the_maelstrom_artifact(self):
        with self.assertRaises(ValueError):
            counter_draw_compat.filter_draw_warnings(self.lines, self.roles, wrapper.base.TRACE_RE)

    def test_default_counter_filter_retains_its_positive_boundary_control(self):
        info = 'info depth 5 score cp 0 nodes 99 time 1 nps 99000 pv f6g8 e2e4'
        lines = [line.replace('Maelstrom', 'Counter').replace(self.info, info) for line in self.lines]
        _, accepted = counter_draw_compat.filter_draw_warnings(lines,
            [{'name': 'Counter', 'sha256': counter_draw_compat.COUNTER_SHA}], wrapper.base.TRACE_RE)
        self.assertEqual(len(accepted), 1)

    def test_every_warning_from_the_preserved_failed_trace_is_independently_verified(self):
        path = Path('/home/ehrli/nnue-owned-morning-20260930/native/matches/smoke/fastchess.log.gz')
        with gzip.open(path, 'rt') as source:
            lines = source.read().splitlines()
        roles = [{'name': 'Maelstrom-3.3.0', 'sha256': wrapper.base.MAELSTROM330_SHA256}]
        filtered, accepted = wrapper.filter_maelstrom_warnings(lines, roles)
        self.assertEqual(len(accepted), 31)
        self.assertFalse(any('Warning; PV continues after' in line for line in filtered))
        self.assertTrue(all(row['every_move_legal'] for row in accepted))


if __name__ == '__main__':
    unittest.main()
