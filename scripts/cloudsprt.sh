#!/usr/bin/env bash
# cloudsprt.sh — disposable AWS spot workers for SPRT runs (2026-06-11).
#
# Fire-and-forget Tier-1/Tier-2 gates on ~$0.10/hr Graviton spot instances so
# the Mac carries zero load. Every worker has THREE shutdown paths:
#   1. hard TTL at boot (user-data `shutdown -h +N`) — fires even if the run wedges;
#      one-time spot instances TERMINATE on OS shutdown, root EBS is delete-on-term
#   2. self-shutdown on run completion (after the final S3 log upload)
#   3. `cloudsprt.sh kill` sweep + `ps` audit (tag app=cloudsprt)
# Worst-case leak = the TTL cap (default 6h x spot price ~= $0.75).
#
# Logs upload to S3 every 60s + at exit, so results survive spot eviction AND
# the Mac sleeping. Workers need no AWS credentials (instance profile, PutObject
# only). Engines run with -lowpower=false (taskpolicy is macOS-only).
#
#   setup   one-time, idempotent: per-region keypair + security group, S3 bucket (30d), IAM
#   smoke   ~$0.001 end-to-end proof: nano spot up -> S3 write -> SELF-shutdown observed
#   launch  cross-compile worktree (cand) + HEAD/-baseref (base) for linux/arm64, shard the
#           openings ACROSS REGIONS, start N workers; default args = Tier-1 gate. -supervise
#           relaunches reclaimed shards in a rotated region until DONE (run it backgrounded).
#   status  per-shard PHASE across ALL regions, the definitive "hung or working?" check:
#           STARTING / PROGRESSING (+age of last S3 upload) / STALE=>LIKELY HUNG / DONE (S3 lags <=1 min)
#   fetch   pool shard logs -> elo/CI/LLR; verdict+sweep once NO live workers remain (all regions)
#   ps      ALL cloudsprt instances in any state, ALL regions (audit — empty when idle)
#   kill    terminate instances across all regions (all, or -runid X)
#   prices  current spot $/hr for the candidate Graviton types (all regions)
#
# Examples:
#   scripts/cloudsprt.sh launch                                   # Tier-1, 1 worker
#   scripts/cloudsprt.sh launch -shards 3 -- -tc 10+0.1 -elo0 -5 -elo1 0 -mingames 64 -maxgames 3000
#   scripts/cloudsprt.sh fetch -runid 260611-120000
#
# Pool shards only with shards (same instance type) — never mix cloud and Mac
# games in one verdict. Graviton types only (binaries are linux/arm64).

set -euo pipefail

APP=cloudsprt
die() { echo "cloudsprt: $*" >&2; exit 1; }
command -v aws >/dev/null 2>&1 || die "aws CLI not installed (brew install awscli)"

REGION=${AWS_REGION:-$(aws configure get region 2>/dev/null || true)}
[ -n "$REGION" ] || REGION=us-east-1
# Multi-region spot spread (#3). PRIMARY = $REGION: the S3 bucket lives there and workers in
# ANY region upload to it cross-region (instance-profile PutObject on cloudsprt-* covers it,
# so there is no per-region bucket). The sweep/audit layers (sweep_runid, fetch live-gate, ps,
# kill) iterate ALL of REGIONS so no region can leak; the per-instance TTL is the ultimate
# backstop if a region is transiently unreachable during a sweep. Each region gets its OWN
# keypair (the key allows CreateKeyPair but NOT ImportKeyPair — an IAM boundary), mapped to a
# local pem by pem_for(). Default drops us-east-1 (no default VPC on this account); override
# the list via CLOUDSPRT_REGIONS. us-east-2 + us-west-2 + ca-central-1 are verified usable.
REGIONS=${CLOUDSPRT_REGIONS:-"$REGION us-west-2 ca-central-1"}
REGIONS=$(echo "$REGIONS" | tr ' ' '\n' | awk 'NF && !seen[$0]++' | tr '\n' ' ')
A()  { aws --region "$REGION" --output text "$@"; }                # primary region (bucket, default)
AR() { local r=$1; shift; aws --region "$r" --output text "$@"; }  # explicit region (multi-region sweeps)
PEM=$HOME/.ssh/$APP.pem                                                               # primary-region pem (legacy path)
pem_for() { [ "$1" = "$REGION" ] && echo "$PEM" || echo "$HOME/.ssh/$APP-$1.pem"; }   # region -> local pem
SSHOPTS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=5 -o ServerAliveInterval=5 -o ServerAliveCountMax=3)
REPO=$(cd "$(dirname "$0")/.." && pwd)

acct()   { A sts get-caller-identity --query Account; }
bucket() { echo "$APP-$(acct)-$REGION"; }
ami()    { aws --region "${1:-$REGION}" --output text ssm get-parameter --name /aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64 --query Parameter.Value; }
sgid()   { aws --region "${1:-$REGION}" --output text ec2 describe-security-groups --filters Name=group-name,Values=$APP --query 'SecurityGroups[0].GroupId' 2>/dev/null || echo None; }

cmd_setup() {
    local acctid
    acctid=$(acct) || die "no AWS credentials — run: aws configure"
    echo "account $acctid, region $REGION"

    # Keypair: one PER REGION. The least-privilege key allows ec2:CreateKeyPair but NOT
    # ImportKeyPair (verified UnauthorizedOperation — a design boundary; do NOT broaden IAM),
    # so each region gets its OWN create-key-pair material saved to a region-suffixed pem and
    # pem_for() maps region -> local pem (primary keeps the legacy $PEM path). A region where
    # create fails (no access) just prints a WARN and is skipped at launch. (#3)
    local r pem
    for r in $REGIONS; do
        pem=$(pem_for "$r")
        if ! AR "$r" ec2 describe-key-pairs --key-names $APP >/dev/null 2>&1; then
            if AR "$r" ec2 create-key-pair --key-name $APP --query KeyMaterial > "$pem" 2>/dev/null; then
                chmod 400 "$pem"; echo "keypair: created in $r ($pem)"
            else
                rm -f "$pem"; echo "keypair: WARN cannot create in $r (no access) — skipped at launch"
            fi
        elif [ ! -f "$pem" ]; then
            AR "$r" ec2 delete-key-pair --key-name $APP >/dev/null 2>&1 || true
            if AR "$r" ec2 create-key-pair --key-name $APP --query KeyMaterial > "$pem" 2>/dev/null; then
                chmod 400 "$pem"; echo "keypair: recreated in $r (local pem was missing)"
            else
                rm -f "$pem"; echo "keypair: WARN cannot recreate in $r — skipped at launch"
            fi
        else
            echo "keypair: ok in $r ($pem)"
        fi
    done

    # Security group: one per region (a default-VPC SG is region-scoped). Skip regions with no
    # default VPC or no access; launch only uses regions that have an SG created here.
    local vpc sg
    for r in $REGIONS; do
        vpc=$(AR "$r" ec2 describe-vpcs --filters Name=is-default,Values=true --query 'Vpcs[0].VpcId' 2>/dev/null || echo None)
        if [ "$vpc" = "None" ]; then echo "security group: WARN no default VPC in $r (skipping)"; continue; fi
        sg=$(sgid "$r")
        if [ "$sg" = "None" ]; then
            sg=$(AR "$r" ec2 create-security-group --group-name $APP --description "$APP disposable spot workers" --vpc-id "$vpc" --query GroupId)
            AR "$r" ec2 authorize-security-group-ingress --group-id "$sg" --protocol tcp --port 22 --cidr 0.0.0.0/0 >/dev/null
            echo "security group: created in $r ($sg, ssh key-only)"
        else
            echo "security group: ok in $r ($sg)"
        fi
    done

    local b; b=$(bucket)
    if ! aws --region "$REGION" s3api head-bucket --bucket "$b" >/dev/null 2>&1; then
        if [ "$REGION" = "us-east-1" ]; then
            A s3api create-bucket --bucket "$b" >/dev/null
        else
            A s3api create-bucket --bucket "$b" --create-bucket-configuration LocationConstraint="$REGION" >/dev/null
        fi
        A s3api put-bucket-lifecycle-configuration --bucket "$b" --lifecycle-configuration \
            '{"Rules":[{"ID":"expire","Status":"Enabled","Filter":{},"Expiration":{"Days":30}}]}'
        echo "bucket: created (s3://$b, 30-day expiry)"
    else
        echo "bucket: ok (s3://$b)"
    fi

    # Worker role/profile are normally pre-created by scripts/cloudsprt_bootstrap.sh
    # (run as root in CloudShell) so the operator key here needs NO iam-write perms.
    if aws iam get-role --role-name $APP-role >/dev/null 2>&1; then
        echo "iam role: ok"
    elif aws iam create-role --role-name $APP-role --assume-role-policy-document \
            '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}' >/dev/null 2>&1; then
        aws iam put-role-policy --role-name $APP-role --policy-name s3put --policy-document \
            "{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",\"Action\":\"s3:PutObject\",\"Resource\":\"arn:aws:s3:::$APP-*/*\"}]}"
        echo "iam role: created"
    else
        die "iam role missing and this key can't create it — run scripts/cloudsprt_bootstrap.sh in CloudShell first"
    fi
    if aws iam get-instance-profile --instance-profile-name $APP-profile >/dev/null 2>&1; then
        echo "instance profile: ok"
    elif aws iam create-instance-profile --instance-profile-name $APP-profile >/dev/null 2>&1; then
        aws iam add-role-to-instance-profile --instance-profile-name $APP-profile --role-name $APP-role
        echo "instance profile: created (allow ~10s to propagate)"
    else
        die "instance profile missing and this key can't create it — run scripts/cloudsprt_bootstrap.sh in CloudShell first"
    fi
    echo "setup complete — next: scripts/cloudsprt.sh smoke"
}

# launch_one REGION TYPE TTL_MINUTES RUNID SHARD -> instance id
# Resolves the AMI + security group + default subnets for REGION (each region has its own),
# then iterates that region's subnets (one per AZ) on spot-capacity misses.
launch_one() {
    local rgn=$1 ty=$2 ttlm=$3 runid=$4 shard=$5
    local ud id try subnets sn snarg arr n rot rotated k ami_id sg_id
    ami_id=$(ami "$rgn" 2>/dev/null) || { echo "launch_one: no AMI in $rgn" >&2; return 1; }
    sg_id=$(sgid "$rgn")
    [ "$sg_id" != "None" ] || { echo "launch_one: no security group in $rgn (run setup)" >&2; return 1; }
    subnets=$(AR "$rgn" ec2 describe-subnets --filters Name=default-for-az,Values=true --query 'Subnets[].SubnetId' 2>/dev/null)
    [ -n "$subnets" ] || subnets=DEFAULT
    # Spread shards across AZs: rotate the AZ (subnet) preference list by shard number so a
    # multi-shard run prefers DIFFERENT AZs (independent shards = zero inter-shard cost;
    # de-correlates spot reclaims, which hit by AZ+type — observed both 4xlarge taken together
    # in us-east-2a). Each shard still falls back to the other AZs on a capacity miss;
    # single-shard rotation is a no-op. Cross-REGION spread is layered on top by cmd_launch.
    if [ "$subnets" != DEFAULT ]; then
        arr=($subnets); n=${#arr[@]}; rot=$(( shard % n )); rotated=""
        for ((k = 0; k < n; k++)); do rotated="$rotated ${arr[$(((k + rot) % n))]}"; done
        subnets=$rotated
    fi
    ud=$(mktemp)
    printf '#!/bin/bash\nshutdown -h +%s\n' "$ttlm" > "$ud"
    for try in 1 2 3 4 5 6; do
        for sn in $subnets; do
            snarg=""
            [ "$sn" != DEFAULT ] && snarg="--subnet-id $sn"
            # shellcheck disable=SC2086
            if id=$(AR "$rgn" ec2 run-instances --image-id "$ami_id" --instance-type "$ty" --key-name $APP \
                    --security-group-ids "$sg_id" --iam-instance-profile Name=$APP-profile $snarg \
                    --instance-market-options 'MarketType=spot,SpotOptions={SpotInstanceType=one-time,InstanceInterruptionBehavior=terminate}' \
                    --user-data "file://$ud" \
                    --tag-specifications "ResourceType=instance,Tags=[{Key=app,Value=$APP},{Key=runid,Value=$runid},{Key=shard,Value=$shard},{Key=region,Value=$rgn},{Key=Name,Value=$APP-$runid-s$shard}]" \
                    --query 'Instances[0].InstanceId' 2>/tmp/cloudsprt_err.$$); then
                rm -f "$ud"; echo "$id"; return 0
            fi
            grep -qi 'capacity' /tmp/cloudsprt_err.$$ && continue          # try next AZ
            if grep -qi 'instance profile' /tmp/cloudsprt_err.$$; then     # IAM propagation
                sleep 5; continue 2
            fi
            break 2                                                        # real error
        done
        break  # every AZ lacked capacity — caller decides (other type/region)
    done
    cat /tmp/cloudsprt_err.$$ >&2; rm -f "$ud" /tmp/cloudsprt_err.$$
    return 1
}

# sweep_runid RUNID — terminate every worker of a run (idempotent, never fails)
sweep_runid() {
    local r ids
    for r in $REGIONS; do
        ids=$(AR "$r" ec2 describe-instances --filters Name=tag:app,Values=$APP "Name=tag:runid,Values=$1" \
            Name=instance-state-name,Values=pending,running --query 'Reservations[].Instances[].InstanceId' 2>/dev/null || true)
        # shellcheck disable=SC2086
        [ -n "$ids" ] && AR "$r" ec2 terminate-instances --instance-ids $ids >/dev/null 2>&1 || true
    done
}

# wait_ssh REGION INSTANCE_ID -> public ip once sshable (fails fast if the instance dies first).
# Heartbeats go to STDERR every ~15s so the launch log NEVER sits silently frozen at "waiting for
# ssh" (that silence is exactly what made a healthy provisioning box indistinguishable from a hung
# one). stdout stays clean: the final `echo "$ip"` is the captured return value.
wait_ssh() {
    local rgn=$1 iid=$2 ip="" st n=0 ssho=(-i "$(pem_for "$1")" "${SSHOPTS[@]}")
    local tag="[$iid $rgn]"
    while :; do
        st=$(AR "$rgn" ec2 describe-instances --instance-ids "$iid" --query 'Reservations[0].Instances[0].State.Name' 2>/dev/null || echo unknown)
        case $st in shutting-down|terminated) echo "$tag died ($st) after $((n*5))s of ssh-wait" >&2; return 1;; esac
        ip=$(AR "$rgn" ec2 describe-instances --instance-ids "$iid" --query 'Reservations[0].Instances[0].PublicIpAddress' 2>/dev/null || true)
        [ -n "$ip" ] && [ "$ip" != "None" ] && break
        [ $((n % 3)) = 0 ] && echo "$tag ssh-wait $((n*5))s: state=$st, awaiting public ip..." >&2
        n=$((n+1)); [ $n -gt 30 ] && { echo "$tag gave up: no public ip after 150s" >&2; return 1; }
        sleep 5
    done
    echo "$tag ip=$ip (state=$st) — probing sshd..." >&2
    n=0
    until ssh "${ssho[@]}" "ec2-user@$ip" true 2>/dev/null; do
        { [ $((n % 3)) = 0 ] && [ "$n" -gt 0 ]; } && echo "$tag sshd not ready yet ($((n*5))s, ip=$ip)..." >&2
        n=$((n+1)); [ $n -gt 30 ] && { echo "$tag gave up: sshd never came up after 150s (ip=$ip)" >&2; return 1; }
        sleep 5
    done
    echo "$tag sshd UP after $((n*5))s (ip=$ip)" >&2
    echo "$ip"
}

cmd_smoke() {
    local b runid id ip state t ssho=(-i "$PEM" "${SSHOPTS[@]}")
    b=$(bucket); SG_ID=$(sgid)
    [ "$SG_ID" != "None" ] || die "run setup first"
    runid=smoke-$(date +%H%M%S)
    local ty
    id=""
    for ty in t4g.nano t4g.micro c7g.xlarge c8g.xlarge; do
        echo "launching $ty spot in $REGION (TTL 20 min)..."
        if id=$(launch_one "$REGION" "$ty" 20 "$runid" 0); then break; fi
    done
    [ -n "$id" ] || die "no spot capacity on any smoke type in $REGION (or quota — Service Quotas -> 'All Standard Spot Instance Requests')"
    echo "instance $id — waiting for ssh (~1 min)..."
    ip=$(wait_ssh "$REGION" "$id") || { A ec2 terminate-instances --instance-ids "$id" >/dev/null; die "ssh never came up (instance terminated)"; }
    echo "up at $ip — S3 marker write + self-shutdown..."
    ssh "${ssho[@]}" "ec2-user@$ip" \
        "sudo shutdown -h +20 >/dev/null 2>&1; for i in 1 2 3 4 5 6 7 8 9 10; do echo SMOKE_OK | aws s3 cp - s3://$b/$runid/ok.txt >/dev/null 2>&1 && break; sleep 3; done; sudo shutdown -h now" || true
    t=0
    until aws --region "$REGION" s3api head-object --bucket "$b" --key "$runid/ok.txt" >/dev/null 2>&1; do
        t=$((t+1)); [ $t -gt 24 ] && { A ec2 terminate-instances --instance-ids "$id" >/dev/null; die "S3 marker never appeared (instance terminated) — instance-profile problem?"; }
        sleep 5
    done
    echo "S3 upload: OK (instance-profile creds work)"
    t=0
    while :; do
        state=$(A ec2 describe-instances --instance-ids "$id" --query 'Reservations[0].Instances[0].State.Name')
        case $state in shutting-down|terminated) break;; esac
        t=$((t+1)); [ $t -gt 36 ] && { A ec2 terminate-instances --instance-ids "$id" >/dev/null; die "instance did NOT self-shutdown (force-terminated) — do not trust launch until investigated"; }
        sleep 5
    done
    echo "self-shutdown -> $state: OK — the shutdown guarantee holds"
    A ec2 terminate-instances --instance-ids "$id" >/dev/null 2>&1 || true
    echo "SMOKE PASS (cost ~\$0.001)"
}

# provision_shard SHARD INSTANCE_ID REGION -> 0 on success (touches "$work/live.SHARD"), 1 on
# failure (terminates+drops the instance). Reuses the run-dir binaries/openings; reads
# $b $runid $conc $ttlmin $work $openings $shards $REGION sprtargs from the (dynamically-scoped)
# caller. Used by BOTH the initial parallel launch AND the -supervise reclaim-recovery loop.
provision_shard() {
    local i=$1 id=$2 rgn=$3 ip nopen pushed alive _
    local ssho=(-i "$(pem_for "$rgn")" "${SSHOPTS[@]}")   # this shard's region keypair
    awk "NR % $shards == $i" "$openings" > "$work/open$i.txt"
    cat > "$work/run$i.sh" <<EOF
#!/bin/bash
cd /home/ec2-user
sudo shutdown -h +$ttlmin >/dev/null 2>&1   # second TTL path, independent of user-data
: > sprt.log   # create the log NOW so the upload-first keeper signals "worker started" within seconds
# Upload-FIRST then sleep (was: sleep-then-upload, which hid the worker for a full 60s): status can
# now decide STARTING-vs-HUNG almost immediately after run.sh begins instead of waiting a minute.
(while :; do aws s3 --region $REGION cp sprt.log s3://$b/$runid/shard$i.log >/dev/null 2>&1; sleep 60; done) &
KEEPER=\$!
# Spot-reclaim watcher (#4): on the 2-min interruption notice, force a final S3 flush so the
# last games aren't lost to the 60s keeper gap (IMDSv2 token required; AL2023 default).
(while sleep 5; do
   TOK=\$(curl -sX PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 60" 2>/dev/null)
   ACT=\$(curl -s -H "X-aws-ec2-metadata-token: \$TOK" "http://169.254.169.254/latest/meta-data/spot/instance-action" 2>/dev/null)
   case "\$ACT" in *terminate*|*stop*) aws s3 --region $REGION cp sprt.log s3://$b/$runid/shard$i.log >/dev/null 2>&1; break;; esac
 done) &
WATCHER=\$!
# Reserve one core for the control plane (sshd + this loop + the S3 uploader) so a
# fully-loaded box stays reachable for diagnosis/kill: pin sprt and its engine children
# to all-but-one core via taskset. nproc-1 cores for the games, the top core for system.
NCPU=\$(nproc)
taskset -c 0-\$(( NCPU > 1 ? NCPU - 2 : 0 )) ./sprt -new ./ngn_cand -base ./ngn_base -lowpower=false -concurrency $conc -openings open.txt ${sprtargs[*]} > sprt.log 2>&1
kill \$KEEPER \$WATCHER 2>/dev/null
echo CLOUDSPRT_DONE >> sprt.log
aws s3 --region $REGION cp sprt.log s3://$b/$runid/shard$i.log
sudo shutdown -h now
EOF
    echo "shard $i ($rgn): waiting for ssh..."
    if ! ip=$(wait_ssh "$rgn" "$id"); then
        AR "$rgn" ec2 terminate-instances --instance-ids "$id" >/dev/null 2>&1 || true
        echo "shard $i ($rgn): gone before provisioning (spot reclaim) — terminated, dropped"
        return 1
    fi
    # Push files with RETRY + VERIFY (boot-window ssh/scp flakiness silently starved later
    # shards of open.txt -> 0 games). Retry 4x and VERIFY open.txt's line count before starting.
    nopen=$(wc -l < "$work/open$i.txt" | tr -d ' ')
    pushed=0
    for _ in 1 2 3 4; do
        if scp "${ssho[@]}" "$work/sprt" "$work/ngn_cand" "$work/ngn_base" "ec2-user@$ip:" >/dev/null 2>&1 \
           && scp "${ssho[@]}" "$work/open$i.txt" "ec2-user@$ip:open.txt" >/dev/null 2>&1 \
           && scp "${ssho[@]}" "$work/run$i.sh" "ec2-user@$ip:run.sh" >/dev/null 2>&1 \
           && [ "$(ssh "${ssho[@]}" "ec2-user@$ip" 'wc -l < open.txt' 2>/dev/null | tr -d ' ')" = "$nopen" ]; then
            pushed=1; break
        fi
        sleep 5
    done
    if [ "$pushed" = 0 ]; then
        AR "$rgn" ec2 terminate-instances --instance-ids "$id" >/dev/null 2>&1 || true
        echo "shard $i ($rgn): openings/binaries push failed after retries — terminated, dropped"
        return 1
    fi
    # Start run.sh fully detached (setsid + </dev/null) and return immediately. The old
    # `nohup ./run.sh & sleep 1` held the ssh connection open while sprt's conc-N engine children
    # ramped up; that load spike got the connection reset ("closed by remote host") so ssh returned
    # 255 and the box was falsely "terminated, dropped" — yet run.sh (detached) kept running and
    # COMPLETED the SPRT (observed 2026-06-15: two shards reported "start failed" finished 1900+ games
    # each). A nonzero start-ssh is now LOG-ONLY (mirrors the liveness probe below): the run.sh
    # self-shutdown + the two TTLs stay the no-leak backstops, and the shard still counts as live so
    # the launch output matches reality.
    if ! ssh "${ssho[@]}" "ec2-user@$ip" 'chmod +x sprt ngn_cand ngn_base run.sh && setsid ./run.sh >/dev/null 2>&1 </dev/null &'; then
        echo "shard $i ($rgn): start-ssh returned nonzero (busy-box connection reset, not necessarily a real failure) — proceeding; S3 progress + TTL are the backstops"
    fi
    # Liveness probe — LOG ONLY, never terminate (a busy box's sshd can't always service a
    # fresh probe; the two boot/run TTLs + S3 progress are the real backstops).
    alive=0
    for _ in 1 2 3; do
        sleep 3
        if ssh "${ssho[@]}" "ec2-user@$ip" 'pgrep -f "\./sprt" >/dev/null' 2>/dev/null; then alive=1; break; fi
    done
    [ "$alive" = 1 ] || echo "shard $i: liveness unconfirmed (busy box / slow sshd) — proceeding; S3 progress + TTL are the real backstops"
    : > "$work/live.$i"   # survivor marker
    echo "shard $i ($rgn): running on $ip"
    return 0
}

# supervise_run — bounded, leak-safe reclaim recovery (#4). Polls the run; any shard that has
# died WITHOUT writing CLOUDSPRT_DONE (reclaimed mid-run) is relaunched in a ROTATED region/AZ,
# reusing the retained run-dir binaries (discard-and-replay: the replacement re-runs the slice
# fresh and OVERWRITES shard$i.log; NGN is deterministic so completed games are identical => no
# duplicates pooled). Bounded by $maxrepl total replacements AND $superwait minutes. EVERY
# replacement is runid+shard-tagged + boot -ttl via launch_one, the region-aware sweep covers it,
# and the per-instance TTL backstops a supervisor death — so this loop CANNOT leak. Reads
# cmd_launch locals via dynamic scope; never sweeps (fetch owns the verdict-time sweep).
supervise_run() {
    local rounds=0 maxrounds=$(( superwait * 2 )) repl=0 i r liveids newrgn newid pending
    echo "supervising $runid for reclaims (<=$maxrepl replacements, <=${superwait}m, rotate region on loss)..."
    while [ "$rounds" -lt "$maxrounds" ]; do
        sleep 30; rounds=$((rounds+1)); pending=0
        for ((i = 0; i < shards; i++)); do
            # DONE? the completion marker is in the last bytes of the shard log on S3
            if aws --region "$REGION" s3api get-object --bucket "$b" --key "$runid/shard$i.log" --range "bytes=-256" "/tmp/cloudsprt_done.$$" >/dev/null 2>&1 \
               && grep -q CLOUDSPRT_DONE "/tmp/cloudsprt_done.$$" 2>/dev/null; then
                continue
            fi
            pending=1
            # LIVE? an instance for this shard is pending/running in any region -> leave it alone
            liveids=""
            for r in $REGIONS; do
                liveids="$liveids $(AR "$r" ec2 describe-instances --filters Name=tag:app,Values=$APP "Name=tag:runid,Values=$runid" "Name=tag:shard,Values=$i" Name=instance-state-name,Values=pending,running --query 'Reservations[].Instances[].InstanceId' 2>/dev/null || true)"
            done
            [ -n "$(echo "$liveids" | tr -d ' ')" ] && continue
            # DOWN (neither done nor live) -> relaunch in a rotated region if budget remains
            if [ "$repl" -ge "$maxrepl" ]; then
                echo "supervise: shard $i down, replacement budget ($maxrepl) spent — leaving to partial-final"; continue
            fi
            newrgn=${regs[$(( (i + repl + 1) % nreg ))]}
            repl=$((repl+1))
            echo "supervise: shard $i DOWN (reclaimed) — relaunch $repl/$maxrepl in $newrgn"
            rm -f "$work/live.$i"
            if newid=$(launch_one "$newrgn" "$chosen" "$ttlmin" "$runid" "$i"); then
                provision_shard "$i" "$newid" "$newrgn" || echo "supervise: shard $i replacement provision failed — retry next round if budget remains"
            else
                echo "supervise: no $chosen capacity for shard $i in $newrgn — retry next round"
            fi
        done
        [ "$pending" = 0 ] && { echo "supervise: all $shards shards DONE"; break; }
    done
    rm -f "/tmp/cloudsprt_done.$$"
    [ "$rounds" -lt "$maxrounds" ] || echo "supervise: ${superwait}m cap reached — stopping (every worker still bounded by its boot -ttl)"
}

cmd_launch() {
    local shards=1 type=c8g.2xlarge ttl=6 conc=0 baseref=HEAD openings=$REPO/output/sprt_openings.txt nullrun=0 supervise=0 maxrepl=8 superwait=0
    while [ $# -gt 0 ]; do
        case $1 in
            -shards)    shards=$2; shift 2;;
            -type)      type=$2; shift 2;;
            -ttl)       ttl=$2; shift 2;;
            -conc)      conc=$2; shift 2;;
            -baseref)   baseref=$2; shift 2;;
            -openings)  openings=$2; shift 2;;
            -null)      nullrun=1; shift;;
            -supervise) supervise=1; shift;;
            -maxrepl)   maxrepl=$2; shift 2;;
            -superwait) superwait=$2; shift 2;;
            --) shift; break;;
            *) die "unknown launch flag $1 (sprt args go after --)";;
        esac
    done
    [ "$superwait" = 0 ] && superwait=$((ttl * 60))   # default: supervise across the whole TTL window
    local sprtargs=("$@")
    [ ${#sprtargs[@]} -gt 0 ] || sprtargs=(-tc 10+0.1 -elo0 -3 -elo1 3 -mingames 200 -maxgames 2000)
    [ -f "$openings" ] || die "openings file not found: $openings"
    case $type in
        c8g.*|c7g.*|m8g.*|m7g.*|r8g.*|r7g.*|t4g.*) ;;
        *) die "$type is not a Graviton type (binaries are linux/arm64)";;
    esac
    local b; b=$(bucket); SG_ID=$(sgid)
    [ "$SG_ID" != "None" ] || die "run setup first"
    local runid work
    runid=$(date +%y%m%d-%H%M%S)
    work=$(mktemp -d)
    # bake values in now (locals are gone when the EXIT trap fires)
    trap "git -C '$REPO' worktree remove --force '$work/basesrc' >/dev/null 2>&1 || true; git -C '$REPO' worktree prune >/dev/null 2>&1 || true" EXIT

    if [ "$nullrun" = 1 ]; then
        echo "building linux/arm64 NULL run (both engines = $baseref) + sprt (worktree)..."
        (cd "$REPO" && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o "$work/sprt" ./cmd/sprt)
        git -C "$REPO" worktree add --detach "$work/basesrc" "$baseref" >/dev/null
        (cd "$work/basesrc" && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o "$work/ngn_base" .)
        git -C "$REPO" worktree remove --force "$work/basesrc"
        cp "$work/ngn_base" "$work/ngn_cand"
    else
        echo "building linux/arm64: cand (worktree) + base ($baseref) + sprt..."
        (cd "$REPO" && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o "$work/ngn_cand" . \
                    && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o "$work/sprt" ./cmd/sprt)
        git -C "$REPO" worktree add --detach "$work/basesrc" "$baseref" >/dev/null
        (cd "$work/basesrc" && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o "$work/ngn_base" .)
        git -C "$REPO" worktree remove --force "$work/basesrc"
    fi

    # Launch all instances first, SPREAD ACROSS REGIONS (shard i -> REGIONS[i % nreg]) so a
    # multi-shard run de-correlates from a regional spot shortage / correlated reclaim. Shard 0
    # resolves the type through a capacity fallback chain in its region; remaining shards reuse
    # that type in THEIR region (shard pooling must be homogeneous — same type fleet-wide). A
    # shard lost to capacity/reclaim is dropped INDIVIDUALLY; the run continues with the
    # survivors. Zero survivors => sweep + die.
    local regs=($REGIONS) nreg rgn
    nreg=${#regs[@]}
    local i id ip ty chosen="" ids=() shardnos=() regionsof=() ttlmin=$((ttl * 60))
    for ((i = 0; i < shards; i++)); do
        rgn=${regs[$((i % nreg))]}
        if [ -z "$chosen" ]; then
            for ty in $type c7g.2xlarge c8g.4xlarge c7g.4xlarge; do
                if id=$(launch_one "$rgn" "$ty" "$ttlmin" "$runid" "$i"); then chosen=$ty; break; fi
                echo "no spot capacity: $ty in $rgn — trying next type"
            done
            [ -n "$chosen" ] || { sweep_runid "$runid"; die "no spot capacity on any allowed type in $rgn right now"; }
        else
            if ! id=$(launch_one "$rgn" "$chosen" "$ttlmin" "$runid" "$i"); then
                echo "shard $i ($rgn): no spot capacity for $chosen — dropped, continuing with the rest"
                continue
            fi
        fi
        ids+=("$id"); shardnos+=("$i"); regionsof+=("$rgn")
        echo "shard $i: $id ($chosen in $rgn)"
    done
    if [ "$conc" = 0 ]; then
        case $chosen in
            *.4xlarge) conc=15;;
            *.2xlarge) conc=7;;
            *.xlarge)  conc=3;;
            *)         conc=1;;
        esac
    fi
    # Provision shards IN PARALLEL. This used to be sequential, so one slow/dead box's
    # wait_ssh (up to ~5 min) blocked and timed out the WHOLE run — the launcher hung
    # for many minutes and accumulated zombie procs. Each shard now provisions in its
    # own background subshell (forked copy of these locals, read-only); a shard that
    # can't be reached drops ITSELF (terminate + no marker) without blocking the others,
    # and a survivor touches live.$i. Failure paths and the boot/run TTLs are unchanged,
    # so the no-leak guarantee holds (a half-provisioned box still self-terminates at TTL).
    local j
    for ((j = 0; j < ${#ids[@]}; j++)); do
        ( provision_shard "${shardnos[$j]}" "${ids[$j]}" "${regionsof[$j]}" ) &
    done
    wait   # block until every shard's parallel provisioning subshell has finished
    local live; live=$(find "$work" -maxdepth 1 -name 'live.*' 2>/dev/null | wc -l | tr -d ' ')
    [ "$live" -gt 0 ] || { sweep_runid "$runid"; die "zero shards survived provisioning — ALL $runid workers terminated"; }
    echo
    echo "runid $runid — $live/$shards shard(s) x $chosen conc-$conc, TTL ${ttl}h, sprt args: ${sprtargs[*]}"
    echo "  watch:  scripts/cloudsprt.sh status   # per-shard phase: STARTING / PROGRESSING / LIKELY HUNG / DONE"
    echo "  result: scripts/cloudsprt.sh fetch -runid $runid"
    # #4 reclaim recovery: when asked, supervise the run and relaunch reclaimed shards in a
    # rotated region (retained binaries) until DONE / budget / cap. Run launch -supervise in the
    # BACKGROUND so it keeps replacing while you work; every worker is still -ttl-bounded.
    if [ "$supervise" = 1 ]; then supervise_run; fi
    find "$work" -mindepth 1 -delete 2>/dev/null; rmdir "$work" 2>/dev/null || true
}

# cmd_status — the DEFINITIVE "is it hung or working?" check. Per shard, across ALL regions
# (status used to query only the primary region and was blind to 2/3 of a multi-region run),
# it classifies one of: STARTING (provisioning / pre-first-upload — normal) / PROGRESSING
# (with the age of the last S3 upload + the latest game line) / STALE => LIKELY HUNG (the box is
# live but its log stopped uploading) / DONE. The S3 upload age is the heartbeat: a live box that
# hasn't uploaded in >150s (uploads run every 60s) is wedged, full stop — no more guessing.
cmd_status() {
    local b r rid k tmpd; b=$(bucket); tmpd=$(mktemp -d)
    # 1) live instances in EVERY region (one JSON array per region; region recovered from filename)
    for r in $REGIONS; do
        aws --region "$r" --output json ec2 describe-instances \
            --filters Name=tag:app,Values=$APP Name=instance-state-name,Values=pending,running \
            --query "Reservations[].Instances[].{shard:Tags[?Key=='shard']|[0].Value,iid:InstanceId,ty:InstanceType,runid:Tags[?Key=='runid']|[0].Value,launch:LaunchTime,state:State.Name}" \
            > "$tmpd/inst.$r.json" 2>/dev/null || echo '[]' > "$tmpd/inst.$r.json"
    done
    local runids; runids=$(python3 -c "
import json,glob
s=set()
for f in glob.glob('$tmpd/inst.*.json'):
    try:
        for x in json.load(open(f)) or []:
            if x.get('runid'): s.add(x['runid'])
    except Exception: pass
print(' '.join(sorted(s)))")
    if [ -z "$runids" ]; then echo "no live cloudsprt instances in any region ($REGIONS)"; rm -rf "$tmpd"; return 0; fi
    # 2) S3 object metadata (key + LastModified + size) for each live run; the bucket is primary-region
    for rid in $runids; do
        aws --region "$REGION" --output json s3api list-objects-v2 --bucket "$b" --prefix "$rid/" \
            --query 'Contents[].{key:Key,lm:LastModified,size:Size}' > "$tmpd/s3.$rid.json" 2>/dev/null || echo '[]' > "$tmpd/s3.$rid.json"
    done
    # 3) download the tail of each *.log for the latest game line + the DONE marker
    python3 -c "
import json,glob
for f in glob.glob('$tmpd/s3.*.json'):
    try:
        for o in json.load(open(f)) or []:
            if str(o.get('key','')).endswith('.log'): print(o['key'])
    except Exception: pass" > "$tmpd/keys.txt"
    while IFS= read -r k; do
        [ -n "$k" ] || continue
        aws --region "$REGION" --output text s3api get-object --bucket "$b" --key "$k" --range "bytes=-1024" "$tmpd/tail.$(basename "$k")" >/dev/null 2>&1 || true
    done < "$tmpd/keys.txt"
    # 4) the verdict report — pure formatting + time math, deterministic
    python3 - "$tmpd" <<'PY'
import sys, os, re, json, glob, datetime
tmpd=sys.argv[1]
now=datetime.datetime.now(datetime.timezone.utc)
def age(iso):
    if not iso: return None
    try: return (now-datetime.datetime.fromisoformat(str(iso).replace('Z','+00:00'))).total_seconds()
    except Exception: return None
def fmt(s):
    if s is None: return "?"
    s=int(s); m,sec=divmod(s,60); h,m=divmod(m,60)
    return f"{h}h{m:02d}m" if h else (f"{m}m{sec:02d}s" if m else f"{sec}s")
inst=[]
for f in glob.glob(os.path.join(tmpd,'inst.*.json')):
    region=os.path.basename(f)[5:-5]
    try: arr=json.load(open(f)) or []
    except Exception: arr=[]
    for x in arr: x['region']=region; inst.append(x)
s3={}
for f in glob.glob(os.path.join(tmpd,'s3.*.json')):
    try: arr=json.load(open(f)) or []
    except Exception: arr=[]
    for o in arr: s3[o['key']]=(age(o.get('lm')), o.get('size') or 0)
from collections import defaultdict
byrun=defaultdict(list)
for x in inst: byrun[x.get('runid') or '?'].append(x)
UP=60; STALE=150; GRACE=420   # GRACE = ssh-wait(~5m)+scp before a worker log is even expected
for runid,xs in sorted(byrun.items()):
    ages=[a for a in (age(x.get('launch')) for x in xs) if a is not None]
    print(f"\n=== runid {runid} — {len(xs)} live instance(s), oldest launched {fmt(max(ages)) if ages else '?'} ago ===")
    for x in sorted(xs, key=lambda z: str(z.get('shard'))):
        n=x.get('shard'); ia=age(x.get('launch'))
        logkey=next((c for c in (f"{runid}/shard{n}.log", f"{runid}/gauntlet{n}.log") if c in s3), None)
        la,sz=s3.get(logkey,(None,0)) if logkey else (None,0)
        gline=""; done=False
        tf=os.path.join(tmpd,'tail.'+os.path.basename(logkey)) if logkey else None
        if tf and os.path.exists(tf):
            t=open(tf,errors='replace').read(); done='CLOUDSPRT_DONE' in t
            g=re.findall(r'^G\d+.*$',t,re.M)
            if g: gline=g[-1].strip()
        if done: v="DONE — worker finished, self-shutting-down"
        elif la is None:
            v=(f"!! LIKELY HUNG — no worker log {fmt(ia)} after launch (provisioning is done by ~7m); read the launch log, consider kill"
               if (ia is not None and ia>GRACE) else
               ".. STARTING — provisioning (ssh+scp) or pre-first-upload; a worker log is expected within ~7m of launch")
        elif la>STALE: v=f"!! LIKELY HUNG — log STALE: last S3 upload {fmt(la)} ago while the box is LIVE (uploads run every {UP}s)"
        else:
            g=gline or f"log {sz}B, no game line yet (sprt computing first games)"
            v=f"OK PROGRESSING — last upload {fmt(la)} ago | {g}"
        print(f"  shard {n}  {x.get('iid')}  {x.get('ty')}  {x.get('region')}  inst[{x.get('state')} {fmt(ia)}]")
        print(f"       {v}")
print()
PY
    rm -rf "$tmpd"
}

cmd_fetch() {
    local runid="" elo0=-3 elo1=3
    while [ $# -gt 0 ]; do
        case $1 in
            -runid) runid=$2; shift 2;;
            -elo0)  elo0=$2; shift 2;;
            -elo1)  elo1=$2; shift 2;;
            *) die "unknown fetch flag $1";;
        esac
    done
    [ -n "$runid" ] || die "usage: fetch -runid <id> [-elo0 -3] [-elo1 3]"
    local b dir live="" nlive=0
    b=$(bucket); dir=$REPO/output/cloud/$runid
    mkdir -p "$dir"
    aws --region "$REGION" s3 sync "s3://$b/$runid/" "$dir/" >/dev/null
    # Verdict gate = ZERO LIVE WORKERS, not all-shards-DONE: an evicted shard never
    # writes CLOUDSPRT_DONE, but its last 60s upload is pooled as partial-final.
    local r lr
    for r in $REGIONS; do
        lr=$(AR "$r" ec2 describe-instances --filters Name=tag:app,Values=$APP "Name=tag:runid,Values=$runid" \
            Name=instance-state-name,Values=pending,running --query 'Reservations[].Instances[].InstanceId' 2>/dev/null || true)
        live="$live $lr"
    done
    nlive=$(echo "$live" | wc -w | tr -d ' ')
    local rc=0
    python3 - "$dir" "$elo0" "$elo1" "$nlive" <<'PY' || rc=$?
import sys, os, re, math, glob
d, e0, e1, nlive = sys.argv[1], float(sys.argv[2]), float(sys.argv[3]), int(sys.argv[4])
W = D = L = 0; done = logs = 0
for f in sorted(glob.glob(os.path.join(d, "shard*.log"))):
    logs += 1
    txt = open(f, errors="replace").read()
    m = None
    for m in re.finditer(r"^G\d+\s+(\d+)W\s+(\d+)D\s+(\d+)L", txt, re.M):
        pass
    fin = "CLOUDSPRT_DONE" in txt
    done += fin
    if m:
        W += int(m.group(1)); D += int(m.group(2)); L += int(m.group(3))
    state = "done" if fin else ("RUNNING" if nlive else "EVICTED->partial-final")
    tail = m.group(0) if m else (txt.strip().splitlines()[-1] if txt.strip() else "(empty)")
    print(f'{os.path.basename(f)}: {state}  {tail}')
if logs == 0 or W + D + L == 0:
    if nlive:
        print(f"no pooled games yet — {nlive} live worker(s), uploads land every 60s")
        sys.exit(3)
    sys.exit("no games and no live workers — run never produced results")
n = W + D + L
s = (W + D / 2) / n
var = (W * (1 - s) ** 2 + D * (0.5 - s) ** 2 + L * s ** 2) / n
se = math.sqrt(var / n) if var > 0 else 0.0
def elo(x):
    x = min(max(x, 1e-9), 1 - 1e-9)
    return -400 * math.log10(1 / x - 1)
def lo(e):
    return 1 / (1 + 10 ** (-e / 400))
s0, s1 = lo(e0), lo(e1)
llr = 0.0 if var == 0 else (s1 - s0) * (2 * s - s0 - s1) / (2 * var) * n
print(f"\npooled: {n}g  {W}W {D}D {L}L  score {100*s:.1f}%  "
      f"elo {elo(s):+.1f} [{elo(s-1.96*se):+.1f},{elo(s+1.96*se):+.1f}]  "
      f"LLR {llr:+.2f}  (elo0 {e0} elo1 {e1}, bounds +/-2.94)")
if nlive:
    print(f"IN PROGRESS: {nlive} live worker(s) — NOT a verdict yet")
    sys.exit(3)
if done < logs:
    print(f"note: {logs-done} shard(s) evicted — pooled at last upload (complete games only; verdict is FINAL)")
PY
    if [ "$rc" = 0 ]; then
        sweep_runid "$runid"
        echo "(no live workers — verdict is final; sweep was an idempotent no-op if already self-terminated)"
    elif [ "$rc" = 3 ]; then
        echo "(run still in progress — workers stay up; TTL still bounds them)"
    else
        return "$rc"
    fi
}

# provision_gshard SHARD INSTANCE_ID REGION -> 0 on success (touches "$work/live.SHARD"), 1 on
# drop. The GAUNTLET analogue of provision_shard: ships ngn+gauntlet+anchors+ratings+openings and
# runs the gauntlet on this box's opening-slice ($ggames/anchor), uploading tally$i.json. The
# shutdown layers (boot/run TTL, completion self-shutdown, the IMDS reclaim flush) MIRROR
# provision_shard EXACTLY — keep them in sync; `smoke` + the calibration run cover both. Reads
# $b $runid $work $ttlmin $shards $ggames $gtc $gonly $andir $REGION from the dynamic scope.
provision_gshard() {
    local i=$1 id=$2 rgn=$3 ip pushed alive _
    local ssho=(-i "$(pem_for "$rgn")" "${SSHOPTS[@]}")
    awk "NR % $shards == $i" "$work/openings.txt" > "$work/gopen$i.txt"
    [ -s "$work/gopen$i.txt" ] || echo "" > "$work/gopen$i.txt"   # empty -> gauntlet uses built-in
    cat > "$work/grun$i.sh" <<EOF
#!/bin/bash
cd /home/ec2-user
sudo shutdown -h +$ttlmin >/dev/null 2>&1   # run.sh TTL path (mirrors provision_shard)
(while sleep 60; do aws s3 --region $REGION cp gauntlet.log s3://$b/$runid/gauntlet$i.log >/dev/null 2>&1; done) &
KEEPER=\$!
# Spot-reclaim watcher: on the 2-min notice, force-upload whatever tally exists (the gauntlet
# only writes tally.json at the end, so a reclaimed box usually contributes nothing — acceptable
# for v1; the run pools the survivors and -ttl bounds the box).
(while sleep 5; do
   TOK=\$(curl -sX PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 60" 2>/dev/null)
   ACT=\$(curl -s -H "X-aws-ec2-metadata-token: \$TOK" "http://169.254.169.254/latest/meta-data/spot/instance-action" 2>/dev/null)
   case "\$ACT" in *terminate*|*stop*) aws s3 --region $REGION cp tally.json s3://$b/$runid/tally$i.json >/dev/null 2>&1; break;; esac
 done) &
WATCHER=\$!
# One game ~= one busy core (engines single-threaded, turn-based). Reserve the top core for the
# control plane (sshd + uploader); run cores-1 concurrent games pinned to the rest.
NCPU=\$(nproc)
GCONC=\$(( NCPU > 1 ? NCPU - 1 : 1 ))
taskset -c 0-\$(( NCPU > 1 ? NCPU - 2 : 0 )) ./gauntlet -ngn ./ngn -anchors ratings.json -openings openings.txt -only $gonly -tc $gtc -concurrency \$GCONC -lowpower=false -games $ggames -no-record -tally-out tally.json > gauntlet.log 2>&1
kill \$KEEPER \$WATCHER 2>/dev/null
aws s3 --region $REGION cp tally.json s3://$b/$runid/tally$i.json
echo CLOUDSPRT_DONE >> gauntlet.log
aws s3 --region $REGION cp gauntlet.log s3://$b/$runid/gauntlet$i.log
sudo shutdown -h now   # completion self-shutdown (mirrors provision_shard)
EOF
    echo "box $i ($rgn): waiting for ssh..."
    if ! ip=$(wait_ssh "$rgn" "$id"); then
        AR "$rgn" ec2 terminate-instances --instance-ids "$id" >/dev/null 2>&1 || true
        echo "box $i ($rgn): gone before provisioning (spot reclaim) — terminated, dropped"
        return 1
    fi
    pushed=0
    for _ in 1 2 3 4; do
        if scp "${ssho[@]}" "$work/ngn" "$work/gauntlet" "$work/ratings.json" "$andir"/blunder_* "$andir"/counter_* "ec2-user@$ip:" >/dev/null 2>&1 \
           && scp "${ssho[@]}" "$work/gopen$i.txt" "ec2-user@$ip:openings.txt" >/dev/null 2>&1 \
           && scp "${ssho[@]}" "$work/grun$i.sh" "ec2-user@$ip:run.sh" >/dev/null 2>&1 \
           && ssh "${ssho[@]}" "ec2-user@$ip" 'test -s ngn && test -s gauntlet && test -s ratings.json' 2>/dev/null; then
            pushed=1; break
        fi
        sleep 5
    done
    if [ "$pushed" = 0 ]; then
        AR "$rgn" ec2 terminate-instances --instance-ids "$id" >/dev/null 2>&1 || true
        echo "box $i ($rgn): payload push failed after retries — terminated, dropped"
        return 1
    fi
    # See provision_shard: start detached + log-only (the `& sleep 1` start-ssh false-negative that
    # terminated healthy workers). run.sh self-shutdown + TTLs stay the no-leak backstops.
    if ! ssh "${ssho[@]}" "ec2-user@$ip" 'chmod +x ngn gauntlet blunder_* counter_* run.sh && setsid ./run.sh >/dev/null 2>&1 </dev/null &'; then
        echo "box $i ($rgn): start-ssh returned nonzero (busy-box connection reset, not necessarily a real failure) — proceeding; S3 result + TTL are the backstops"
    fi
    alive=0
    for _ in 1 2 3; do
        sleep 3
        if ssh "${ssho[@]}" "ec2-user@$ip" 'pgrep -f "\./gauntlet" >/dev/null' 2>/dev/null; then alive=1; break; fi
    done
    [ "$alive" = 1 ] || echo "box $i: liveness unconfirmed (busy box / slow sshd) — proceeding; S3 result + TTL are the real backstops"
    : > "$work/live.$i"
    echo "box $i ($rgn): running on $ip"
    return 0
}

# cmd_gauntlet — the CLOUD gauntlet: N boxes, each running cmd/gauntlet at concurrency=(cores-1)
# over its own opening-slice ($games/N games per anchor), pooled by gfetch. Reuses launch_one (boot
# TTL) + sweep_runid (failure/verdict sweep) UNCHANGED. Anchors ship flat with bare-path ratings.
cmd_gauntlet() {
    local shards=4 type=c8g.8xlarge ttl=2 games=40 gonly="v6.1.0,v7.2.0,v7.4.0,v8.0.0,counter-3.8" gtc="120+1" ref=HEAD
    while [ $# -gt 0 ]; do
        case $1 in
            -shards) shards=$2; shift 2;;
            -type)   type=$2; shift 2;;
            -ttl)    ttl=$2; shift 2;;
            -games)  games=$2; shift 2;;   # TOTAL games per anchor, split across the boxes
            -only)   gonly=$2; shift 2;;
            -tc)     gtc=$2; shift 2;;
            -ref)    ref=$2; shift 2;;
            *) die "unknown gauntlet flag $1";;
        esac
    done
    case $type in
        c8g.*|c7g.*|m8g.*|m7g.*|r8g.*|r7g.*|t4g.*) ;;
        *) die "$type is not a Graviton type (binaries are linux/arm64)";;
    esac
    local andir=$REPO/opponents/linux-arm64
    ls "$andir"/blunder_* "$andir"/counter_* >/dev/null 2>&1 || die "no linux anchors in $andir — run scripts/build-anchors.sh first"
    local b; b=$(bucket); SG_ID=$(sgid)
    [ "$SG_ID" != "None" ] || die "run setup first"
    local runid work; runid=$(date +%y%m%d-%H%M%S); work=$(mktemp -d)
    trap "git -C '$REPO' worktree remove --force '$work/ngnsrc' >/dev/null 2>&1 || true; git -C '$REPO' worktree prune >/dev/null 2>&1 || true" EXIT
    local ggames=$(( games / shards )); [ $(( ggames % 2 )) -eq 0 ] || ggames=$(( ggames + 1 )); [ "$ggames" -ge 2 ] || ggames=2

    echo "building linux/arm64: ngn ($ref) + gauntlet..."
    git -C "$REPO" worktree add --detach "$work/ngnsrc" "$ref" >/dev/null
    (cd "$work/ngnsrc" && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o "$work/ngn" .)
    git -C "$REPO" worktree remove --force "$work/ngnsrc"
    (cd "$REPO" && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o "$work/gauntlet" ./cmd/gauntlet)
    sed 's#opponents/#./#g' "$REPO/opponents/ratings.json" > "$work/ratings.json"   # cwd-relative anchor paths (./blunder_XXX) so exec finds the flat-shipped binaries
    cp "$REPO/output/sprt_openings.txt" "$work/openings.txt" 2>/dev/null || : > "$work/openings.txt"

    local regs=($REGIONS) nreg=${#regs[@]} i id ty chosen="" ids=() shardnos=() regionsof=() ttlmin=$((ttl * 60)) rgn
    for ((i = 0; i < shards; i++)); do
        rgn=${regs[$((i % nreg))]}
        if [ -z "$chosen" ]; then
            for ty in $type c7g.4xlarge c8g.4xlarge c7g.2xlarge; do
                if id=$(launch_one "$rgn" "$ty" "$ttlmin" "$runid" "$i"); then chosen=$ty; break; fi
                echo "no spot capacity: $ty in $rgn — trying next type"
            done
            [ -n "$chosen" ] || { sweep_runid "$runid"; die "no spot capacity on any allowed type in $rgn"; }
        else
            if ! id=$(launch_one "$rgn" "$chosen" "$ttlmin" "$runid" "$i"); then
                echo "box $i ($rgn): no spot capacity for $chosen — dropped, continuing"; continue
            fi
        fi
        ids+=("$id"); shardnos+=("$i"); regionsof+=("$rgn")
        echo "box $i: $id ($chosen in $rgn)"
    done
    local j
    for ((j = 0; j < ${#ids[@]}; j++)); do
        ( provision_gshard "${shardnos[$j]}" "${ids[$j]}" "${regionsof[$j]}" ) &
    done
    wait
    local live; live=$(find "$work" -maxdepth 1 -name 'live.*' 2>/dev/null | wc -l | tr -d ' ')
    [ "$live" -gt 0 ] || { sweep_runid "$runid"; die "zero gauntlet boxes survived provisioning — ALL $runid terminated"; }
    echo
    echo "runid $runid — $live/$shards box(es) x $chosen, ${ggames}g/anchor each (~$((ggames * live)) per anchor pooled), TC $gtc"
    echo "  anchors: $gonly"
    echo "  result:  scripts/cloudsprt.sh gfetch -runid $runid"
    find "$work" -mindepth 1 -delete 2>/dev/null; rmdir "$work" 2>/dev/null || true
}

# cmd_gfetch — pool the per-box tally JSONs into one CCRL rating (gauntlet -pool), gated on ZERO
# LIVE boxes (mirrors cmd_fetch's sweep gate); sweeps the runid when final.
cmd_gfetch() {
    local runid=""
    while [ $# -gt 0 ]; do
        case $1 in
            -runid) runid=$2; shift 2;;
            *) die "unknown gfetch flag $1";;
        esac
    done
    [ -n "$runid" ] || die "usage: gfetch -runid <id>"
    local b dir; b=$(bucket); dir=$REPO/output/cloud/$runid; mkdir -p "$dir"
    aws --region "$REGION" s3 sync "s3://$b/$runid/" "$dir/" >/dev/null
    local r lr live=""
    for r in $REGIONS; do
        lr=$(AR "$r" ec2 describe-instances --filters Name=tag:app,Values=$APP "Name=tag:runid,Values=$runid" \
            Name=instance-state-name,Values=pending,running --query 'Reservations[].Instances[].InstanceId' 2>/dev/null || true)
        live="$live $lr"
    done
    local nlive; nlive=$(echo "$live" | wc -w | tr -d ' ')
    local tallies; tallies=$(find "$dir" -maxdepth 1 -name 'tally*.json' 2>/dev/null | sort | paste -sd, -)
    if [ -z "$tallies" ]; then
        [ "$nlive" -gt 0 ] && { echo "no tallies yet — $nlive live box(es); results land at box completion (~end of run)"; return 0; }
        die "no tallies and no live boxes — the run produced nothing"
    fi
    echo "pooling $(echo "$tallies" | tr ',' '\n' | wc -l | tr -d ' ') box tally file(s):"
    (cd "$REPO" && go run ./cmd/gauntlet -pool "$tallies" -no-record)
    if [ "$nlive" -gt 0 ]; then
        echo "IN PROGRESS: $nlive live box(es) — partial pool, NOT a final verdict"
    else
        sweep_runid "$runid"
        echo "(no live boxes — verdict is final; sweep idempotent)"
    fi
}

cmd_ps() {
    local r
    for r in $REGIONS; do
        echo "== region $r =="
        aws --region "$r" ec2 describe-instances --filters Name=tag:app,Values=$APP \
            --query 'Reservations[].Instances[].[InstanceId,State.Name,InstanceType,Tags[?Key==`runid`]|[0].Value,LaunchTime]' \
            --output table 2>&1 || echo "  (region $r: query failed — disabled / no access?)"
    done
}

cmd_kill() {
    local r ids any=0 filters=("Name=tag:app,Values=$APP" "Name=instance-state-name,Values=pending,running")
    if [ "${1:-}" = "-runid" ]; then
        [ -n "${2:-}" ] || die "kill -runid <id>"
        filters+=("Name=tag:runid,Values=$2")
    fi
    for r in $REGIONS; do
        ids=$(aws --region "$r" --output text ec2 describe-instances --filters "${filters[@]}" --query 'Reservations[].Instances[].InstanceId' 2>/dev/null || true)
        [ -n "$ids" ] || continue
        any=1; echo "== region $r =="
        # shellcheck disable=SC2086
        aws --region "$r" --output text ec2 terminate-instances --instance-ids $ids --query 'TerminatingInstances[].[InstanceId,CurrentState.Name]'
    done
    [ "$any" = 1 ] || echo "nothing running"
}

cmd_prices() {
    local r
    for r in $REGIONS; do
        echo "== region $r =="
        aws --region "$r" ec2 describe-spot-price-history \
            --instance-types c8g.2xlarge c8g.4xlarge c7g.2xlarge c7g.4xlarge t4g.nano \
            --product-descriptions "Linux/UNIX" --start-time "$(date -u +%Y-%m-%dT%H:%M:%S)" \
            --query 'sort_by(SpotPriceHistory,&InstanceType)[].[InstanceType,AvailabilityZone,SpotPrice]' \
            --output table 2>&1 || echo "  (region $r: query failed — disabled / no access?)"
    done
}

case "${1:-help}" in
    setup)  shift; cmd_setup "$@";;
    smoke)  shift; cmd_smoke "$@";;
    launch) shift; cmd_launch "$@";;
    gauntlet) shift; cmd_gauntlet "$@";;
    gfetch) shift; cmd_gfetch "$@";;
    status) shift; cmd_status "$@";;
    fetch)  shift; cmd_fetch "$@";;
    ps)     shift; cmd_ps "$@";;
    kill)   shift; cmd_kill "$@";;
    prices) shift; cmd_prices "$@";;
    *) sed -n '2,36p' "$0" | sed 's/^# \{0,1\}//'; exit 1;;
esac
