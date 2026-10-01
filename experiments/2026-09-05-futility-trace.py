#!/usr/bin/env python3
"""Targeted observation only: trace actual futility skips along frozen SF lines."""
import importlib.util,json,os,subprocess,sys,hashlib,time
from pathlib import Path
sys.dont_write_bytecode=True
ROOT=Path(__file__).resolve().parents[1];OUT=ROOT/'output/recovery-2026-09-04'
spec=importlib.util.spec_from_file_location('study',ROOT/'experiments/2026-09-05-safe-check-study.py');s=importlib.util.module_from_spec(spec);spec.loader.exec_module(s)
s.DEADLINE=time.monotonic()+900
study=json.load(open(OUT/'safe-study-result.json'))
# Preselected five improved cases and all three harmed controls, before tracing.
lines=[105,188,197,194,140,179,138,114]
rows=[g for g in study['games'] if g['game']['line_number'] in lines]
assert len(rows)==8
sf=s.Engine(s.SF);oracle=s.oracle.Stockfish()
selection=[]
try:
 for row in rows:
  line=row['game']['line_number'];prefix=row['prefix'];move=row['ngn']['bestmove']
  reply=sf.analyse(prefix+[move],'go depth 16')
  assert reply['usable']
  states=[];seq=prefix+[move]
  for i in range(min(8,len(reply['pv']))+1):
   fen,check,legal=oracle.position(seq)
   states.append({'fen':fen,'prefix':seq.copy(),'reference_move':reply['pv'][i] if i<len(reply['pv']) else None})
   if i<min(8,len(reply['pv'])):
    assert reply['pv'][i] in legal;seq.append(reply['pv'][i])
  selection.append({'line':line,'root':row,'states':states})
finally:sf.close();oracle.close()
(OUT/'futility-trace-selection.json').write_text(json.dumps(selection,indent=2)+'\n')
if subprocess.check_output(['git','diff','1387fd6','--',':(glob)**/*.go','go.mod','go.sum'],cwd=ROOT):
 raise RuntimeError('Go source differs from the frozen trace parent')
source=(ROOT/'engine/search.go').read_text()
assert source.count('"math"')==1
source=source.replace('"math"','"math"\n "encoding/json"\n "os"')
args='pos, info, move, depth, ply, alpha, beta, staticEval, corrStaticEval, int(ttEval), int(ttDepth), int(ttNodeType), ttHit, improving'
needle='\t\t// Futility pruning - skip quiet moves if position is hopeless (but never prune'
assert source.count(needle)==1
source=source.replace(needle,'\t\treviewFutilityTrace("consider", '+args+')\n'+needle)
needle='\t\t\tinfo.FutilityPrunes++\n\t\t\tcontinue'
assert source.count(needle)==1
source=source.replace(needle,'\t\t\treviewFutilityTrace("futility", '+args+')\n'+needle)
needle='\t\t// MakeMove alone cannot validate castling: its rook relocation can shield an'
assert source.count(needle)==1
source=source.replace(needle,'\t\treviewFutilityTrace("attempt", '+args+')\n'+needle)
source+=r'''

var reviewFutilityWatches map[uint64]bool
var reviewFutilityLog *json.Encoder
func reviewFutilityTrace(event string, pos *Position, info *SearchInfo, move Move, depth, ply, alpha, beta, staticEval, corrected, ttEval, ttDepth, ttType int, ttHit, improving bool) {
 if reviewFutilityWatches==nil {
  reviewFutilityWatches=map[uint64]bool{}
  data,e:=os.ReadFile(os.Getenv("NGN_REVIEW_WATCHES"));if e!=nil {panic(e)}
  var fens []string;if e=json.Unmarshal(data,&fens);e!=nil {panic(e)}
  for _,fen:=range fens {p,e:=ParseFEN(fen);if e!=nil {panic(e)};reviewFutilityWatches[p.Hash()]=true}
  f,e:=os.Create(os.Getenv("NGN_REVIEW_TRACE"));if e!=nil {panic(e)};reviewFutilityLog=json.NewEncoder(f)
 }
 if !reviewFutilityWatches[pos.hash] {return}
 p:=&Position{Board:pos.Board,EnPassant:pos.EnPassant,Tag:pos.Tag,hash:pos.hash,HalfMoveClock:pos.HalfMoveClock}
 legal:=!move.IsCastle() || isLegalCastle(p,move)
 p.MakeMove(move);legal=legal && !isInCheck(p,move.MovingPiece().Color())
 var path []string;for i:=0;i<ply;i++ {path=append(path,info.MoveStack[i].ToString())}
 row:=map[string]any{"event":event,"fen":GenerateFEN(pos),"hash":pos.hash,"move":move.ToString(),"root_depth":info.RootDepth,"depth":depth,"ply":ply,"nodes":info.Nodes,"alpha":alpha,"beta":beta,"static":staticEval,"corrected":corrected,"tt_eval":ttEval,"tt_depth":ttDepth,"tt_type":ttType,"tt_hit":ttHit,"improving":improving,"legal":legal,"gives_check":legal && p.IsInCheck(),"path":path}
 if e:=reviewFutilityLog.Encode(row);e!=nil {panic(e)}
}
'''
code=OUT/'futility-trace-search.go.txt';code.write_text(source)
overlay=OUT/'futility-trace-overlay.json';overlay.write_text(json.dumps({'Replace':{str(ROOT/'engine/search.go'):str(code)}}))
binary=OUT/'futility-trace.bin'
env=os.environ.copy();env.update(GOCACHE='/private/tmp/ngn-go-cache',GOMODCACHE='/private/tmp/ngn-go-modcache')
p=subprocess.run(['go','build','-overlay',str(overlay),'-o',str(binary),'.'],cwd=ROOT,env=env,capture_output=True,text=True,timeout=120)
(OUT/'futility-trace-build.log').write_text(p.stdout+p.stderr);p.check_returncode()
result={'binary_sha256':s.sha(binary),'overlay_source_sha256':s.sha(code),'cases':[]}
for case in selection:
 line=case['line'];watch=OUT/f'futility-trace-watch-{line}.json';watch.write_text(json.dumps([x['fen'] for x in case['states']]))
 trace=OUT/f'futility-trace-{line}.jsonl';os.environ['NGN_REVIEW_WATCHES']=str(watch);os.environ['NGN_REVIEW_TRACE']=str(trace)
 engine=s.Engine(binary)
 try:got=engine.analyse(case['root']['prefix'],'go nodes 400000')
 finally:engine.close()
 base=case['root']['ngn'];fields=['bestmove','kind','value','depth','pv']
 assert all(got[k]==base[k] for k in fields),(line,got,base)
 # The last complete info score record's node count must match too.
 def nodes(x):
  raw=[l.split() for l in x['raw'] if ' score ' in l and ' pv ' in l][-1]
  return int(raw[raw.index('nodes')+1])
 assert nodes(got)==nodes(base)
 events=[json.loads(x) for x in trace.read_text().splitlines()] if trace.exists() else []
 skipped=[x for x in events if x['event']=='futility']
 referenced=[]
 for e in skipped:
  for st in case['states']:
   # Match board/turn/castling/EP; the engine's serializer has no fullmove counter.
   if e['fen'].split()[:4]==st['fen'].split()[:4] and e['move']==st['reference_move']:
    referenced.append(e);break
 result['cases'].append({'line':line,'identity':'PASS','search':got,'trace_rows':len(events),'futility_skips':len(skipped),'reference_move_skips':referenced})
 print(line,'rows',len(events),'skips',len(skipped),'SF-reference-skips',len(referenced),flush=True)
(OUT/'futility-trace-result.json').write_text(json.dumps(result,indent=2)+'\n')
