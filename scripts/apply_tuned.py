#!/usr/bin/env python3
"""Splice a `texel -mode gradient/tune` output blob into the engine source.

Usage: scripts/apply_tuned.py <model.json|legacy_blob.go> [--dry-run]

Versioned JSON validates every name, value and reserved slot before replacing
material, PST, scalar, mobility and threat declarations. Legacy Go input below
remains supported for coordinate descent. The caller must rebuild the engine.

The blob contains `var pestoMGMaterial = [7]int{...}`, `var pestoEGMaterial = ...`,
the 12 `var <name>Table = [64]int{...}` PST blocks, and (after a
`// eval.go tunable weights:` marker) tab-indented `name = value` weight lines.
Each `var` block is matched by name and replaces the corresponding declaration in
engine/eval_pesto.go (order-independent); each weight replaces its definition in
engine/eval.go (the `\tname = N` line, never a usage site). Git is the undo.
"""
import re
import sys
import os
import json
import tempfile
from pathlib import Path

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PESTO = os.path.join(ROOT, "engine", "eval_pesto.go")
EVAL = os.path.join(ROOT, "engine", "eval.go")

SCALARS = ["passedPawnBonus", "doubledPawnPenalty", "isolatedPawnPenalty",
           "backwardPawnPenalty", "pawnChainWeight", "mobilityWeight",
           "rookOpenWeight", "outpostWeight", "kingSafetyWeight", "kingActivityWeight"]
THREATS = dict(zip(
    ["threat_pawn", "threat_minor_major", "threat_rook_queen", "threat_hanging_minor", "threat_hanging_rook", "threat_hanging_queen"],
    ["threatByPawn", "threatMinorOnMajor", "threatRookOnQueen", "threatHangingMinor", "threatHangingRook", "threatHangingQueen"]))
MOBILITY = {"knight": 9, "bishop": 14, "rook": 15, "queen": 28}
PIECES = ["pawn", "knight", "bishop", "rook", "queen", "king"]


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON field: " + key)
        result[key] = value
    return result


def expected_names():
    return ([f"{ph}_material_{p}" for ph in ("mg", "eg") for p in PIECES[:-1]] +
            [f"{ph}_pst_{p}_{i}" for ph in ("mg", "eg") for p in PIECES for i in range(64)] +
            SCALARS + [n+"EG" for n in SCALARS] +
            [f"{ph}_mobility_{p}_{i}" for ph in ("mg", "eg") for p, n in MOBILITY.items() for i in range(n)] +
            list(THREATS))


def array_match(source, name, length):
    pattern = re.compile(r"^(?P<indent>[ \t]*)(?P<var>var[ \t]+)?" + re.escape(name) +
                         r"[ \t]*=[ \t]*\[(?P<size>\d+)\]int\{(?P<body>[^}]*)\}", re.M)
    matches = list(pattern.finditer(source))
    if len(matches) != 1 or int(matches[0]['size']) != length:
        raise ValueError(f"expected one [{length}]int declaration for {name}")
    return pattern, matches[0]


def replace_array(source, name, values):
    pattern, match = array_match(source, name, len(values))
    prefix = match['indent'] + (match['var'] or '') + name
    body = ', '.join(str(v) for v in values)
    replacement = f"{prefix} = [{len(values)}]int{{{body}}}"
    return pattern.sub(lambda _: replacement, source, count=1)


def replace_scalar(source, name, value):
    pattern = re.compile(r"^([ \t]+"+re.escape(name)+r"[ \t]*=[ \t]*)-?\d+", re.M)
    if len(list(pattern.finditer(source))) != 1:
        raise ValueError("expected one scalar declaration for " + name)
    return pattern.sub(lambda m: m[1]+str(value), source, count=1)


def apply_json(blob, pesto, evalsrc):
    model = json.loads(blob, object_pairs_hook=unique_object)
    fields = {"Version", "Names", "Values", "StoredCount", "ActiveCount", "InactiveReservations"}
    if not isinstance(model, dict) or set(model) != fields:
        raise ValueError("unexpected v2 model fields")
    if (model["Version"] != "ngn-texel-v2-936" or type(model["StoredCount"]) is not int or
            type(model["ActiveCount"]) is not int or model["StoredCount"] != 936 or
            model["ActiveCount"] != 934 or model["InactiveReservations"] != ["mobilityWeight", "mobilityWeightEG"]):
        raise ValueError("incompatible model version or parameter counts")
    names = expected_names()
    values = model["Values"]
    if model["Names"] != names or not isinstance(values, list) or len(values) != len(names):
        raise ValueError("model names/order/values do not match all 936 parameters")
    if any(type(v) is not int or not -(2**31) <= v < 2**31 for v in values):
        raise ValueError("model values must be signed 32-bit integers")
    named = dict(zip(names, values))
    for ph in ("mg", "eg"):
        material = "pesto"+ph.upper()+"Material"
        _, match = array_match(pesto, material, 7)
        body = re.sub(r"//[^\n]*|/\*.*?\*/", "", match['body'], flags=re.S)
        old = [int(v.strip()) for v in body.split(',') if v.strip()]
        if len(old) != 7 or old[0] != 0 or old[6] != 0:
            raise ValueError("unexpected reserved material values for " + material)
        pesto = replace_array(pesto, material, [0]+[named[f"{ph}_material_{p}"] for p in PIECES[:-1]]+[0])
        for p in PIECES:
            pesto = replace_array(pesto, ph+p.title()+"Table", [named[f"{ph}_pst_{p}_{i}"] for i in range(64)])
        for p, n in MOBILITY.items():
            evalsrc = replace_array(evalsrc, p+"Mob"+ph.upper(), [named[f"{ph}_mobility_{p}_{i}"] for i in range(n)])
    for name in SCALARS+[n+"EG" for n in SCALARS]:
        evalsrc = replace_scalar(evalsrc, name, named[name])
    for name, variable in THREATS.items():
        evalsrc = replace_scalar(evalsrc, variable, named[name])
    return pesto, evalsrc


def write_sources(pesto, evalsrc):
    # Parse/validate and stage both files before changing either source file.
    staged = []
    try:
        for destination, content in ((PESTO, pesto), (EVAL, evalsrc)):
            with tempfile.NamedTemporaryFile(mode='w', dir=os.path.dirname(destination), delete=False) as f:
                staged.append((f.name, destination))
                f.write(content)
            os.chmod(f.name, os.stat(destination).st_mode & 0o777)
        for temporary, destination in staged:
            os.replace(temporary, destination)
    finally:
        for temporary, _ in staged:
            if os.path.exists(temporary):
                os.unlink(temporary)


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    dry = "--dry-run" in sys.argv
    if len(args) != 1:
        sys.exit("usage: apply_tuned.py <model.json|legacy_blob.go> [--dry-run]")
    blob = open(args[0]).read()

    if blob.lstrip().startswith('{'):
        try:
            pesto, evalsrc = apply_json(blob, Path(PESTO).read_text(), Path(EVAL).read_text())
            if not dry:
                write_sources(pesto, evalsrc)
        except (ValueError, OSError) as exc:
            sys.exit("ERROR: " + str(exc))
        print("validated all 936 v2 entries (934 active)" + ("; dry run" if dry else "; source updated"))
        return

    # Parse `var NAME = [N]int{ ... }` declarations. The body is `[^}]*` (flat int
    # arrays have no nested braces, and [^}] spans newlines), so this matches single-
    # line material arrays and multi-line tables uniformly without over-running.
    var_decls = re.findall(r"var (\w+) = (\[\d+\]int\{[^}]*\})", blob)
    # Parse weight lines after the marker.
    wmarker = blob.find("// eval.go tunable weights:")
    weights = []
    if wmarker >= 0:
        weights = re.findall(r"^\s*(\w+)\s*=\s*(-?\d+)\s*$", blob[wmarker:], re.MULTILINE)

    if not var_decls and not weights:
        sys.exit("no var declarations or weights found in blob")

    pesto = open(PESTO).read()
    changed = 0
    for name, decl in var_decls:
        pat = re.compile(r"var %s = \[\d+\]int\{[^}]*\}" % re.escape(name))
        if not pat.search(pesto):
            sys.exit("ERROR: %s not found in %s" % (name, PESTO))
        new = "var %s = %s" % (name, decl)
        pesto, n = pat.subn(new, pesto, count=1)
        if n:
            changed += 1
            print("  eval_pesto.go: %s" % name)

    evalsrc = open(EVAL).read()
    for name, val in weights:
        pat = re.compile(r"^(\t+%s\s*=\s*)-?\d+" % re.escape(name), re.MULTILINE)
        if not pat.search(evalsrc):
            sys.exit("ERROR: weight %s definition not found in %s" % (name, EVAL))
        evalsrc, n = pat.subn(r"\g<1>%s" % val, evalsrc, count=1)
        if n:
            changed += 1
            print("  eval.go: %s = %s" % (name, val))

    print("%d declarations spliced%s" % (changed, " (dry-run, not written)" if dry else ""))
    if not dry:
        open(PESTO, "w").write(pesto)
        open(EVAL, "w").write(evalsrc)


if __name__ == "__main__":
    main()
