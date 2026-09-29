#!/usr/bin/env python3
"""Run fastchess and retain live per-role child affinity/environment witnesses."""
from __future__ import annotations

import argparse
import json
import os
import signal
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

from common import CandidateMatchError, atomic_json, sha256, utc_now


def proc_stat(pid: int) -> tuple[int, int]:
    raw = Path(f"/proc/{pid}/stat").read_text(encoding="utf-8")
    close = raw.rfind(")")
    fields = raw[close + 2 :].split()
    return int(fields[1]), int(fields[2])  # ppid, pgid


def snapshot_processes() -> dict[int, dict[str, Any]]:
    rows: dict[int, dict[str, Any]] = {}
    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        pid = int(entry.name)
        try:
            ppid, pgid = proc_stat(pid)
            rows[pid] = {"pid": pid, "ppid": ppid, "pgid": pgid}
        except (FileNotFoundError, ProcessLookupError, PermissionError, ValueError, IndexError):
            continue
    return rows


def descendant_pids(root: int, rows: dict[int, dict[str, Any]]) -> set[int]:
    children: dict[int, list[int]] = {}
    for pid, row in rows.items():
        children.setdefault(row["ppid"], []).append(pid)
    result: set[int] = set()
    frontier = [root]
    while frontier:
        parent = frontier.pop()
        for child in children.get(parent, []):
            if child not in result:
                result.add(child)
                frontier.append(child)
    return result


def read_environment(pid: int) -> dict[str, str]:
    values = Path(f"/proc/{pid}/environ").read_bytes().split(b"\0")
    result = {}
    for item in values:
        if b"=" in item:
            key, value = item.split(b"=", 1)
            result[key.decode(errors="replace")] = value.decode(errors="replace")
    return result


def read_allowed(pid: int) -> str:
    for line in Path(f"/proc/{pid}/status").read_text(encoding="utf-8").splitlines():
        if line.startswith("Cpus_allowed_list:"):
            return line.split(":", 1)[1].strip()
    raise CandidateMatchError(f"pid {pid}: Cpus_allowed_list absent")


def read_process(pid: int, base: dict[str, Any]) -> dict[str, Any]:
    result = dict(base)
    result["exe"] = str(Path(f"/proc/{pid}/exe").resolve(strict=True))
    result["cwd"] = str(Path(f"/proc/{pid}/cwd").resolve(strict=True))
    result["cmdline"] = [part.decode(errors="replace") for part in Path(f"/proc/{pid}/cmdline").read_bytes().split(b"\0") if part]
    result["cpus_allowed_list"] = read_allowed(pid)
    result["gomaxprocs"] = read_environment(pid).get("GOMAXPROCS")
    result["task_count"] = sum(1 for item in Path(f"/proc/{pid}/task").iterdir() if item.name.isdigit())
    return result


def observation_violations(role: dict[str, Any], row: dict[str, Any]) -> list[str]:
    problems = []
    if row["exe"] != role["engine"]:
        problems.append(f"unexpected executable {row['exe']}")
    if row["gomaxprocs"] != role["gomaxprocs"]:
        problems.append(f"GOMAXPROCS {row['gomaxprocs']} != {role['gomaxprocs']}")
    if row["cpus_allowed_list"] not in role["allowed_cpu_masks"]:
        problems.append(f"CPU mask {row['cpus_allowed_list']} not in {role['allowed_cpu_masks']}")
    return problems


def load_config(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    required = {"schema", "command", "cwd", "environment", "roles", "sample_interval_seconds", "match_stdout", "match_stderr", "witness"}
    if set(value) != required or value["schema"] != "ngn-candidate-match-stage-v1":
        raise CandidateMatchError("invalid match-stage config")
    if not isinstance(value["command"], list) or not value["command"] or not all(isinstance(x, str) and x for x in value["command"]):
        raise CandidateMatchError("match-stage command must be nonempty argv")
    if not isinstance(value["environment"], dict) or not all(isinstance(k, str) and isinstance(v, str) for k, v in value["environment"].items()):
        raise CandidateMatchError("match-stage environment invalid")
    if not isinstance(value["sample_interval_seconds"], (int, float)) or not 0 < value["sample_interval_seconds"] <= 1:
        raise CandidateMatchError("match-stage sample interval invalid")
    if not isinstance(value["roles"], list) or len(value["roles"]) != 2:
        raise CandidateMatchError("match-stage requires two roles")
    ids = set()
    for role in value["roles"]:
        if set(role) != {"id", "engine", "engine_sha256", "cwd", "gomaxprocs", "allowed_cpu_masks", "launcher"}:
            raise CandidateMatchError("match-stage role keys invalid")
        ids.add(role["id"])
        if role["gomaxprocs"] != "1" or not isinstance(role["allowed_cpu_masks"], list) or not role["allowed_cpu_masks"]:
            raise CandidateMatchError("match-stage role width/masks invalid")
    if len(ids) != 2:
        raise CandidateMatchError("match-stage role IDs must be unique")
    return value


def stop_process(process: subprocess.Popen[bytes], grace: float = 2.0) -> None:
    if process.poll() is not None:
        return
    process.terminate()
    try:
        process.wait(timeout=grace)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=grace)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, required=True)
    args = parser.parse_args()
    process: subprocess.Popen[bytes] | None = None
    config = load_config(args.config.resolve(strict=True))
    witness_path = Path(config["witness"])
    witness_path.parent.mkdir(parents=True, exist_ok=True)
    receipt = {"schema": "ngn-candidate-child-process-witness-v1", "state": "STARTING", "started_utc": utc_now()}
    atomic_json(witness_path, receipt)
    try:
        cwd = Path(config["cwd"]).resolve(strict=True)
        roles = {}
        for item in config["roles"]:
            engine = Path(item["engine"]).resolve(strict=True)
            role_cwd = Path(item["cwd"]).resolve(strict=True)
            launcher = Path(item["launcher"]).resolve(strict=True)
            if sha256(engine) != item["engine_sha256"]:
                raise CandidateMatchError(f"role {item['id']}: engine identity mismatch")
            roles[item["id"]] = {
                **item,
                "engine": str(engine),
                "cwd": str(role_cwd),
                "launcher": str(launcher),
            }
        engine_roles = {role["engine"]: role for role in roles.values()}
        if len(engine_roles) != len(roles):
            raise CandidateMatchError("match-stage requires distinct frozen engine paths per role")
        observations: list[dict[str, Any]] = []
        instances: dict[int, dict[str, Any]] = {}
        violations: list[str] = []
        with Path(config["match_stdout"]).open("xb") as stdout, Path(config["match_stderr"]).open("xb") as stderr:
            process = subprocess.Popen(config["command"], cwd=cwd, env=config["environment"], stdout=stdout, stderr=stderr, start_new_session=False)
            while process.poll() is None:
                rows = snapshot_processes()
                for pid in sorted(descendant_pids(process.pid, rows)):
                    try:
                        row = read_process(pid, rows[pid])
                    except (FileNotFoundError, ProcessLookupError, PermissionError, CandidateMatchError):
                        continue
                    exact_engine_role = engine_roles.get(row["exe"])
                    launcher_roles = [role for role in roles.values() if role["launcher"] in row["cmdline"]]
                    cwd_roles = [role for role in roles.values() if row["cwd"] == role["cwd"]]
                    if exact_engine_role is not None:
                        role = exact_engine_role
                        if row["cwd"] != role["cwd"]:
                            violations.append(f"role {role['id']} pid {pid}: cwd {row['cwd']} != {role['cwd']}")
                            continue
                    elif launcher_roles:
                        if len(launcher_roles) != 1:
                            violations.append(f"pid {pid}: ambiguous role launcher paths {row['cmdline']}")
                            continue
                        role = launcher_roles[0]
                        if row["cwd"] != role["cwd"]:
                            violations.append(f"role {role['id']} launcher pid {pid}: cwd {row['cwd']} != {role['cwd']}")
                        continue
                    elif cwd_roles:
                        role = cwd_roles[0]
                        violations.append(f"role {role['id']} pid {pid}: unexpected executable {row['exe']}")
                        continue
                    else:
                        continue
                    observed = {
                        "monotonic": time.monotonic(), "role_id": role["id"], **row,
                        "engine_sha256": role["engine_sha256"],
                    }
                    observations.append(observed)
                    instances[pid] = observed
                    for problem in observation_violations(role, row):
                        violations.append(f"role {role['id']} pid {pid}: {problem}")
                if violations:
                    stop_process(process)
                    break
                time.sleep(float(config["sample_interval_seconds"]))
            returncode = process.wait()
        seen_by_role = {role_id: sorted(pid for pid, row in instances.items() if row["role_id"] == role_id) for role_id in roles}
        missing = [role_id for role_id, pids in seen_by_role.items() if not pids]
        if returncode != 0:
            violations.append(f"fastchess returncode {returncode}")
        if missing:
            violations.append(f"roles never observed as frozen engines: {missing}")
        receipt.update({
            "state": "COMPLETE" if not violations else "FAILED", "ended_utc": utc_now(),
            "fastchess_pid": process.pid, "fastchess_returncode": returncode,
            "observation_count": len(observations), "observed_instances": seen_by_role,
            "observations": observations, "violations": sorted(set(violations)),
        })
        atomic_json(witness_path, receipt)
        if violations:
            raise CandidateMatchError("; ".join(sorted(set(violations))))
        print(json.dumps({"state": "COMPLETE", "observed_instances": seen_by_role, "observations": len(observations)}, sort_keys=True))
        return 0
    except Exception as error:
        if process is not None:
            stop_process(process)
        receipt.update({"state": "FAILED", "ended_utc": utc_now(), "error": str(error)})
        atomic_json(witness_path, receipt)
        print(f"match stage failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
