#!/usr/bin/env python3
"""Profile-aware trace extension for fixed NGN Counter/external cells."""
from __future__ import annotations

import argparse
import json
import re
from collections import Counter
from pathlib import Path

from common import CandidateMatchError, atomic_json, sha256, utc_now
from external_match_manifest import warning_policy_for
from external_profiles import profile_for
from trace_audit import (
    FATAL_PHRASES,
    NONZERO_STATUS_RE,
    THREADS_RE,
    TRACE_RE,
    clock_go_violation,
    crash_lines,
)

UCI_MOVE_RE = re.compile(r"^[a-h][1-8][a-h][1-8][nbrq]?$")
COUNTER55_PV_WARNING_RE = re.compile(
    r"^(?P<actor>fastchess) --- Warning; PV continues after fifty-move rule - "
    r"move (?P<move>[a-h][1-8][a-h][1-8][nbrq]?) from Counter 5\.5$"
)
COUNTER55_INFO_DIAGNOSTIC_RE = re.compile(
    r"^Info; info depth [0-9]+ score (?:cp|mate) -?[0-9]+ nodes [0-9]+ "
    r"time [0-9]+ nps [0-9]+ pv (?P<pv>.+)$"
)


def integer(value, where: str, low: int = 0) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value < low:
        raise CandidateMatchError(f"{where}: expected integer >= {low}")
    return value


def load_config(path: Path) -> dict:
    value = json.loads(path.read_text(encoding="utf-8"))
    exact = {"trace", "match_exit", "match_stderr", "output", "candidate", "opponent", "warning_policy"}
    if not isinstance(value, dict) or set(value) != exact:
        raise CandidateMatchError("external trace config keys differ")
    candidate = value["candidate"]
    opponent = value["opponent"]
    if not isinstance(candidate, dict) or set(candidate) != {"display_name", "width", "options", "expected_processes", "expected_refreshes", "crash_log"}:
        raise CandidateMatchError("external trace candidate config differs")
    if not isinstance(opponent, dict) or set(opponent) != {"display_name", "profile_id", "width", "options", "expected_processes", "expected_refreshes"}:
        raise CandidateMatchError("external trace opponent config differs")
    for label, role in (("candidate", candidate), ("opponent", opponent)):
        if not isinstance(role["display_name"], str) or not role["display_name"].strip():
            raise CandidateMatchError(f"external trace {label} display invalid")
        width = integer(role["width"], f"external trace {label} width", 1)
        if width not in (1, 8) or not isinstance(role["options"], list):
            raise CandidateMatchError(f"external trace {label} width/options invalid")
        integer(role["expected_processes"], f"external trace {label} expected_processes", 1)
        integer(role["expected_refreshes"], f"external trace {label} expected_refreshes", 1)
    if candidate["width"] != opponent["width"] or candidate["display_name"] == opponent["display_name"]:
        raise CandidateMatchError("external trace role width/name collision")
    profile = profile_for(opponent["profile_id"])
    if opponent["display_name"] != profile.display_name:
        raise CandidateMatchError("external trace opponent display/profile differs")
    if value["warning_policy"] != warning_policy_for(opponent["profile_id"]):
        raise CandidateMatchError("external trace warning policy differs")
    crash = candidate["crash_log"]
    if not isinstance(crash, dict) or set(crash) != {"path", "logged_path", "expected_processes"}:
        raise CandidateMatchError("external trace crash config differs")
    integer(crash["expected_processes"], "external trace crash expected_processes", 1)
    for name in ("trace", "match_exit", "match_stderr", "output"):
        if not isinstance(value[name], str) or not value[name] or "\0" in value[name]:
            raise CandidateMatchError(f"external trace {name} path invalid")
    return value


def prepare_trace_lines(lines: list[str], config: dict) -> tuple[list[tuple[int, str]], list[dict]]:
    """Consume only the exact Counter 5.5 Fastchess warning plus its diagnostic block."""
    policy = config["warning_policy"]
    opponent = config["opponent"]
    profile = profile_for(opponent["profile_id"])
    if opponent["display_name"] != profile.display_name:
        raise CandidateMatchError("external trace opponent display/profile differs")
    if policy != warning_policy_for(opponent["profile_id"]):
        raise CandidateMatchError("external trace warning policy differs")
    prepared: list[tuple[int, str]] = []
    allowed: list[dict] = []
    index = 0
    while index < len(lines):
        line = lines[index]
        parsed = TRACE_RE.fullmatch(line)
        warning = None
        level = ""
        if parsed is not None:
            level = parsed.group("level").strip().upper()
            warning = COUNTER55_PV_WARNING_RE.fullmatch(parsed.group("body").strip())
        permits_counter_warning = (
            policy["id"] == "counter55-pv-after-fifty-v1"
            and policy["fastchess_strict"] is False
            and len(policy["allowed_fastchess_warnings"]) == 1
        )
        if warning is None or level != "WARN" or not permits_counter_warning:
            prepared.append((index + 1, line))
            index += 1
            continue
        if index + 4 >= len(lines):
            raise CandidateMatchError(f"line {index + 1}: truncated Counter warning diagnostic block")
        info, position, moves, blank = lines[index + 1:index + 5]
        info_match = COUNTER55_INFO_DIAGNOSTIC_RE.fullmatch(info)
        if info_match is None:
            raise CandidateMatchError(f"line {index + 2}: malformed Counter warning Info diagnostic")
        pv = info_match.group("pv").split()
        if not pv or any(UCI_MOVE_RE.fullmatch(move) is None for move in pv) or warning.group("move") not in pv:
            raise CandidateMatchError(f"line {index + 2}: malformed Counter warning PV diagnostic")
        if position != "Position; startpos":
            raise CandidateMatchError(f"line {index + 3}: malformed Counter warning Position diagnostic")
        if not moves.startswith("Moves; "):
            raise CandidateMatchError(f"line {index + 4}: malformed Counter warning Moves diagnostic")
        history = moves[len("Moves; "):].split()
        if not history or any(UCI_MOVE_RE.fullmatch(move) is None for move in history):
            raise CandidateMatchError(f"line {index + 4}: malformed Counter warning move history")
        if blank != "":
            raise CandidateMatchError(f"line {index + 5}: Counter warning diagnostic block lacks blank terminator")
        allowed.append({
            "kind": "pv-continues-after-fifty-move-rule",
            "line": index + 1,
            "level": level,
            "actor": warning.group("actor"),
            "role": "opponent",
            "profile_id": config["opponent"]["profile_id"],
            "display_name": config["opponent"]["display_name"],
            "move": warning.group("move"),
            "raw_lines": lines[index:index + 5],
        })
        index += 5
    return prepared, allowed


def audit(lines: list[str], config: dict) -> dict:
    candidate = config["candidate"]
    opponent = config["opponent"]
    by_name = {candidate["display_name"]: ("candidate", candidate),
               opponent["display_name"]: ("opponent", opponent)}
    refreshes: dict[tuple[str, str], list[str]] = {}
    widths: Counter[tuple[str, str]] = Counter()
    pending: dict[str, int] = {}
    normal_exits: Counter[str] = Counter()
    completed_refreshes: Counter[str] = Counter()
    external_stderr: list[str] = []
    failures: list[str] = []
    probable_book = []
    prepared_lines, allowed_warnings = prepare_trace_lines(lines, config)
    for line_number, line in prepared_lines:
        match = TRACE_RE.fullmatch(line)
        if match is None:
            failures.append(f"line {line_number}: unrecognized trace")
            continue
        level = match.group("level").strip().upper()
        thread = match.group("thread").strip()
        body = match.group("body").strip()
        lower = body.lower()
        if level in {"WARN", "ERROR", "FATAL", "CRITICAL"} or "warning;" in lower:
            failures.append(f"line {line_number}: forbidden level/message: {body}")
        if any(phrase in lower for phrase in FATAL_PHRASES):
            failures.append(f"line {line_number}: operational failure phrase: {body}")
        status = NONZERO_STATUS_RE.search(body)
        if status and int(status.group(1)) != 0:
            failures.append(f"line {line_number}: nonzero status: {body}")

        prefix = "fastchess --- Refreshing engine "
        if body.startswith(prefix):
            name = body[len(prefix):]
            key = (thread, name)
            if name not in by_name:
                failures.append(f"line {line_number}: unknown refresh {name}")
            elif key in refreshes:
                failures.append(f"line {line_number}: duplicate active refresh {name}")
            else:
                refreshes[key] = []
                widths[key] = 0
            continue
        prefix = "fastchess --- Sending setoption to engine "
        if body.startswith(prefix):
            rest = body[len(prefix):]
            names = [name for name in by_name if rest.startswith(name + " ")]
            if len(names) != 1 or (thread, names[0]) not in refreshes:
                failures.append(f"line {line_number}: unassociated setoption: {body}")
            else:
                refreshes[(thread, names[0])].append(rest[len(names[0]) + 1:])
            continue
        if body.startswith("fastchess --- Engine ") and body.endswith(" refreshed."):
            name = body[len("fastchess --- Engine "):-len(" refreshed.")]
            actual = refreshes.pop((thread, name), None)
            if name not in by_name or actual is None:
                failures.append(f"line {line_number}: unmatched refresh completion {name}")
            else:
                expected = [f"{item['name']} {item['value']}" for item in by_name[name][1]["options"]]
                if actual != expected:
                    failures.append(f"line {line_number}: option order {name} actual={actual} expected={expected}")
                if by_name[name][0] == "candidate" and widths[(thread, name)] != 1:
                    failures.append(f"line {line_number}: candidate configured-width receipts={widths[(thread,name)]}")
                if by_name[name][0] == "opponent" and widths[(thread, name)] != 0:
                    failures.append(f"line {line_number}: external opponent emitted NGN width receipt")
                completed_refreshes[name] += 1
                widths.pop((thread, name), None)
            continue

        for name in by_name:
            sent = name + " <--- "
            if body.startswith(sent):
                command = body[len(sent):].strip()
                if command == "go" or command.startswith("go "):
                    violation = clock_go_violation(command)
                    if violation is not None:
                        failures.append(f"line {line_number}: non-clock go: {violation}")
                    elif name in pending:
                        failures.append(f"line {line_number}: duplicate pending go {name}")
                    else:
                        pending[name] = 0
                break

        if " ---> " not in body:
            continue
        actor, payload = body.split(" ---> ", 1)
        actor = actor.strip()
        payload = payload.strip()
        is_stderr = actor.startswith("<stderr>")
        name = actor[len("<stderr>"):].strip() if is_stderr else actor
        if name not in by_name:
            continue
        kind, _ = by_name[name]
        if payload.startswith("info string error"):
            failures.append(f"line {line_number}: canonical engine error: {payload}")
        if kind == "candidate" and not is_stderr and payload.startswith("info depth 1 score cp 50 nodes 1 time 0 nps 0 pv "):
            probable_book.append({"line": line_number, "role": name, "payload": payload})
            failures.append(f"line {line_number}: probable embedded-book signature")
        width_match = THREADS_RE.fullmatch(payload) if not is_stderr else None
        if width_match is not None:
            configured, effective = map(int, width_match.groups())
            if kind != "candidate":
                failures.append(f"line {line_number}: external width diagnostic")
            elif (thread, name) in refreshes:
                if configured != candidate["width"] or effective != candidate["width"]:
                    failures.append(f"line {line_number}: configured width {configured}/{effective}")
                widths[(thread, name)] += 1
            elif name in pending:
                if configured != candidate["width"] or effective != candidate["width"]:
                    failures.append(f"line {line_number}: go width {configured}/{effective}")
                pending[name] += 1
            else:
                failures.append(f"line {line_number}: stale width diagnostic")
        if not is_stderr and payload.startswith("bestmove "):
            count = pending.pop(name, None)
            if count is None:
                failures.append(f"line {line_number}: bestmove without go")
            elif kind == "candidate" and count != (1 if candidate["width"] > 1 else 0):
                failures.append(f"line {line_number}: candidate per-go width receipts={count}")
            elif kind == "opponent" and count != 0:
                failures.append(f"line {line_number}: opponent per-go width receipts={count}")
        if is_stderr:
            if payload == "Process exited normally with status 0":
                normal_exits[name] += 1
            elif kind == "opponent":
                external_stderr.append(payload)
            else:
                failures.append(f"line {line_number}: candidate stderr {payload}")

    for key in refreshes:
        failures.append(f"unterminated refresh {key}")
    for name in pending:
        failures.append(f"pending go {name}")
    for name, (_, role) in by_name.items():
        if completed_refreshes[name] != role["expected_refreshes"]:
            failures.append(f"{name}: refreshes={completed_refreshes[name]} expected={role['expected_refreshes']}")
        if normal_exits[name] != role["expected_processes"]:
            failures.append(f"{name}: normal exits={normal_exits[name]} expected={role['expected_processes']}")
    profile = profile_for(opponent["profile_id"])
    # Profile validator checks each exact Counter startup pair or requires empty Rodent stderr.
    from external_profiles import COUNTER55, validate_stderr
    if profile is COUNTER55:
        expected = opponent["expected_processes"]
        if len(external_stderr) != 2 * expected:
            raise CandidateMatchError(
                f"Counter stderr lines={len(external_stderr)} expected={2 * expected}"
            )
        for index in range(expected):
            validate_stderr(profile, external_stderr[2 * index:2 * index + 2], opponent["width"])
    else:
        validate_stderr(profile, external_stderr, opponent["width"])
    if failures:
        raise CandidateMatchError("external trace failures: " + " | ".join(failures[:20]))
    return {"trace_lines": len(lines), "normal_exits": dict(normal_exits),
            "warning_policy": config["warning_policy"],
            "allowed_fastchess_warnings": allowed_warnings,
            "allowed_fastchess_warning_counts": dict(Counter(
                warning["kind"] for warning in allowed_warnings
            )),
            "forbidden_warning_count": 0,
            "probable_embedded_book_signature_plies": len(probable_book), "pass": True}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, required=True)
    args = parser.parse_args()
    config = load_config(args.config.resolve(strict=True))
    output = Path(config["output"])
    receipt = {"schema": "ngn-external-trace-audit-v1", "state": "STARTING", "started_utc": utc_now()}
    try:
        if Path(config["match_exit"]).read_text().strip() != "0" or Path(config["match_stderr"]).stat().st_size:
            raise CandidateMatchError("match process status/stderr differs")
        report = audit(Path(config["trace"]).read_text(encoding="utf-8", errors="replace").splitlines(), config)
        crash = config["candidate"]["crash_log"]
        report["candidate_crash_log"] = crash_lines(Path(crash["path"]), Path(crash["logged_path"]), crash["expected_processes"])
        receipt.update({"state": "COMPLETE", "ended_utc": utc_now(), "trace_sha256": sha256(Path(config["trace"])), "trace": report, "pass": True})
        atomic_json(output, receipt)
        return 0
    except Exception as error:
        receipt.update({"state": "FAILED", "ended_utc": utc_now(), "error": str(error), "pass": False})
        atomic_json(output, receipt)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
