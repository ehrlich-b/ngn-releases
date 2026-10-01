#!/usr/bin/env python3
"""Prove model export -> source import -> rebuilt executable fidelity.

Runs in a temporary source copy, never changes this checkout's coefficients.
Requires Go and the environment/cache settings normally used to build NGN.
"""
import copy
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def run(args, cwd, ok=True):
    result = subprocess.run(args, cwd=cwd, text=True, capture_output=True)
    if ok and result.returncode:
        raise RuntimeError(f"{args}: {result.stdout}\n{result.stderr}")
    return result


def main():
    with tempfile.TemporaryDirectory(prefix="ngn-model-roundtrip-") as directory:
        tree = Path(directory)
        for source in ("go.mod", "go.sum", "main.go"):
            if (ROOT / source).exists():
                shutil.copy2(ROOT / source, tree / source)
        for folder in ("engine", "cmd/texel", "internal"):
            if not (ROOT / folder).exists():
                continue
            for source in (ROOT / folder).rglob("*.go"):
                if source.name.endswith("_test.go"):
                    continue
                target = tree / source.relative_to(ROOT)
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(source, target)
        (tree / "scripts").mkdir()
        shutil.copy2(ROOT / "scripts/apply_tuned.py", tree / "scripts/apply_tuned.py")
        baseline = json.loads(run(["go", "run", "./cmd/texel", "-mode", "dump"], tree).stdout)
        model = copy.deepcopy(baseline)
        model["Values"] = [v + (i % 7 + 1) * (-1 if i % 2 else 1)
                           for i, v in enumerate(model["Values"])]
        assert all(a != b for a, b in zip(baseline["Values"], model["Values"]))
        modelpath = tree / "model.json"
        modelpath.write_text(json.dumps(model))
        command = [sys.executable, "scripts/apply_tuned.py", str(modelpath)]
        run(command, tree)
        run(["go", "build", "-o", str(tree / "ngn"), "."], tree)
        run(["go", "build", "-o", str(tree / "texel"), "./cmd/texel"], tree)
        rebuilt = json.loads(run([str(tree / "texel"), "-mode", "dump"], tree).stdout)
        assert rebuilt == model, "export/source/compiled model disagrees"
        print("PASS: every one of 936 changed entries survives source import and rebuild")

        sources = [tree / "engine/eval.go", tree / "engine/eval_pesto.go"]
        snapshot = [path.read_bytes() for path in sources]
        cases = []
        for name, mutate in (
            ("version", lambda x: x.update(Version="ngn-texel-v1")),
            ("name", lambda x: x["Names"].__setitem__(798, "wrong")),
            ("missing value", lambda x: x["Values"].pop()),
            ("boolean value", lambda x: x["Values"].__setitem__(0, True)),
            ("NaN value", lambda x: x["Values"].__setitem__(0, float("nan"))),
            ("out-of-range value", lambda x: x["Values"].__setitem__(0, 2**31)),
            ("extra field", lambda x: x.update(Unknown=1)),
            ("float count", lambda x: x.update(StoredCount=936.0)),
            ("reservations", lambda x: x.update(InactiveReservations=[])),
        ):
            invalid = copy.deepcopy(model)
            mutate(invalid)
            cases.append((name, json.dumps(invalid)))
        cases.append(("duplicate field", json.dumps(model)[:-1] + ', "Version": "ngn-texel-v2-936"}'))
        for name, blob in cases:
            modelpath.write_text(blob)
            result = run(command, tree, ok=False)
            assert result.returncode != 0, f"invalid {name} accepted"
            assert [path.read_bytes() for path in sources] == snapshot, f"invalid {name} changed sources"
            print(f"PASS: invalid {name} rejected before either source changes")


if __name__ == "__main__":
    main()
