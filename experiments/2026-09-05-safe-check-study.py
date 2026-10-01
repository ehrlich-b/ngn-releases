#!/usr/bin/env python3
"""Offline follow-up to the fixed loss sample; see the adjacent protocol."""
import collections
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import select
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / 'output/recovery-2026-09-04'
REF = ROOT / 'output/review-2026-09-04/chess-3'
SF = Path(os.environ.get('STOCKFISH', '/opt/homebrew/bin/stockfish'))
NGN = ROOT / 'build/ngn_20260904_qcap'
STUDY = OUT / 'loss_study.json'
DEADLINE = None
sys.dont_write_bytecode = True

def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()

def save(name, value):
    (OUT / name).write_text(json.dumps(value, indent=2) + '\n')

spec = importlib.util.spec_from_file_location('oracle', ROOT / 'experiments/2026-09-05-corpus-smoke-oracle.py')
oracle = importlib.util.module_from_spec(spec)
spec.loader.exec_module(oracle)

class Engine:
    def __init__(self, binary):
        self.p = subprocess.Popen([str(binary)], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, bufsize=0)
        self.buf = b''
        self.send('uci'); self.until('uciok')
        self.send('setoption name Hash value 64')
        self.send('setoption name Threads value 1')
        self.send('isready'); self.until('readyok')

    def send(self, command):
        self.p.stdin.write((command + '\n').encode())

    def until(self, marker):
        lines = []
        deadline = min(time.monotonic() + 60, DEADLINE or float('inf'))
        while time.monotonic() < deadline:
            if b'\n' in self.buf:
                line, self.buf = self.buf.split(b'\n', 1)
                line = line.decode(errors='replace').strip()
                lines.append(line)
                if line.startswith(marker):
                    return lines
            elif select.select([self.p.stdout], [], [], .2)[0]:
                chunk = os.read(self.p.stdout.fileno(), 65536)
                if not chunk:
                    raise RuntimeError('engine exited')
                self.buf += chunk
        raise TimeoutError(marker)

    def analyse(self, prefix, command):
        self.send('ucinewgame'); self.send('isready'); self.until('readyok')
        self.send('position startpos moves ' + ' '.join(prefix))
        self.send(command)
        lines = self.until('bestmove')
        result = {'command': command, 'bestmove': lines[-1].split()[1], 'raw': lines, 'usable': False}
        for line in lines:
            f = line.split()
            if 'score' not in f or 'depth' not in f or 'pv' not in f:
                continue
            idx = f.index('score'); kind = f[idx+1]; value = int(f[idx+2])
            result.update(depth=int(f[f.index('depth')+1]), kind=kind, value=value,
                          cp=max(-1500,min(1500,value)) if kind == 'cp' else (1500 if value > 0 else -1500),
                          bounded=('lowerbound' in f or 'upperbound' in f), pv=f[f.index('pv')+1:])
            result['usable'] = not result['bounded'] and ('depth' not in command or result['depth'] >= 16)
        return result

    def close(self):
        if self.p.poll() is None:
            self.send('quit')
            try:
                self.p.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.p.kill(); self.p.wait()

def phase(fen):
    return min(24, sum({'n':1,'b':1,'r':2,'q':4}.get(c.lower(),0) for c in fen.split()[0]))

def overlay_features(fens):
    assert subprocess.check_output(['git','rev-parse','HEAD'],cwd=REF,text=True).strip() == 'a33531629cbe82eef6810f982ec814e79d52a3b3'
    fen_path = OUT / 'safe-study-fens.json'; fen_path.write_text(json.dumps(fens))
    ref_source = (REF / 'eval/king_attacks.go').read_text()
    ref_source += '\nvar studySafe, studyUnsafe [2][6]BitBoard\n'
    for method, target in [('addSafeChecks','studySafe'),('addUnsafeChecks','studyUnsafe')]:
        needle = f'func (ka *kingAttacks[T]) {method}(color Color, pType Piece, checks BitBoard, c *CoeffSet[T]) {{'
        assert ref_source.count(needle) == 1
        ref_source = ref_source.replace(needle, needle + f'\n {target}[color][pType] |= checks')
    ref_test = r'''
package eval
import("testing";"encoding/json";"os";"fmt";"github.com/paulsonkoly/chess-3/board";. "github.com/paulsonkoly/chess-3/chess")
func TestStudySafe(t *testing.T) {
 data,e:=os.ReadFile(INPUT);if e!=nil {t.Fatal(e)};var fens []string;if e=json.Unmarshal(data,&fens);e!=nil {t.Fatal(e)}
 rows:=[]any{}
 for _,fen:=range fens {b,e:=board.FromFEN(fen);if e!=nil {t.Fatal(e)}
  studySafe=[2][6]BitBoard{};studyUnsafe=[2][6]BitBoard{};_=Eval(b,&Coefficients)
  var s,u [2][6]string;for c:=range 2 {for p:=range 6 {s[c][p]=fmt.Sprintf("%016x",uint64(studySafe[c][p]));u[c][p]=fmt.Sprintf("%016x",uint64(studyUnsafe[c][p]))}}
  rows=append(rows,map[string]any{"fen":fen,"safe":s,"unsafe":u})
 }
 data,e=json.MarshalIndent(rows,"","  ");if e!=nil {t.Fatal(e)};if e=os.WriteFile(OUTPUT,data,0644);e!=nil {t.Fatal(e)}
}
'''
    ngn_test = r'''
package engine
import("testing";"encoding/json";"os")
func TestStudySafe(t *testing.T) {
 data,e:=os.ReadFile(INPUT);if e!=nil {t.Fatal(e)};var fens []string;if e=json.Unmarshal(data,&fens);e!=nil {t.Fatal(e)}
 rows:=[]any{}
 for _,fen:=range fens {pos,e:=ParseFEN(fen);if e!=nil {t.Fatal(e)};b:=&pos.Board;var a [64]uint64;fillSliderAttacks(b,&a)
  var zone [2]int
  for i,c:=range []Color{White,Black} {enemy:=c.Other();zone[i]=evaluateKingAttackPatterns(&a,b.GetBitboardOf(GetPiece(Queen,enemy)),b.GetBitboardOf(GetPiece(Rook,enemy)),b.GetBitboardOf(GetPiece(Bishop,enemy)),b.GetBitboardOf(GetPiece(Knight,enemy)),trailingZeros(b.GetBitboardOf(GetPiece(King,c))),c)}
  rows=append(rows,map[string]any{"fen":fen,"zone_penalty_white_black":zone})
 }
 data,e=json.MarshalIndent(rows,"","  ");if e!=nil {t.Fatal(e)};if e=os.WriteFile(OUTPUT,data,0644);e!=nil {t.Fatal(e)}
}
'''
    source_path = OUT / 'safe-study-ref-source.go.txt'; source_path.write_text(ref_source)
    env = os.environ.copy(); env.update(GOCACHE='/private/tmp/ngn-go-cache',GOMODCACHE='/private/tmp/ngn-go-modcache',GOMAXPROCS='2')
    results = {}
    for name,cwd,pkg,code in [('reference',REF,'eval',ref_test),('ngn',ROOT,'engine',ngn_test)]:
        result_path = OUT / f'safe-study-{name}-features.json'
        code = code.replace('INPUT',json.dumps(str(fen_path))).replace('OUTPUT',json.dumps(str(result_path)))
        test_path = OUT / f'safe-study-{name}_test.go.txt'; test_path.write_text(code)
        replacements = {str(cwd/pkg/'study_safe_test.go'):str(test_path)}
        if name == 'reference': replacements[str(REF/'eval/king_attacks.go')] = str(source_path)
        overlay = OUT / f'safe-study-{name}-overlay.json'; overlay.write_text(json.dumps({'Replace':replacements}))
        p = subprocess.run(['go','test','-overlay',str(overlay),'./'+pkg,'-run','^TestStudySafe$','-count=1','-v'],cwd=cwd,env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,timeout=120)
        (OUT/f'safe-study-{name}-overlay.log').write_text(p.stdout);p.check_returncode()
        results[name] = json.loads(result_path.read_text())
    return {fen:{k:v[i] for k,v in results.items()} for i,fen in enumerate(fens)}

def main():
    global DEADLINE
    start = time.monotonic(); DEADLINE = start + 1200
    old = json.loads(STUDY.read_text())
    assert len(old['games']) == 24
    capture = OUT / 'r0904pin.pgn'
    assert sha(capture) == old['corpus_sha256']
    records = {}
    for line in capture.read_text().splitlines():
        if line.startswith('GAME ') and ' | ' in line:
            head, moves = line.split(' | ', 1)
            records[hashlib.sha256(line.encode()).hexdigest()] = (head.split(), moves.split())
    assert len(records) == 200
    for game in old['games']:
        head, moves = records[game['game']['game_sha256']]
        assert head[1:4] == [game['game']['anchor'], game['game']['result'], game['game']['ngn_color']]
        for scan in game['candidate']['scan_results']:
            assert scan['prefix'] == moves[:scan['ply']] and scan['move'] == moves[scan['ply']]
            if scan['status'] == 'eligible':
                assert scan['before']['score_usable'] and scan['after']['score_usable']
                assert scan['shallow_before_ngn_cp'] >= -300
    assert sha(NGN) == '24d71a1ac381320d2ab790b1a88a7fdb30761bcedf573f9b98973b5a1e7eb702'
    audit = oracle.Stockfish(); sf = ngn = None
    report = {'protocol':'experiments/2026-09-05-safe-check-study.md','input_sha256':sha(STUDY),
              'script_sha256':sha(__file__),'corpus_sha256':sha(capture),
              'source_commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),
              'go_version':subprocess.check_output(['go','version'],text=True).strip(),
              'binary_sha256':{'ngn':sha(NGN),'stockfish':sha(SF)},'games':[]}
    try:
        for game in old['games']:
            row = {'game':game['game'],'status':'no_eligible_middlegame'}
            candidates = sorted((s for s in game['candidate']['scan_results'] if s['status']=='eligible'),key=lambda s:(-s['shallow_drop_cp'],s['ply']))
            for s in candidates:
                fen,check,legal = audit.position(s['prefix'])
                if phase(fen) < 12: continue
                assert s['move'] in legal
                row.update(status='selected',prefix=s['prefix'],played=s['move'],fen=fen,phase=phase(fen),old_drop=s['shallow_drop_cp']);break
            report['games'].append(row)
        save('safe-study-selection.json',report)
        sf = Engine(SF); ngn = Engine(NGN)
        all_fens = []
        for row in report['games']:
            if row['status'] != 'selected': continue
            prefix=row['prefix']
            row['ngn']=ngn.analyse(prefix,'go nodes 400000')
            row['sf']=sf.analyse(prefix,'go depth 16')
            fen,check,legal=audit.position(prefix)
            choices=collections.defaultdict(list)
            for label,move in [('played',row['played']),('current_ngn',row['ngn']['bestmove']),('sf',row['sf']['bestmove'])]:
                assert move in legal,(row['game'],label,move)
                choices[move].append(label)
            row['choices']=[]
            for move,labels in choices.items():
                after=prefix+[move];fen,check,legal=audit.position(after)
                reply=sf.analyse(after,'go depth 16')
                pieces=oracle.squares(fen.split()[0]);quiet_checks=[]
                for m in sorted(legal):
                    # Quiet piece checks only; never pawn/EP/promotion/capture.
                    if pieces[m[:2]].lower() not in 'nbrq' or len(m)!=4 or m[2:4] in pieces: continue
                    _,gives_check,_=audit.position(after+[m])
                    if gives_check: quiet_checks.append({'move':m,'piece':pieces[m[:2]].lower()})
                if reply['bestmove'] not in ('(none)','0000'): assert reply['bestmove'] in legal
                item={'move':move,'labels':labels,'fen':fen,'reply':reply,'quiet_checks':quiet_checks,
                      'ngn_pov_cp':-reply['cp'] if reply['usable'] else None}
                row['choices'].append(item);all_fens.append(fen)
            save('safe-study-partial.json',report)
            print('completed game',row['game']['line_number'],flush=True)
        features=overlay_features(list(dict.fromkeys(all_fens)))
        counts=collections.Counter(); witnesses=[]
        for row in report['games']:
            if row['status']!='selected':counts['unavailable']+=1;continue
            counts['selected']+=1
            sf_choice=next(c for c in row['choices'] if 'sf' in c['labels'])
            for c in row['choices']:
                c['features']=features[c['fen']]
                color=0 if c['fen'].split()[1]=='w' else 1
                for m in c['quiet_checks']:
                    idx={'n':2,'b':3,'r':4,'q':5}[m['piece']]
                    dst=(ord(m['move'][2])-97)+8*(int(m['move'][3])-1)
                    m['reference_safe']=bool(int(c['features']['reference']['safe'][color][idx],16)&(1<<dst))
                    m['reference_unsafe']=bool(int(c['features']['reference']['unsafe'][color][idx],16)&(1<<dst))
                best=next((m for m in c['quiet_checks'] if m['move']==c['reply']['bestmove']),None)
                c['sf_reply_quiet_check']=best
                c['relative_loss_cp']=None if sf_choice['ngn_pov_cp'] is None or c['ngn_pov_cp'] is None else sf_choice['ngn_pov_cp']-c['ngn_pov_cp']
                counts['positions']+=1
                counts['positions_with_legal_safe_quiet_check']+=int(any(m['reference_safe'] for m in c['quiet_checks']))
                counts['positions_sf_bestreply_safe_quiet_check']+=int(bool(best and best['reference_safe']))
                if 'sf' not in c['labels'] and c['relative_loss_cp'] is not None and c['relative_loss_cp']>=50 and best and best['reference_safe']:
                    witnesses.append({'game':row['game'],'move':c['move'],'labels':c['labels'],'loss_cp':c['relative_loss_cp'],'reply':best,'fen':c['fen']})
        report.update(counts=dict(counts),witnesses=witnesses,status='complete',elapsed_seconds=time.monotonic()-start)
        save('safe-study-result.json',report)
        print(json.dumps({'counts':report['counts'],'witnesses':witnesses,'elapsed_seconds':report['elapsed_seconds']},indent=2))
    finally:
        for e in (audit,sf,ngn):
            if e is not None:e.close()

if __name__=='__main__':main()
