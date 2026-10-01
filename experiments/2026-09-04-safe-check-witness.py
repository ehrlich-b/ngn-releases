#!/usr/bin/env python3
"""Actual-function overlays: NGN king-zone score vs Chess-3 safe-check features.
No production files are edited. Pinned Chess-3 checkout must exist locally.
This feature witness does not establish an evaluation or playing-strength gain.
"""
from pathlib import Path
import json,os,subprocess
root=Path(__file__).resolve().parents[1]
out=root/'output/recovery-2026-09-04'; out.mkdir(exist_ok=True,parents=True)
ref=root/'output/review-2026-09-04/chess-3'
assert subprocess.check_output(['git','rev-parse','HEAD'],cwd=ref,text=True).strip()=='a33531629cbe82eef6810f982ec814e79d52a3b3'
fens=['6k1/6p1/8/3N4/8/8/R7/1QB2K2 w - - 0 1','6k1/7p/8/3N4/8/8/R7/1QB2K2 w - - 0 1']
fen_literal=',\n'.join(json.dumps(x) for x in fens)+','
ngn=r'''
package engine
import "testing"
func TestReviewSafeChecks(t *testing.T) {
 var got []int
 for _,fen:=range []string{ FENS } {
  pos,err:=ParseFEN(fen); if err!=nil {t.Fatal(err)}
  if isInCheck(pos, Black) {t.Fatal("previous mover left in check")}
  b:=&pos.Board; var a [64]uint64; fillSliderAttacks(b,&a)
  n:=evaluateKingAttackPatterns(&a,b.GetBitboardOf(WhiteQueen),b.GetBitboardOf(WhiteRook),b.GetBitboardOf(WhiteBishop),b.GetBitboardOf(WhiteKnight),trailingZeros(b.GetBitboardOf(BlackKing)),Black)
  t.Logf("NGN zone penalty=%d FEN=%s",n,fen); got=append(got,n)
 }
 if got[0]!=got[1] {t.Fatalf("zone distinction changed: %v",got)}
}
'''.replace('FENS',fen_literal)
ngn_path=out/'safe-check-ngn_test.go.txt'; ngn_path.write_text(ngn)
source=(ref/'eval/king_attacks.go').read_text()
source+='\nvar reviewSafe, reviewUnsafe [2][6]BitBoard\n'
for method,target in [('addSafeChecks','reviewSafe'),('addUnsafeChecks','reviewUnsafe')]:
 needle=f'func (ka *kingAttacks[T]) {method}(color Color, pType Piece, checks BitBoard, c *CoeffSet[T]) {{'
 assert source.count(needle)==1
 source=source.replace(needle,needle+f'\n {target}[color][pType] |= checks')
ref_source=out/'safe-check-chess3-king_attacks.go.txt'; ref_source.write_text(source)
ref_test=r'''
package eval
import (
 "testing"
 "github.com/paulsonkoly/chess-3/board"
 . "github.com/paulsonkoly/chess-3/chess"
)
func TestReviewSafeChecks(t *testing.T) {
 for i,fen:=range []string{ FENS } {
  b,err:=board.FromFEN(fen); if err!=nil {t.Fatal(err)}
  reviewSafe=[2][6]BitBoard{}; reviewUnsafe=[2][6]BitBoard{}
  _=Eval(b,&Coefficients)
  s,u:=reviewSafe[White][Knight],reviewUnsafe[White][Knight]
  t.Logf("Chess-3 white knight safe=%d unsafe=%d safeMask=%016x unsafeMask=%016x FEN=%s",s.Count(),u.Count(),uint64(s),uint64(u),fen)
  if s.Count()!=1+i || u.Count()!=1-i {t.Fatalf("unexpected partition: %d/%d",s.Count(),u.Count())}
 }
}
'''.replace('FENS',fen_literal)
ref_test_path=out/'safe-check-chess3_test.go.txt'; ref_test_path.write_text(ref_test)
env=os.environ.copy(); env.setdefault('GOCACHE','/private/tmp/ngn-go-cache'); env.setdefault('GOMODCACHE','/private/tmp/ngn-go-modcache')
for name,cwd,pkg,replacements in [('ngn',root,'./engine',{str(root/'engine/review_safe_checks_test.go'):str(ngn_path)}),('chess3',ref,'./eval',{str(ref/'eval/king_attacks.go'):str(ref_source),str(ref/'eval/review_safe_checks_test.go'):str(ref_test_path)})]:
 overlay=out/f'safe-check-{name}-overlay.json'; overlay.write_text(json.dumps({'Replace':replacements},indent=2)+'\n')
 p=subprocess.run(['go','test','-overlay',str(overlay),pkg,'-run','^TestReviewSafeChecks$','-count=1','-v'],cwd=cwd,env=env,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=120)
 (out/f'safe-check-{name}-result.txt').write_text(p.stdout); print(p.stdout,end=''); p.check_returncode()
