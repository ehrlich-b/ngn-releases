"""Exercise the scalar NGNN5 reference in a NumPy-only environment."""
import json
from pathlib import Path
import unittest

from trainer.ngnp import parse_fen
from trainer.refeval import Network
from trainer.threats import reference_indices


class ScalarThreatTest(unittest.TestCase):
    def test_reviewed_rows_and_scores(self):
        root = Path('testdata/ngnn5')
        cases = json.loads((root / 'trained_h16_k8_b8_evals.json').read_text())['cases']
        network = Network(root / 'trained_h16_k8_b8.nnue')
        for case in cases:
            with self.subTest(fen=case['fen']):
                position = parse_fen(case['fen'])
                self.assertEqual([reference_indices(position, p, network.king_map) for p in (0, 1)],
                                 case['threat_rows'])
                self.assertEqual(network.evaluate(position), case['eval_cp'])


if __name__ == '__main__':
    unittest.main()
