"""Owned T80 validation, bounded training, exact export checks and fixed gates.

Run only on WSL from the isolated worktree. All outputs stay under --root.
"""

import argparse
import collections
import datetime
import hashlib
import importlib.util
import json
import math
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time

import numpy as np
import torch

from trainer.archive import ARCHIVE_DTYPE, Archive, canonical_records
from trainer.model import NGNN
from trainer.ngnp import RECORD_DTYPE, Shards, decode_record, to_fen
from trainer.refeval import Network
from trainer.train import validation_metrics

CORPUS = Path('/home/ehrli/nnue-owned-k4-20260920/archive-a1/corpus-a1/train-archive.bf')
BASE = Path('/home/ehrli/ngn-data/nets/net1/net1.nnue')
BOOK = Path('/home/ehrli/ngn-data/net1-gates-20261006/openings.txt')
BASE_SHA = '6a203ffe128541ef422f9344f2906bffbebea6d93931bb1f50e27f44bf2beff6'
BOOK_SHA = '974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222'
DEADLINE = datetime.datetime(2026, 10, 7, 10, 55, tzinfo=datetime.timezone.utc).timestamp()
GO = '/usr/local/go/bin/go'
ACTIVE_GROUP = None


def configure_interop(root):
    """Use the existing persistent keepalive's Windows interop endpoint.

    The launcher endpoint can become unusable after a detached SSH/WSL shell
    exits. Only this controller's environment changes; no host service changes.
    """
    for process in Path('/proc').iterdir():
        if not process.name.isdecimal():
            continue
        try:
            if (process/'cmdline').read_bytes().split(b'\0')[:2] != [b'sleep',b'infinity']:
                continue
            endpoint = next(value.split(b'=',1)[1].decode() for value in (process/'environ').read_bytes().split(b'\0')
                            if value.startswith(b'WSL_INTEROP='))
            if not Path(endpoint).is_socket():
                continue
            os.environ['WSL_INTEROP'] = endpoint
            write(root/'interop.json',dict(keepalive_pid=int(process.name),endpoint=endpoint))
            return
        except (OSError,StopIteration):
            continue
    raise RuntimeError('Existing ngn-wsl-keepalive interop endpoint unavailable')


def stop_group(pid):
    try:
        os.killpg(pid, signal.SIGTERM)
    except ProcessLookupError:
        return
    # One bounded grace period; descendants are killed even if leader exited.
    time.sleep(2)
    try:
        os.killpg(pid, signal.SIGKILL)
    except ProcessLookupError:
        pass


def interrupted(signum, frame):
    if ACTIVE_GROUP is not None:
        stop_group(ACTIVE_GROUP)
    raise RuntimeError(f'Controller interrupted by signal {signum}')


def internal_stage(root, stage):
    disk(root, stage)
    if time.time() >= DEADLINE:
        raise RuntimeError('Stop deadline reached')
    write(root / 'current-job.json', dict(stage=stage, pid=os.getpid(),
          stop_command=f'kill -TERM {os.getpid()}', implementation='controller internal stage'))


def write(path, value):
    tmp = path.with_suffix(path.suffix + '.tmp')
    tmp.write_text(json.dumps(value, indent=2, allow_nan=False) + '\n')
    tmp.replace(path)


def sha(path):
    result = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            result.update(chunk)
    return result.hexdigest()


def load_script(filename):
    spec = importlib.util.spec_from_file_location(filename.replace('-', '_'), Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def disk(root, stage):
    free = float(subprocess.check_output(['powershell.exe', '-NoProfile', '-Command',
                                         '(Get-PSDrive C).Free/1GB'], stdin=subprocess.DEVNULL,
                                        text=True, timeout=60).strip())
    receipt = dict(stage=stage, free_gib=free, utc=datetime.datetime.now(datetime.timezone.utc).isoformat())
    with (root / 'disk-checks.jsonl').open('a') as stream:
        stream.write(json.dumps(receipt) + '\n')
    if free < 6:
        raise RuntimeError(f'NEEDS_DISK: C: has {free:.6f} GiB free before {stage}')
    outputs = [p.stat().st_size for p in root.rglob('*') if p.is_file()]
    if sum(outputs) > 15 * 1024**3 or max(outputs, default=0) > 5 * 1024**3:
        raise RuntimeError('Own outputs exceed write allocation')
    return free


def run(root, stage, command, env=None, timeout=None):
    global ACTIVE_GROUP
    disk(root, stage)
    remaining = DEADLINE - time.time()
    if remaining <= 0:
        raise RuntimeError('Stop deadline reached')
    limit = min(timeout or remaining, remaining)
    log = root / (stage + '.log')
    started = time.monotonic()
    with log.open('w') as stream:
        process = subprocess.Popen(command, stdin=subprocess.DEVNULL, stdout=stream,
                                   stderr=subprocess.STDOUT, env=env, start_new_session=True)
        ACTIVE_GROUP = process.pid
        job = dict(stage=stage, pid=process.pid, pgid=process.pid, command=command,
                   timeout_seconds=limit, stop_command=f'kill -TERM -- -{process.pid}')
        write(root / 'current-job.json', job)
        try:
            process.wait(timeout=limit)
            if process.returncode:
                raise RuntimeError(f'{stage} failed, exit {process.returncode}; see {log}')
        finally:
            stop_group(process.pid)
            process.wait()
            ACTIVE_GROUP = None
    receipt = dict(stage=stage, command=command, wall_seconds=time.monotonic() - started,
                   log_sha256=sha(log), returncode=process.returncode,
                   environment={key:(env or os.environ).get(key) for key in
                                ('GOAMD64','GOMAXPROCS','GOTOOLCHAIN','GOPROXY','OMP_NUM_THREADS',
                                 'MKL_NUM_THREADS','OPENBLAS_NUM_THREADS','PYTHONPATH','WSL_INTEROP','WSLENV')})
    with (root / 'commands.jsonl').open('a') as stream:
        stream.write(json.dumps(receipt) + '\n')
    return receipt


def prepare(root):
    import chess  # Validation oracle only; never part of reader/trainer/runtime.
    disk(root, 'archive-validation')
    manifest = json.loads(CORPUS.with_name('manifest.json').read_text())
    assert CORPUS.stat().st_size == manifest['train']['bytes'] == manifest['records'] * 32
    records = np.memmap(CORPUS, mode='r', dtype=ARCHIVE_DTYPE)
    indexes = np.random.default_rng(20261006).choice(len(records), 8192, replace=False)
    selected = canonical_records(records[indexes])
    positions = [decode_record(record) for record in selected]
    invalid = [i for i, position in enumerate(positions) if not chess.Board(to_fen(position)).is_valid()]
    assert not invalid, invalid[:20]
    net = Network(BASE)
    evaluations = np.asarray([net.evaluate(position) for position in positions], dtype=np.float64)
    scores = selected['score'].astype(np.float64)
    result = selected['result'].astype(np.float64) / 2
    sigmoid = lambda x: 1 / (1 + np.exp(-x))
    metrics = []
    for k in (240, 400, 600):
        prediction = sigmoid(evaluations / k)
        teacher = sigmoid(scores / k)
        metrics.append(dict(k=k, net1_lambda_075_loss=float(np.mean((prediction - .75*teacher - .25*result)**2)),
                            teacher_result_loss=float(np.mean((teacher-result)**2))))
    centered = (np.abs(scores) < 600) & (np.abs(evaluations) < 2000)
    scale = dict(sample_records=len(selected), centered_records=int(centered.sum()),
                 net1_over_archive_slope=float(evaluations[centered] @ scores[centered] / (scores[centered] @ scores[centered])),
                 archive_score_quantiles=np.quantile(scores, [0,.01,.1,.5,.9,.99,1]).tolist(),
                 net1_eval_quantiles=np.quantile(evaluations, [0,.01,.1,.5,.9,.99,1]).tolist(),
                 label_metrics=metrics, mapping='identity: gather already applied manifest knots', k=400,
                 decision='retain documented mapped score/400 contract; k comparisons are diagnostics, not tuned selection')
    # Check actual source records whose original STM survived in probe metadata.
    # Reversing normalization must preserve both score sign and integer NNUE eval.
    import csv
    from trainer.ngnp import Position, encode_record
    probe = CORPUS.parent.parent/'scatter/probe.csv'
    known = []
    with probe.open() as stream:
        for row in csv.DictReader(stream):
            packed = np.frombuffer(bytes.fromhex(row['board_hex']), dtype=ARCHIVE_DTYPE)
            canonical = canonical_records(packed)[0]
            assert int(canonical['score']) == int(row['raw_score'])
            assert int(canonical['result']) == int(row['result'])+1
            normalized = decode_record(canonical)
            black = row['side'] == 'b'
            original = (Position(normalized.squares ^ 56,(normalized.pieces+6)%12,1)
                        if black else normalized)
            white_score = int(row['raw_score'])*(-1 if black else 1)
            white_result = 2-int(canonical['result']) if black else int(canonical['result'])
            original_record = encode_record(original,white_score,white_result)
            assert net.evaluate(original) == net.evaluate(normalized)
            assert int(original_record['score'])*(1-2*int(original_record['stm'])) == int(canonical['score'])
            known.append(dict(source_side=row['side'],original_fen=to_fen(original),
                              canonical_fen=to_fen(normalized),raw_score_stm=int(canonical['score']),
                              white_score=white_score,net1_eval_equal=True))
            if len(known)==20:
                break
    write(root / 'decode-scale.json', dict(corpus_manifest=manifest,
          manifest_sha256=sha(CORPUS.with_name('manifest.json')), size_verified=True,
          sampled_legal_boards=len(positions), original_stm='discarded; canonical white is source side to move',
          known_source_positions=known,
          samples=[dict(index=int(indexes[i]), fen=to_fen(positions[i]), score_stm=int(scores[i]),
                        result_stm=float(result[i]), net1_cp=int(evaluations[i])) for i in range(20)], scale=scale))
    started = time.monotonic()
    archive = Archive([str(CORPUS)], seed=20261006)
    write(root / 'archive-split.json', dict(**archive.split_receipt, index_seconds=time.monotonic()-started))
    holdout = archive.holdout()
    # Fixed exact 20-case export fixtures span the held-out rows.
    targets = np.linspace(0, holdout.count - 1, 20, dtype=np.int64)
    chosen, seen = [], 0
    for raw in holdout.batches(16384, 0, shuffle=False):
        block = raw.reshape(-1).view(RECORD_DTYPE)
        for target in targets[(targets >= seen) & (targets < seen + len(raw))]:
            chosen.append(block[int(target - seen)].copy())
        seen += len(raw)
    assert seen == holdout.count and len(chosen) == 20
    np.asarray(chosen, dtype=RECORD_DTYPE).tofile(root / 'parity-records.ngnp')
    write(root / 'parity-positions.json', dict(filtered_indexes=targets.tolist(),
                                             fens=[to_fen(decode_record(record)) for record in chosen]))
    internal_stage(root, 'corpus-digest')
    digest = sha(CORPUS)
    assert digest == manifest['train']['sha256']
    write(root / 'corpus-digest.json', dict(path=str(CORPUS), sha256=digest, bytes=CORPUS.stat().st_size,
                                          matched_manifest=True))
    print(json.dumps(dict(event='prepared', split=archive.split_receipt, scale=scale)), flush=True)


def game_result(log, games):
    text = log.read_text()
    total = re.search(r'Games: (\d+)\s+W-D-L: (\d+)-(\d+)-(\d+)', text)
    paired = re.search(r'Pentanomial \[LL (\d+)\s+LD (\d+)\s+\{LW,DD\} (\d+)\s+WD (\d+)\s+WW (\d+)\] over (\d+) pairs', text)
    flags = re.search(r'Flag-outs \(lost on time\): new (\d+), base (\d+)', text)
    assert total and paired and flags
    n, w, d, l = map(int, total.groups())
    counts = list(map(int, paired.groups()[:5]))
    assert n == games == w+d+l and sum(counts) == games//2 == int(paired[6])
    assert sum(i*c for i,c in enumerate(counts)) == 2*w+d
    previous, reasons = [0,0,0], collections.Counter()
    for game,cw,cd,cl,reason in re.findall(r'^G(\d+)\s+(\d+)W\s+(\d+)D\s+(\d+)L\s+.*?\s+(\S+)\s*$', text, re.M):
        now = list(map(int,(cw,cd,cl)))
        assert sum(now) == int(game) == sum(previous)+1
        assert sorted(now[i]-previous[i] for i in range(3)) == [0,0,1]
        reasons[reason] += 1
        previous = now
    assert previous == [w,d,l] and sum(reasons.values()) == n
    anomalies = [marker for marker in ('WATCHDOG:', 'no-move', 'illegal-move', 'info string error',
                  'failed to start', 'no uciok within', 'no readyok within', 'panic:', 'fatal error:') if marker in text]
    anomalies += [reason for reason in reasons if reason not in ('checkmate','stalemate','draw-rule','max-moves')]
    assert not anomalies and list(map(int,flags.groups())) == [0,0], (anomalies,flags.groups())
    mean = (w+d/2)/n
    variance = max(.001, sum(c*(k/4)**2 for k,c in enumerate(counts))/(n/2)-mean**2)
    se = math.sqrt(variance/(n/2))
    elo = lambda p: -800. if p <= 1e-4 else 800. if p >= 1-1e-4 else 400*math.log10(p/(1-p))
    estimate, interval = elo(mean), [elo(mean-1.96*se),elo(mean+1.96*se)]
    printed = re.search(r'Penta Elo: ([+-]?[\d.]+)\s+95% CI \[([+-]?\d+), ([+-]?\d+)\]', text)
    assert printed and abs(float(printed[1])-estimate) <= .051
    assert all(abs(float(printed[i+2])-interval[i]) <= .501 for i in (0,1))
    return dict(games=n,wdl=[w,d,l],penta=counts,elo=estimate,elo_95_ci=interval,
                flags=[0,0],anomalies=[],reasons=dict(reasons),log_sha256=sha(log))


def match_command(root, net, base, games, concurrency, book, base_options=None):
    command = [str(root/'sprt'),'-new',str(root/'ngn'),'-base',str(base),'-tc','10+0.1',
               '-concurrency',str(concurrency),'-maxgames',str(games),'-mingames','1000000',
               '-maxmoves','400','-gametimeout','180','-openings',str(book),'-lowpower=false']
    for side, options in [('new',['EvalFile='+str(net),'UseNNUE=true','Hash=64','Threads=1','OwnBook=false']),
                           ('base',base_options)]:
        for option in options:
            command += ['-'+side+'option',option]
    return command


def orientation(root, net, env):
    original = json.loads(Path('/home/ehrli/ngn-data/net1-gates-20261006/orientation/result.json').read_text())
    book = root/'orientation-openings.txt'
    book.write_text('\n'.join(BOOK.read_text().splitlines()[:35])+'\n')
    results = []
    for anchor in original['metadata']['anchors']:
        assert sha(anchor['path']) == anchor['sha256']
        stage = 'orientation-'+anchor['name']
        command = match_command(root,net,anchor['path'],70,4,book,anchor['options'])
        timing = run(root,stage,command,env,1800)
        result = {**timing, **game_result(root/(stage+'.log'),70), 'anchor':anchor}
        w,d,l = result['wdl']; p=(w+d/2+.5)/71; m=35.5
        vp=((sum(c*(k/4)**2 for k,c in enumerate(result['penta']))+.25)/m-p*p)/m
        vr=(400/(math.log(10)*p*(1-p)))**2*vp
        result.update(smoothed_score=p,performance=anchor['label']+400*math.log10(p/(1-p)),variance=vr)
        results.append(result)
        write(root/'orientation-results.json',dict(anchors=results))
    def pool(rows):
        weight=sum(1/r['variance'] for r in rows)
        rating=sum(r['performance']/r['variance'] for r in rows)/weight
        return dict(elo=rating,elo_95_ci=[rating-1.96/math.sqrt(weight),rating+1.96/math.sqrt(weight)])
    write(root/'orientation-results.json',dict(anchors=results,three_anchor=pool(results[:3]),four_anchor=pool(results),
          caveat='nominal match-only orientation; excludes label, shared-opening covariance and clock-transfer error'))


def train_gate(root, epochs, positions, hidden):
    env = dict(os.environ,GOAMD64='v3',GOMAXPROCS='2',GOTOOLCHAIN='local',GOPROXY='off',OMP_NUM_THREADS='2',MKL_NUM_THREADS='2')
    assert sha(BASE) == BASE_SHA and sha(BOOK) == BOOK_SHA
    name = 'h'+str(hidden)
    out = root/name
    out.mkdir(exist_ok=False)
    command = ['taskset','-c','12,14','nice','-n','19',sys.executable,'-m','trainer.train','--data',str(CORPUS),'--data-format','archive',
               '--archive-holdout-modulus','1024','--archive-split-seed','20261006','--out',str(out),
               '--format','2','--hidden',str(hidden),'--buckets','8','--batch','16384','--lr','.001',
               '--epochs',str(epochs),'--positions-per-epoch',str(positions),'--wdl','.75','--k','400',
               '--decay','cosine','--min-lr-ratio','.1','--threads','2','--precision','bf16',
               '--seed','20261006','--keep-best-only']
    import platform
    metadata=dict(commit=subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),
                  script_sha256=sha(Path(__file__)),host=platform.node(),os=platform.platform(),
                  go_version=subprocess.check_output([GO,'version'],text=True).strip(),
                  torch_version=torch.__version__,gpu=torch.cuda.get_device_name(),
                  cpu_count=os.cpu_count(),started_utc=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                  corpus_digest_verified=json.loads((root/'corpus-digest.json').read_text()))
    write(root/('metadata-'+name+'.json'),metadata)
    write(root/('plan-'+name+'.json'),dict(command=command,training_budget='about 3 GPU hours',gate_games=400,
             gate_rule='paired CI lower > 0 with zero anomalies',deadline_utc='2026-10-07T10:55:00Z',
             corpus_sha256_from_manifest=json.loads(CORPUS.with_name('manifest.json').read_text())['train']['sha256']))
    timing = run(root,'train-'+name,command,env,14400)
    internal_stage(root, 'parity-'+name)
    selected = np.fromfile(root/'parity-records.ngnp',dtype=RECORD_DTYPE)
    parity = load_script('2026-10-06-net1-recipe-trials.py').parity(out,selected)
    history = json.loads((out/'history.json').read_text())
    assert len(history) == epochs
    checkpoint = torch.load(out/'best.pt',map_location='cpu',weights_only=True)
    internal_stage(root, 'gen0-holdout-'+name)
    model = NGNN(hidden,8).cuda()
    model.load_state_dict(checkpoint['model'])
    snapshot = Path('/home/ehrli/ngn-data/trials-20261006/snapshot.json')
    holdout_paths = json.loads(snapshot.read_text())['holdout']
    holdout_receipt = dict(snapshot_sha256=sha(snapshot),sources=[dict(path=p,sha256=sha(p),bytes=Path(p).stat().st_size)
                                                              for p in holdout_paths])
    validation = Shards(holdout_paths)
    gen0 = validation_metrics(model,validation,16384,'cuda',.75,400)
    del model
    torch.cuda.empty_cache()
    training = dict(**timing,**parity,best=min(history,key=lambda r:r['validation_loss']),gen0_holdout=gen0,
                    gen0_holdout_records=validation.count,gen0_holdout_receipt=holdout_receipt,epoch_count=epochs)
    write(root/('training-'+name+'.json'),training)
    for binary,package in [('ngn','.'),('sprt','./cmd/sprt')]:
        if not (root/binary).exists():
            run(root,'build-'+binary,[GO,'build','-p','2','-o',str(root/binary),package],env,300)
    write(root/'binary-digests.json',dict(ngn=sha(root/'ngn'),sprt=sha(root/'sprt'),base=sha(BASE),book=sha(BOOK)))
    # Counter 3.8 is a Windows binary; explicitly propagate its Go thread limit.
    match_env=dict(env,GOMAXPROCS='1',WSLENV=env.get('WSLENV','')+':GOMAXPROCS/w')
    baseline = BASE
    if hidden != 256:
        previous = json.loads((root/'gate-h256.json').read_text())
        assert previous['accepted']
        baseline = root/'h256/best.nnue'
    options=['EvalFile='+str(baseline),'UseNNUE=true','Hash=64','Threads=1','OwnBook=false']
    # Existing same-binary controls in the net1 gate receipts apply to unchanged runtime;
    # a fresh exact-setting null preflight is also run before the first T80 gate.
    if hidden == 256:
        null_options=['EvalFile='+str(BASE),'UseNNUE=true','Hash=64','Threads=1','OwnBook=false']
        command=match_command(root,BASE,root/'ngn',40,8,BOOK,null_options)
        run(root,'aa-preflight',command,match_env,600)
        null=game_result(root/'aa-preflight.log',40)
        assert null['elo_95_ci'][0] <= 0 <= null['elo_95_ci'][1]
        write(root/'aa-preflight.json',null)
    stage='gate-'+name
    command=match_command(root,out/'best.nnue',root/'ngn',400,8,BOOK,options)
    timing=run(root,stage,command,match_env,3600)
    gate={**timing,**game_result(root/(stage+'.log'),400),'baseline':str(baseline),'baseline_sha256':sha(baseline)}
    gate['accepted']=gate['elo_95_ci'][0]>0
    write(root/(stage+'.json'),gate)
    print(json.dumps(dict(event='gate_completed',hidden=hidden,training=training,gate=gate)),flush=True)
    if gate['accepted'] and hidden == 256:
        orientation(root,out/'best.nnue',match_env)
    write(root/('completed-'+name+'.json'),dict(gate=gate,training=training,free_gib=disk(root,'completed-'+name)))
    write(root/'current-job.json',dict(stage='idle',pid=None))


def main():
    cli=argparse.ArgumentParser(description=__doc__)
    cli.add_argument('stage',choices=('prepare','run'))
    cli.add_argument('--root',type=Path,default=Path('/home/ehrli/ngn-data/t80-20261006'))
    cli.add_argument('--epochs',type=int,default=12)
    cli.add_argument('--positions-per-epoch',type=int,default=256_000_000)
    cli.add_argument('--hidden',type=int,choices=(256,512),default=256)
    args=cli.parse_args()
    args.root.mkdir(parents=True,exist_ok=True)
    configure_interop(args.root)
    torch.set_num_threads(2)
    torch.set_num_interop_threads(1)
    for signum in (signal.SIGTERM, signal.SIGINT, signal.SIGALRM):
        signal.signal(signum, interrupted)
    signal.alarm(max(1, int(DEADLINE-time.time())))
    if args.stage == 'prepare':
        prepare(args.root)
    else:
        train_gate(args.root,args.epochs,args.positions_per_epoch,args.hidden)


if __name__ == '__main__':
    main()
