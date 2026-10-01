#!/usr/bin/env python3
"""Run the separately frozen fresh Counter anchor after the owned follow-up."""
import argparse
import datetime
import json
import os
import subprocess
import time
from pathlib import Path

import ownednight
from ownednight import atomic_json, require_wsl, sha256


def checked(path):
    path=Path(path)
    return {'path':str(path),'sha256':sha256(path)}


def run(root,plan_path):
    plan=json.loads(plan_path.read_text());out=root/'external-fresh';out.mkdir(exist_ok=True)
    deadline=datetime.datetime.fromisoformat(plan['deadline_utc'].replace('Z','+00:00')).timestamp()
    os.environ['NGN_COUNTER_DRAW_DEPS']=str(root/'counterdraw-deps/chess-1.11.2')
    import ownedmatch_counterdraw as wrapper
    controls=json.loads((root/'native-counter/draw-controls.json').read_text())
    if not controls['pass'] or not controls['negative_fixtures'] or controls['instrument']!=wrapper.instrument():
        raise RuntimeError('semantic controls did not pass')
    smoke=json.loads((out/'smoke/STATE.json').read_text())
    if smoke['state']!='COMPLETE':raise RuntimeError('new instrument smoke did not pass')
    if json.loads((out/'smoke/control.json').read_text())['instrument']!=controls['instrument']:
        raise RuntimeError('smoke instrument differs from proved controls')
    atomic_json(out/'STATE.json',{'state':'WAITING_OWNED','plan':checked(plan_path),'started':time.time()})
    while True:
        state=json.loads((root/'followup/STATE.json').read_text())
        if state['state'] in ['COMPLETE','FAILED','SKIPPED']:
            unit=json.loads((root/'followup/unit.json').read_text())['unit']
            status=subprocess.run(['systemctl','--user','show',unit,'-p','ActiveState','--value'],capture_output=True,text=True,check=True)
            if status.stdout.strip()!='active':break
        if time.time()>=deadline-plan['minimum_remaining_seconds']:
            atomic_json(out/'STATE.json',{'state':'SKIPPED','reason':'fixed-cell time admission','completed':time.time()});return
        time.sleep(min(600,max(1,deadline-plan['minimum_remaining_seconds']-time.time())))
    if time.time()>deadline-plan['minimum_remaining_seconds'] or ownednight.capacity()<25:
        atomic_json(out/'STATE.json',{'state':'SKIPPED','reason':'time/capacity admission','completed':time.time()});return
    spec=json.loads((root/'specs/external-long.json').read_text());baseline=spec['roles'][0];counter=spec['roles'][1]
    for key in ['tc','concurrency','cpus','strict']:
        if spec[key]!=plan['configuration'][key]:raise RuntimeError('fresh configuration differs')
    if baseline['engine']['sha256']!=plan['engine_sha256'] or counter['engine']['sha256']!=plan['counter_sha256']:
        raise RuntimeError('fixed engines differ')
    verdict=root/'followup/verdict.json'
    if state['state']=='COMPLETE':
        decision=json.loads(verdict.read_text())
        if decision['report']['sha256']!=sha256(Path(decision['report']['path'])):raise RuntimeError('owned verdict differs')
        if decision['decision']=='PROMOTE_E20':
            ready=json.loads((root/'training/e20-resume-ready.json').read_text());baseline=dict(baseline,name='Owned-E20',network=ready['network'])
        elif decision['decision']!='RETAIN_SELECTED':raise RuntimeError('unknown owned decision')
    atomic_json(out/'selection.json',{'baseline':baseline,'plan':checked(plan_path),'owned_terminal':state})
    master=root/'openings/master.txt'
    if sha256(master)!=plan['opening_master_sha256']:raise RuntimeError('master openings differ')
    prefixes=master.read_text().splitlines();receipt=json.loads((root/'specs/openings-receipt.json').read_text());converter=Path(receipt['converter_path'])
    if sha256(converter)!=receipt['converter_sha256']:raise RuntimeError('converter differs')
    env=dict(os.environ,PATH='/usr/bin:/bin',NGN_COUNTER_DRAW_DEPS=str(root/'counterdraw-deps/chess-1.11.2'))
    for cell in plan['matches']:
        name=cell['name'];start,end=cell['opening_lines'];text=out/(name+'.txt');pgn=out/(name+'.pgn')
        with text.open('x') as handle:handle.write('\n'.join(prefixes[start-1:end])+'\n')
        subprocess.run([str(converter),str(text),str(pgn),str(end-start+1)],check=True,timeout=30)
        current=dict(spec,purpose='aa' if name.endswith('-aa') else 'anchor',pairs=cell['games']//2,
                     roles=[dict(baseline,name='Selected-A'),dict(baseline,name='Selected-B')] if name.endswith('-aa') else [baseline,counter],
                     openings_pgn=checked(pgn),opening_prefixes=checked(text),limit_seconds=min(21600,int(deadline-time.time()-300)))
        if not name.endswith('-aa'):current['control_report']=checked(out/'counter-fresh-aa/control.json')
        path=out/(name+'.json');atomic_json(path,current)
        atomic_json(out/'STATE.json',{'state':'RUNNING','name':name,'started':time.time()})
        ownednight.run_bounded(['python3',str(root/'source/scripts/ownedmatch_counterdraw.py'),'--spec',str(path),'--out',str(out/name)],
                               out/(name+'-run'),deadline,env,'4,6,8,10,12,14')
        if json.loads((out/name/'STATE.json').read_text())['state']!='COMPLETE':raise RuntimeError('fresh cell lacks accepted terminal')
        print(json.dumps({'name':name,'report':json.loads((out/name/'report.json').read_text())}),flush=True)
    atomic_json(out/'STATE.json',{'state':'COMPLETE','completed':time.time()})


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--root',type=Path,required=True);parser.add_argument('--plan',type=Path,required=True)
    args=parser.parse_args();require_wsl()
    try:run(args.root.resolve(),args.plan.resolve())
    except Exception as error:
        atomic_json(args.root/'external-fresh/STATE.json',{'state':'FAILED','error':str(error),'completed':time.time()});raise
