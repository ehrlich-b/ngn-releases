#!/usr/bin/env python3
"""Reproducible 16-case qcap/Futility-off diagnostic; writes output only."""
import argparse, hashlib, importlib.util, json, os, select, subprocess, sys, time
from pathlib import Path

sys.dont_write_bytecode=True
ROOT=Path(__file__).resolve().parents[1]; OUT=ROOT/'output/recovery-2026-09-04'
STUDY=OUT/'safe-study-result.json'; PRIOR=OUT/'search-ablation-selection-2026-09-05.json'
SEL=OUT/'futility-controls-selection-2026-09-05.json'; RESULT=OUT/'futility-controls-result-2026-09-05.json'
BASE=ROOT/'build/ngn_20260904_qcap'; FUT=OUT/'search-ablation-Futility.bin'; SF=Path('/opt/homebrew/bin/stockfish')
BASE_SHA='24d71a1ac381320d2ab790b1a88a7fdb30761bcedf573f9b98973b5a1e7eb702'
FUT_SHA='ba23e023683abcf429b1f30d70d478bb0b0ad00aa7e4fade1f033c730181bfbd'
DEADLINE=None
def sha(p): return hashlib.sha256(Path(p).read_bytes()).hexdigest()
def save(p,x): Path(p).write_text(json.dumps(x,indent=2)+'\n')
def chk():
 if time.monotonic()>DEADLINE: raise TimeoutError('15-minute diagnostic cap reached')
class Engine:
 def __init__(self,path,label):
  self.label=label; self.p=subprocess.Popen([str(path)],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,bufsize=0); self.b=b''
  self.send('uci'); self.until('uciok'); self.send('setoption name Hash value 64'); self.send('setoption name Threads value 1'); self.send('isready'); self.until('readyok')
 def send(self,s): self.p.stdin.write((s+'\n').encode())
 def until(self,mark):
  lines=[]
  while time.monotonic()<DEADLINE:
   if b'\n' in self.b:
    q,self.b=self.b.split(b'\n',1); q=q.decode(errors='replace').strip(); lines.append(q)
    if q.startswith(mark): return lines
   elif select.select([self.p.stdout],[],[],.2)[0]:
    q=os.read(self.p.stdout.fileno(),65536)
    if not q: raise RuntimeError(self.label+' exited')
    self.b+=q
  raise TimeoutError(self.label+' '+mark)
 def run(self,moves,cmd):
  chk(); self.send('ucinewgame'); self.send('isready'); self.until('readyok'); self.send('position startpos moves '+' '.join(moves)); self.send(cmd); raw=self.until('bestmove')
  d={'command':cmd,'bestmove':raw[-1].split()[1],'raw':raw,'usable':False}
  for line in raw:
   f=line.split()
   if not ('score'in f and 'depth'in f and 'pv'in f): continue
   i=f.index('score'); d.update(kind=f[i+1],value=int(f[i+2]),depth=int(f[f.index('depth')+1]),nodes=int(f[f.index('nodes')+1]) if 'nodes'in f else None,pv=f[f.index('pv')+1:],bounded=('lowerbound'in f or 'upperbound'in f))
   d['cp']=max(-1500,min(1500,d['value'])) if d['kind']=='cp' else (1500 if d['value']>0 else -1500); d['usable']=not d['bounded'] and ('depth' not in cmd or d['depth']>=16)
  return d
 def close(self):
  if self.p.poll() is None:
   self.send('quit')
   try:self.p.wait(timeout=5)
   except subprocess.TimeoutExpired:self.p.kill();self.p.wait()
def final(raw):
 d=None; best=None
 for line in raw:
  f=line.split()
  if line.startswith('bestmove'): best=f[1]
  if 'score'in f and 'depth'in f and 'pv'in f:
   i=f.index('score'); d={'kind':f[i+1],'value':int(f[i+2]),'depth':int(f[f.index('depth')+1]),'nodes':int(f[f.index('nodes')+1]),'pv':f[f.index('pv')+1:]}
 if not d or not best: raise RuntimeError('missing final UCI score or move')
 d['bestmove']=best; return d
def freeze():
 if sha(STUDY)!='95926b6cc5dd4aa66615b071ead937f413d9c9f7ed4b3957835d1a6afcf82737': raise RuntimeError('safe-study input changed')
 study=json.loads(STUDY.read_text()); prior=json.loads(PRIOR.read_text()); excluded={x['line_number'] for x in prior['cases']}
 allcases=[r for r in study['games'] if r.get('status')=='selected']; cases=[]
 for r in allcases:
  if r['game']['line_number'] in excluded: continue
  cases.append({'line_number':r['game']['line_number'],'game':r['game'],'prefix':r['prefix'],'fen':r['fen'],'stored_base_move':r['ngn']['bestmove']})
 x={'safe_study':str(STUDY),'safe_study_sha256':sha(STUDY),'prior_selection':str(PRIOR),'prior_selection_sha256':sha(PRIOR),'selected_game_count':len(allcases),'excluded_prior_ablation_line_numbers':sorted(excluded),'cases':cases,'case_count':len(cases)}
 if len(cases)!=16: raise RuntimeError('expected remaining 16 cases')
 save(SEL,x); return x
def main():
 global DEADLINE
 ap=argparse.ArgumentParser();ap.add_argument('--freeze-selection',action='store_true'); args=ap.parse_args(); selection=freeze()
 if args.freeze_selection: print(json.dumps({'frozen':str(SEL),'cases':len(selection['cases'])},indent=2)); return
 start=time.monotonic(); DEADLINE=start+900
 if sha(BASE)!=BASE_SHA or sha(FUT)!=FUT_SHA: raise RuntimeError('unexpected frozen binary')
 study={r['game']['line_number']:r for r in json.loads(STUDY.read_text())['games']}; report={'protocol':str(ROOT/'experiments/2026-09-05-futility-followup.md'),'script_sha256':sha(__file__),'source_commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'selection':selection,'baseline':{'path':str(BASE),'sha256':sha(BASE),'command':'go nodes 400000','hash_mb':64,'threads':1},'futility_off':{'path':str(FUT),'sha256':sha(FUT),'command':'go nodes 400000','hash_mb':64,'threads':1,'source_overlay':str(OUT/'search-ablation-Futility-search.go.txt'),'source_overlay_sha256':sha(OUT/'search-ablation-Futility-search.go.txt')},'stockfish':{'path':str(SF),'sha256':sha(SF),'command':'go depth 16'},'cases':[],'status':'running'}
 base=fut=sf=oracle=None
 try:
  base=Engine(BASE,'base')
  frozen=[]
  for c in selection['cases']:
   q=base.run(c['prefix'],'go nodes 400000'); exp=final(study[c['line_number']]['ngn']['raw']); got=final(q['raw'])
   if got!=exp: raise RuntimeError('baseline full final UCI parity failure line '+str(c['line_number']))
   c=dict(c); c['baseline_400k']=q; frozen.append(c)
  save(OUT/'futility-controls-baseline-parity-2026-09-05.json',{'verdict':'PASS','cases':[{'line_number':c['line_number'],'final':final(c['baseline_400k']['raw'])} for c in frozen]})
  fut=Engine(FUT,'futility_off'); sf=Engine(SF,'stockfish')
  spec=importlib.util.spec_from_file_location('oracle',ROOT/'experiments/2026-09-05-corpus-smoke-oracle.py'); om=importlib.util.module_from_spec(spec); spec.loader.exec_module(om); oracle=om.Stockfish()
  for c in frozen:
   fr=fut.run(c['prefix'],'go nodes 400000'); c['runs']={'base_400k':c.pop('baseline_400k'),'futility_off_400k':fr}
   choices={}
   for label,r in c['runs'].items(): choices.setdefault(r['bestmove'],[]).append(label)
   _,_,legal=oracle.position(c['prefix'])
   if not set(choices).issubset(legal): raise RuntimeError('illegal chosen move')
   scored=[]
   for move,labels in sorted(choices.items()):
    r=sf.run(c['prefix']+[move],'go depth 16'); scored.append({'move':move,'labels':labels,'reply':r,'ngn_pov_cp':(-r['cp'] if r['usable'] else None)})
   b=next(x for x in scored if 'base_400k'in x['labels'])
   for x in scored:
    x['delta_vs_base_scored_choice_cp']=None if b['ngn_pov_cp'] is None or x['ngn_pov_cp'] is None else x['ngn_pov_cp']-b['ngn_pov_cp'];x['diagnostic_rescue_50cp']=x['delta_vs_base_scored_choice_cp'] is not None and x['delta_vs_base_scored_choice_cp']>=50;x['diagnostic_regression_50cp']=x['delta_vs_base_scored_choice_cp'] is not None and x['delta_vs_base_scored_choice_cp']<=-50
   c['scored_choices']=scored; f=next(x for x in scored if 'futility_off_400k'in x['labels']); c['summary']={'futility_move_changed':fr['bestmove']!=c['runs']['base_400k']['bestmove'],'futility_delta_vs_base_scored_choice_cp':f['delta_vs_base_scored_choice_cp'],'diagnostic_rescue_50cp':f['diagnostic_rescue_50cp'],'diagnostic_regression_50cp':f['diagnostic_regression_50cp'],'final_depth_change':fr.get('depth',0)-c['runs']['base_400k'].get('depth',0),'final_nodes_change':fr.get('nodes',0)-c['runs']['base_400k'].get('nodes',0)}
   report['cases'].append(c); save(OUT/'futility-controls-partial-2026-09-05.json',report);print('completed line',c['line_number'],flush=True)
  summaries=[c['summary'] for c in report['cases']]; report['aggregate']={'case_count':len(summaries),'move_changes':sum(x['futility_move_changed'] for x in summaries),'rescues_50cp':sum(x['diagnostic_rescue_50cp'] for x in summaries),'regressions_50cp':sum(x['diagnostic_regression_50cp'] for x in summaries),'all_negative_count':sum(x['futility_delta_vs_base_scored_choice_cp'] is not None and x['futility_delta_vs_base_scored_choice_cp']<0 for x in summaries),'unavailable_scores':sum(x['futility_delta_vs_base_scored_choice_cp'] is None for x in summaries),'completed_depth_changes':sum(x['final_depth_change']!=0 for x in summaries)};report['status']='complete';report['elapsed_seconds']=time.monotonic()-start;save(RESULT,report);print(json.dumps({'status':'complete','aggregate':report['aggregate']},indent=2))
 finally:
  for x in (base,fut,sf,oracle):
   if x:x.close()
if __name__=='__main__':main()
