#!/usr/bin/env python3
"""Run the separately frozen e20 follow-up after the main overnight queue."""
from __future__ import annotations

import argparse
import datetime
import json
import os
import subprocess
import sys
import time
from pathlib import Path

import ownednight

sys.path.insert(0, str(Path(__file__).resolve().parent / 'candidate-match'))
from common import atomic_json, require_wsl, safe_environment, sha256
from uci_preflight import Protocol, parse_advertised_options


def checked(path: Path) -> dict:
    return {'path': str(path), 'sha256': sha256(path)}


def lifecycle(root: Path, network: Path, out: Path) -> None:
    """Check real width-eight sessions without attributing strength or speed."""
    bundle = root / 'bundle-wdl25'
    win = Path('/mnt/c/Users/ehrli/AppData/Local/Temp/ngn-owned-candidate-20260928')
    # The Windows candidate directory already contains these verified binaries.
    if sha256(root / 'build/ngn.exe') != sha256(win / 'ngn.exe'):
        raise RuntimeError('Windows smoke copy differs')
    win_net = win / 'lifecycle.nnue'
    win_net.write_bytes(network.read_bytes())
    if sha256(win_net) != sha256(network):
        raise RuntimeError('Windows smoke network differs')
    win_cmd = win / 'lifecycle.cmd'
    win_cmd.write_bytes(b'@echo off\r\nset "GOMAXPROCS=8"\r\n"%~dp0ngn.exe" -eval-backend ngn-k4-768-v1 -eval-file "%~dp0lifecycle.nnue" -k4-eval-scale 60 -own-book=false\r\n')
    commands = [
        ('linux', ['taskset','-c','0,1,2,3',str(root/'build/ngn'),'-eval-backend','ngn-k4-768-v1','-eval-file',str(network),'-k4-eval-scale','60','-own-book=false'], bundle),
        ('windows', ['/mnt/c/Windows/System32/cmd.exe','/d','/c',r'C:\Users\ehrli\AppData\Local\Temp\ngn-owned-candidate-20260928\lifecycle.cmd'],win),
    ]
    reports = []
    for platform, command, cwd in commands:
        trace = out / ('lifecycle-' + platform + '.jsonl')
        with trace.open('x', buffering=1) as transcript:
            process = subprocess.Popen(command, cwd=cwd, env=dict(safe_environment(),GOMAXPROCS='8'),
                                       stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            try:
                protocol = Protocol(process, transcript)
                protocol.send('uci')
                options = parse_advertised_options(protocol.until(lambda x:x=='uciok',30,'uci'))
                assert options['K4EvalScale']['default']==60 and options['OwnBook']['default'] is False
                protocol.send('debug on'); protocol.send('isready')
                protocol.until(lambda x:x=='readyok',30,'debug')
                for name,value in [('Threads',8),('Hash',32)]:
                    protocol.send(f'setoption name {name} value {value}');protocol.send('isready')
                    lines=protocol.until(lambda x:x=='readyok',30,name)
                    assert f'info string option set: {name} = {value}' in lines
                searches=[]
                for label,command in [('nodes','go nodes 80000'),('stop','go infinite'),('restart','go nodes 40000')]:
                    protocol.send('ucinewgame');protocol.send('isready')
                    protocol.until(lambda x:x=='readyok',30,'newgame')
                    protocol.send('position startpos');protocol.send(command)
                    if label=='stop':
                        protocol.until(lambda x:x.startswith('info depth 4 '),30,'live search')
                        protocol.send('stop')
                    lines=protocol.until(lambda x:x.startswith('bestmove '),30,label)
                    best=lines[-1].split()[1]
                    # Root is always startpos; this static set is its complete legal move set.
                    legal={f'{f}2{f}{rank}' for f in 'abcdefgh' for rank in '34'}|{'b1a3','b1c3','g1f3','g1h3'}
                    assert best in legal
                    searches.append({'case':label,'bestmove':best,'last_info':next((x for x in reversed(lines) if x.startswith('info depth ')),None)})
                for name,value in [('Threads',1),('K4EvalScale',100),('K4EvalScale',60),('Threads',8)]:
                    protocol.send(f'setoption name {name} value {value}');protocol.send('isready')
                    lines=protocol.until(lambda x:x=='readyok',30,name)
                    assert f'info string option set: {name} = {value}' in lines
                protocol.send('position startpos');protocol.send('go nodes 40000')
                lines=protocol.until(lambda x:x.startswith('bestmove '),30,'reconfigured')
                assert lines[-1].split()[1] in legal
                searches.append({'case':'reconfigured','bestmove':lines[-1].split()[1]})
                protocol.send('quit');assert process.wait(timeout=30)==0
                for thread in protocol.threads:
                    thread.join(timeout=5);assert not thread.is_alive()
                assert not protocol.stderr_lines
                rows=[json.loads(line) for line in trace.read_text().splitlines()]
                output=[row['line'] for row in rows if row['direction']=='engine-stdout']
                assert sum(line.startswith('bestmove ') for line in output)==4
                assert not any(line.startswith('info string error') for line in output)
                reports.append({'platform':platform,'pass':True,'searches':searches,'transcript':checked(trace)})
            finally:
                if process.poll() is None:
                    process.kill();process.wait()
    atomic_json(out/'lifecycle.json',{'pass':True,'network':checked(network),'threads':8,'linux_gomaxprocs':8,
                'linux_cpus':[0,1,2,3],'claim':'lifecycle smoke only; no SMP Elo or throughput comparison','reports':reports})


def run(root: Path, plan_path: Path) -> None:
    plan=json.loads(plan_path.read_text());out=root/'followup';out.mkdir(exist_ok=True)
    deadline=datetime.datetime.fromisoformat(plan['deadline_utc'].replace('Z','+00:00')).timestamp()
    atomic_json(out/'STATE.json',{'state':'WAITING_MAIN','plan':checked(plan_path),'started':time.time()})
    while True:
        state=json.loads((root/'matches/STATE.json').read_text())
        if state['state']=='FAILED':
            if not plan.get('allow_held_external_anchor',False):
                raise RuntimeError('main queue failed; follow-up is held')
            external=json.loads((root/'matches/external-long/STATE.json').read_text())
            if external['state']!='FAILED' or not external['error'].startswith('trace audit failures:'):
                raise RuntimeError('main failure is not the admitted held external audit')
            for name in ['aa-short','reference','wdl40','aa-long']:
                terminal=json.loads((root/'matches'/name/'STATE.json').read_text())
                control=json.loads((root/'matches'/name/'control.json').read_text())
                if terminal['state']!='COMPLETE' or not control['pass']:
                    raise RuntimeError('owned controls or completed gate failed')
            unit=json.loads((root/'matches/unit.json').read_text())['unit']
            status=subprocess.run(['systemctl','--user','show',unit,'-p','ActiveState','--value'],capture_output=True,text=True,check=True)
            if status.stdout.strip()=='active':
                raise RuntimeError('main match unit remains active')
            break
        if state['state']=='COMPLETE':
            break
        if time.time()>=deadline-plan['minimum_remaining_seconds']:
            atomic_json(out/'STATE.json',{'state':'SKIPPED','reason':'insufficient time for the fixed cell','completed':time.time()})
            return
        time.sleep(min(600,max(1,deadline-plan['minimum_remaining_seconds']-time.time())))
    if time.time()>deadline-plan['minimum_remaining_seconds'] or ownednight.capacity()<25:
        atomic_json(out/'STATE.json',{'state':'SKIPPED','reason':'admission time/capacity floor','completed':time.time()})
        return
    verdict=json.loads((root/'matches/wdl40-verdict.json').read_text())
    if verdict['report_sha256']!=sha256(root/'matches/wdl40/report.json'):
        raise RuntimeError('main gate verdict report differs')
    spec=json.loads((root/'specs/reference.json').read_text())
    for key in ['tc','concurrency','cpus','strict','opening_plies']:
        if spec[key]!=plan['configuration'][key]:
            raise RuntimeError('follow-up configuration differs from frozen plan')
    baseline=dict(spec['roles'][0])
    if verdict['decision']=='PROMOTE':
        ready=json.loads((root/'training/wdl40-e10-ready.json').read_text())
        baseline.update(name='Owned-WDL40',network=ready['network'])
        expected=plan['wdl40_network_sha256']
    elif verdict['decision']=='RETAIN_WDL25':
        expected=plan['wdl25_network_sha256']
    else:
        raise RuntimeError('unknown main verdict')
    if baseline['network']['sha256']!=expected or sha256(Path(baseline['network']['path']))!=expected:
        raise RuntimeError('selected baseline differs')
    if baseline['engine']['sha256']!=plan['engine_sha256'] or sha256(Path(baseline['engine']['path']))!=plan['engine_sha256']:
        raise RuntimeError('engine differs')
    ready=json.loads((root/'training/e20-resume-ready.json').read_text())
    if ready['state']!='COMPLETE' or ready['network']['sha256']!=plan['e20_network_sha256'] or sha256(Path(ready['network']['path']))!=plan['e20_network_sha256']:
        raise RuntimeError('e20 export differs')
    for name in ['wdl40-e10','e20-resume']:
        receipt=json.loads((root/'training'/(name+'-ready.json')).read_text())
        parity=root/'training'/(name+'-parity.stdout')
        if sha256(parity)!=receipt['parity_sha256'] or not json.loads(parity.read_text())['pass']:
            raise RuntimeError('export parity differs')
    atomic_json(out/'selection.json',{'baseline':baseline,'e20':ready['network'],'main_verdict':checked(root/'matches/wdl40-verdict.json'),'plan':checked(plan_path)})
    lifecycle(root,Path(baseline['network']['path']),out)
    master=root/'openings/master.txt'
    if sha256(master)!=plan['opening_master_sha256']:
        raise RuntimeError('opening master differs')
    prefixes=master.read_text().splitlines()
    receipt=json.loads((root/'specs/openings-receipt.json').read_text())
    converter=Path(receipt['converter_path'])
    if sha256(converter)!=receipt['converter_sha256']:
        raise RuntimeError('opening converter differs')
    for cell in plan['matches']:
        name=cell['name'];start,end=cell['opening_lines']
        text=out/(name+'.txt');pgn=out/(name+'.pgn')
        with text.open('x') as handle: handle.write('\n'.join(prefixes[start-1:end])+'\n')
        subprocess.run([str(converter),str(text),str(pgn),str(end-start+1)],check=True,timeout=30)
        current=dict(spec,purpose='aa' if name=='e20-aa' else 'gate',pairs=cell['games']//2,
                     roles=[dict(baseline,name='Selected-A'),dict(baseline,name='Selected-B')] if name=='e20-aa' else [dict(baseline,name='Owned-E20',network=ready['network']),baseline],
                     openings_pgn=checked(pgn),opening_prefixes=checked(text),
                     limit_seconds=min(7200,int(deadline-time.time()-300)))
        if name=='e20-gate':current['control_report']=checked(out/'e20-aa/control.json')
        spec_path=out/(name+'.json');atomic_json(spec_path,current)
        atomic_json(out/'STATE.json',{'state':'RUNNING','name':name,'started':time.time()})
        ownednight.run_bounded(['python3',str(root/'source/scripts/ownedmatch.py'),'--spec',str(spec_path),'--out',str(out/name)],
                               out/(name+'-run'),deadline,dict(os.environ,PATH='/usr/bin:/bin'),'4,6,8,10,12,14')
        if json.loads((out/name/'STATE.json').read_text())['state']!='COMPLETE':
            raise RuntimeError('follow-up has no accepted terminal')
        print(json.dumps({'name':name,'report':json.loads((out/name/'report.json').read_text())}),flush=True)
    report=json.loads((out/'e20-gate/report.json').read_text())
    atomic_json(out/'verdict.json',{'state':'COMPLETE','decision':'PROMOTE_E20' if report['elo_interval_95'][0]>0 else 'RETAIN_SELECTED',
                'baseline':baseline['name'],'rule':plan['matches'][1]['decision'],'report':checked(out/'e20-gate/report.json')})
    atomic_json(out/'STATE.json',{'state':'COMPLETE','completed':time.time()})


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root',type=Path,required=True);parser.add_argument('--plan',type=Path,required=True)
    args=parser.parse_args();require_wsl()
    try:run(args.root.resolve(),args.plan.resolve())
    except Exception as error:
        atomic_json(args.root/'followup/STATE.json',{'state':'FAILED','error':str(error),'completed':time.time()})
        raise
