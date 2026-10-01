#!/usr/bin/env python3
"""One-toggle fixed-node diagnostic for frozen safe-study cases.

This writes only output/recovery-2026-09-04/search-ablation-* artifacts.  It
does not alter playing source, parameters, or run games.
"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import select
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "output/recovery-2026-09-04"
STUDY = OUT / "safe-study-result.json"
SF = Path(os.environ.get("STOCKFISH", "/opt/homebrew/bin/stockfish"))
BASE = ROOT / "build/ngn_20260904_qcap"
FITTED = ROOT / "build/ngn_20260905_texelv2"
DEADLINE = None
FAMILIES = ("NullMove", "Futility", "RFP", "Probcut", "LMP", "SEEPrune", "LMR", "HistPrune")
DEFAULTS = ("true",) * 10


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def write(name, value):
    path = OUT / name
    path.write_text(json.dumps(value, indent=2) + "\n")
    return path


def check_deadline():
    if time.monotonic() > DEADLINE:
        raise TimeoutError("15-minute diagnostic cap reached")


class Engine:
    def __init__(self, binary, label):
        self.label, self.binary = label, Path(binary)
        self.p = subprocess.Popen([str(binary)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                  stderr=subprocess.STDOUT, bufsize=0)
        self.buf = b""
        self.send("uci"); self.until("uciok")
        self.send("setoption name Hash value 64")
        self.send("setoption name Threads value 1")
        self.send("isready"); self.until("readyok")

    def send(self, command):
        self.p.stdin.write((command + "\n").encode())

    def until(self, marker):
        lines = []
        while time.monotonic() < DEADLINE:
            if b"\n" in self.buf:
                line, self.buf = self.buf.split(b"\n", 1)
                line = line.decode(errors="replace").strip()
                lines.append(line)
                if line.startswith(marker):
                    return lines
            elif select.select([self.p.stdout], [], [], .2)[0]:
                chunk = os.read(self.p.stdout.fileno(), 65536)
                if not chunk:
                    raise RuntimeError(f"{self.label} exited")
                self.buf += chunk
        raise TimeoutError(f"{self.label}: {marker}")

    def analyse(self, moves, command):
        check_deadline()
        self.send("ucinewgame"); self.send("isready"); self.until("readyok")
        self.send("position startpos moves " + " ".join(moves))
        self.send(command)
        lines = self.until("bestmove")
        result = {"command": command, "bestmove": lines[-1].split()[1], "raw": lines,
                  "usable": False}
        for line in lines:
            fields = line.split()
            if "score" not in fields or "depth" not in fields or "pv" not in fields:
                continue
            score = fields.index("score")
            kind, value = fields[score + 1], int(fields[score + 2])
            result.update(depth=int(fields[fields.index("depth") + 1]), kind=kind, value=value,
                          cp=max(-1500, min(1500, value)) if kind == "cp" else (1500 if value > 0 else -1500),
                          bounded=("lowerbound" in fields or "upperbound" in fields),
                          nodes=int(fields[fields.index("nodes") + 1]) if "nodes" in fields else None,
                          pv=fields[fields.index("pv") + 1:])
            result["usable"] = not result["bounded"] and ("depth" not in command or result["depth"] >= 16)
        return result

    def close(self):
        if self.p.poll() is None:
            self.send("quit")
            try:
                self.p.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.p.kill(); self.p.wait()


def select_cases(study):
    selected = []
    for row in study["games"]:
        if row.get("status") != "selected":
            continue
        sf = next((x for x in row["choices"] if "sf" in x["labels"]), None)
        ngn = next((x for x in row["choices"] if "current_ngn" in x["labels"]), None)
        if not sf or not ngn or ngn.get("relative_loss_cp") is None or ngn["relative_loss_cp"] < 50:
            continue
        selected.append({"line_number": row["game"]["line_number"], "game": row["game"],
                         "prefix": row["prefix"], "fen": row["fen"],
                         "stored_base_move": row["ngn"]["bestmove"], "stored_sf_move": row["sf"]["bestmove"],
                         "stored_relative_loss_cp": ngn["relative_loss_cp"]})
    return selected


def make_overlay(family):
    # Refuse a later playing-source revision: these overlays were defined on
    # e549f3c. Documentation-only commits are safe to reproduce from.
    changed = subprocess.check_output(["git", "diff", "e549f3c", "--", ":(glob)**/*.go", "go.mod", "go.sum"], cwd=ROOT)
    if changed:
        raise RuntimeError("playing/offline Go source differs from the frozen experiment parent")
    source = (ROOT / "engine/search.go").read_text()
    values = list(DEFAULTS)
    values[FAMILIES.index(family)] = "false"
    old = "}{" + ", ".join(DEFAULTS) + "}"
    new = "}{" + ", ".join(values) + "}"
    if source.count(old) != 1:
        raise RuntimeError("cannot identify SearchToggles default literal")
    overlay_source = OUT / f"search-ablation-{family}-search.go.txt"
    overlay_source.write_text(source.replace(old, new))
    overlay = OUT / f"search-ablation-{family}-overlay.json"
    overlay.write_text(json.dumps({"Replace": {str(ROOT / "engine/search.go"): str(overlay_source)}}, indent=2) + "\n")
    binary = OUT / f"search-ablation-{family}.bin"
    env = os.environ.copy(); env.update(GOCACHE="/private/tmp/ngn-go-cache", GOMODCACHE="/private/tmp/ngn-go-modcache")
    p = subprocess.run(["go", "build", "-overlay", str(overlay), "-o", str(binary), "."], cwd=ROOT, env=env,
                       stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, timeout=120)
    (OUT / f"search-ablation-{family}-build.log").write_text(p.stdout)
    p.check_returncode()
    return {"family": family, "binary": str(binary), "binary_sha256": sha(binary),
            "overlay": str(overlay), "overlay_sha256": sha(overlay),
            "source_overlay": str(overlay_source), "source_overlay_sha256": sha(overlay_source),
            "toggles": dict(zip(FAMILIES + ("IID", "Singular"), values))}


def score_choices(sf, case):
    choices = {}
    for label, run in case["runs"].items():
        move = run["bestmove"]
        choices.setdefault(move, []).append(label)
    scored = []
    for move, labels in sorted(choices.items()):
        reply = sf.analyse(case["prefix"] + [move], "go depth 16")
        item = {"move": move, "labels": labels, "reply": reply,
                "ngn_pov_cp": -reply["cp"] if reply["usable"] else None}
        scored.append(item)
    base = next(x for x in scored if "base_400k" in x["labels"])
    for item in scored:
        item["delta_vs_base_scored_choice_cp"] = None if base["ngn_pov_cp"] is None or item["ngn_pov_cp"] is None else item["ngn_pov_cp"] - base["ngn_pov_cp"]
        item["diagnostic_rescue_50cp"] = item["delta_vs_base_scored_choice_cp"] is not None and item["delta_vs_base_scored_choice_cp"] >= 50
    case["scored_choices"] = scored
    four = next(x for x in scored if "base_4m" in x["labels"])
    case["base_4m_alone"] = {"changes_move": case["runs"]["base_4m"]["bestmove"] != case["runs"]["base_400k"]["bestmove"],
                              "diagnostic_rescue_50cp": four["diagnostic_rescue_50cp"]}


def main():
    global DEADLINE
    start = time.monotonic(); DEADLINE = start + 900
    if sha(BASE) != "24d71a1ac381320d2ab790b1a88a7fdb30761bcedf573f9b98973b5a1e7eb702": raise RuntimeError("unexpected qcap baseline")
    if sha(FITTED) != "a98c654f425539b520d9aa0449b31875e60776a2ca74f9b3208322198516fc0b": raise RuntimeError("unexpected fitted control")
    if sha(STUDY) != "95926b6cc5dd4aa66615b071ead937f413d9c9f7ed4b3957835d1a6afcf82737":
        raise RuntimeError("safe-study input changed")
    study = json.loads(STUDY.read_text())
    cases = select_cases(study)
    selection = {"input": str(STUDY), "input_sha256": sha(STUDY), "threshold_relative_loss_cp": 50,
                 "cases": cases, "case_count": len(cases)}
    write("search-ablation-selection-2026-09-05.json", selection)
    if not cases: raise RuntimeError("no frozen cases")

    report = {"protocol": "experiments/2026-09-05-search-ablation.md",
              "script_sha256": sha(__file__), "source_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
              "baseline": {"path": str(BASE), "sha256": sha(BASE), "command": "go nodes 400000", "hash_mb": 64, "threads": 1},
              "fitted_evaluation_control": {"path": str(FITTED), "sha256": sha(FITTED), "command": "go nodes 400000", "hash_mb": 64, "threads": 1},
              "stockfish": {"path": str(SF), "sha256": sha(SF), "command": "go depth 16"},
              "selection": selection, "overlays": [], "cases": [], "status": "running"}
    base = fitted = sf = None
    overlays = []
    try:
        base = Engine(BASE, "base")
        # Identity gate BEFORE building/running variants.
        for frozen in cases:
            got = base.analyse(frozen["prefix"], "go nodes 400000")
            if got["bestmove"] != frozen["stored_base_move"]:
                raise RuntimeError(f"baseline identity failure line {frozen['line_number']}: {got['bestmove']} != {frozen['stored_base_move']}")
            frozen["base_400k"] = got
        write("search-ablation-baseline-identity-2026-09-05.json", {"status": "PASS", "cases": cases})
        for family in FAMILIES:
            overlays.append(make_overlay(family))
        report["overlays"] = overlays
        engines = {"base_400k": base}
        for item in overlays:
            engines["overlay_" + item["family"]] = Engine(item["binary"], item["family"])
        fitted = Engine(FITTED, "fitted_control")
        engines["fitted_control_400k"] = fitted
        sf = Engine(SF, "stockfish")
        for frozen in cases:
            case = {k: v for k, v in frozen.items() if k != "base_400k"}
            case["runs"] = {"base_400k": frozen["base_400k"]}
            case["runs"]["base_4m"] = base.analyse(case["prefix"], "go nodes 4000000")
            for family in FAMILIES:
                case["runs"]["overlay_" + family] = engines["overlay_" + family].analyse(case["prefix"], "go nodes 400000")
            case["runs"]["fitted_control_400k"] = fitted.analyse(case["prefix"], "go nodes 400000")
            score_choices(sf, case)
            report["cases"].append(case)
            write("search-ablation-partial-2026-09-05.json", report)
            print(f"completed line {case['line_number']}", flush=True)
        report["status"] = "complete"; report["elapsed_seconds"] = time.monotonic() - start
        write("search-ablation-result-2026-09-05.json", report)
        print(json.dumps({"status": report["status"], "cases": len(report["cases"]), "elapsed_seconds": report["elapsed_seconds"]}, indent=2))
    finally:
        for e in ([base, fitted, sf] + [x for x in locals().get("engines", {}).values()]):
            if e is not None: e.close()


if __name__ == "__main__":
    main()
