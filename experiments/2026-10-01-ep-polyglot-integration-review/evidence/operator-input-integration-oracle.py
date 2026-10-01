import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import sys

root = Path('/home/ehrli/ngn-personal-correctness-20261001')
source = root / 'repo/engine/enpassant_repetition_correctness_test.go'
sha = lambda data: hashlib.sha256(data).hexdigest()
assert os.sched_getaffinity(0) == {0, 2} and os.getpriority(os.PRIO_PROCESS, 0) == 10
sys.path.insert(0, '/home/ehrli/nnue-owned-continuation-20260928/counterdraw-deps/chess-1.11.2')
import chess
import chess.polyglot
assert sha(Path(chess.__file__).read_bytes()) == '1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b'
assert sha(Path(chess.polyglot.__file__).read_bytes()) == '8dc20733bdc1297a9e8993a76737ba4f328a99185ae94b311ddbb5342fce32fc'
body = source.read_text().split('var enPassantRepetitionFixtures = []enPassantRepetitionFixture{', 1)[1].split('\nfunc mustParseEnPassantPosition', 1)[0]
rows = []
for block in body.split('\n\t{')[1:]:
    name = re.search(r'name:\s*"([^"]+)"', block).group(1)
    for fen_field, old_field in (('epFEN', 'wantPolyglotEP'), ('noEPFEN', 'wantPolyglotNoEP')):
        fen = re.search(fen_field + r':\s*"([^"]+)"', block).group(1)
        old = re.search(old_field + r':\s*0x([0-9A-Fa-f]+)', block).group(1).lower()
        board = chess.Board(fen)
        assert board.is_valid()
        rows.append({'fixture': name, 'fen_field': fen_field, 'value_field': old_field,
                     'fen': fen, 'legacy_key': old, 'standard_key': f'{chess.polyglot.zobrist_hash(board):016x}',
                     'pseudo_legal_ep': board.has_pseudo_legal_en_passant(), 'legal_ep': board.has_legal_en_passant()})
assert len(rows) == 12 and len({row['fixture'] for row in rows}) == 6
output = {'schema': 'ngn-independent-ep-canonical-book-key-transition-v1',
    'source_fixture_sha256': sha(source.read_bytes()), 'chess_core_sha256': sha(Path(chess.__file__).read_bytes()),
    'polyglot_source_sha256': sha(Path(chess.polyglot.__file__).read_bytes()),
    'scope': 'only12 standard numeric replacements for existing six EP/noEP controls; historical fixtures unchanged', 'rows': rows}
target = root / 'integration-independent-key-transition.json'
assert not target.exists()
target.write_text(json.dumps(output, indent=2) + '\n')
print(json.dumps({'observed_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'source_fixture_sha256': output['source_fixture_sha256'], 'fixture_sha256': sha(target.read_bytes()), 'values': len(rows)}))
