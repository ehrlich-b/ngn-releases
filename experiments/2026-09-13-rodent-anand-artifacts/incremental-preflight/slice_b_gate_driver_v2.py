#!/usr/bin/env python3
"""Held serial Slice B oracle/check driver. Linux/WSL only."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile

SCHEMA = "ngn-rodenteval-slice-b-serial-gate-v1"
RELEASE_STATE = "RELEASED_AFTER_INDEPENDENT_REVIEW"


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def atomic_json(path, value):
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    os.replace(temporary, path)


def pending_paths(value, prefix=""):
    rows = []
    if isinstance(value, dict):
        for key, child in value.items():
            rows.extend(pending_paths(child, f"{prefix}.{key}" if prefix else key))
    elif isinstance(value, list):
        for index, child in enumerate(value):
            rows.extend(pending_paths(child, f"{prefix}[{index}]"))
    elif isinstance(value, str) and value.startswith("PENDING"):
        rows.append(prefix)
    return rows


def checked(spec, label, allow_pending=False):
    if pending_paths(spec):
        if allow_pending:
            return None
        raise RuntimeError(f"{label} is pending")
    path = Path(spec["path"]).resolve()
    if not path.is_file():
        raise RuntimeError(f"{label} absent: {path}")
    actual = sha256(path)
    if actual != spec["sha256"]:
        raise RuntimeError(f"{label} SHA mismatch: {actual}")
    return path


def safe_extract(archive, destination):
    with tarfile.open(archive, "r:gz") as source:
        members = source.getmembers()
        for member in members:
            target = (destination / member.name).resolve()
            if destination.resolve() not in target.parents and target != destination.resolve():
                raise RuntimeError(f"unsafe archive member {member.name}")
        source.extractall(destination)
    modules = list(destination.rglob("go.mod"))
    if len(modules) != 1:
        raise RuntimeError(f"tag archive contains {len(modules)} Go modules")
    return modules[0].parent


def supervise(monitor, stage, command, cpu, limit_seconds, memory_kib, cwd=None):
    argv = [
        sys.executable, str(monitor), "--label", stage.name,
        "--limit-seconds", str(limit_seconds), "--term-grace-seconds", "5",
        "--sample-interval-seconds", "0.05", "--memory-limit-kib", str(memory_kib),
        "--cpu-list", str(cpu), "--stdout", str(stage / "stdout"),
        "--stderr", str(stage / "stderr"), "--time-output", str(stage / "time.txt"),
        "--samples", str(stage / "process-tree.jsonl"), "--receipt", str(stage / "supervisor-receipt.json"),
        "--", "/usr/bin/taskset", "-c", str(cpu), *command,
    ]
    result = subprocess.run(argv, cwd=cwd, check=False)
    receipt_path = stage / "supervisor-receipt.json"
    if result.returncode != 0 or not receipt_path.is_file():
        raise RuntimeError(f"supervised stage {stage.name} failed rc={result.returncode}")
    receipt = json.loads(receipt_path.read_text())
    if receipt.get("state") != "COMPLETE" or receipt.get("command_returncode") != 0 or receipt.get("surviving_processes") or receipt.get("monitor_error"):
        raise RuntimeError(f"supervised stage {stage.name} receipt invalid")
    return receipt


def require_test_pass(log, test_name):
    text = log.read_text()
    if text.count(f"=== RUN   {test_name}") != 1 or text.count(f"--- PASS: {test_name} ") != 1:
        raise RuntimeError(f"missing/duplicate RUN/PASS for {test_name}")
    if re.search(r"(?m)^--- SKIP:", text) or "[no tests to run]" in text:
        raise RuntimeError(f"tagged test skipped or selected no tests: {test_name}")


def file_inventory(root):
    rows = {}
    excluded_directories = {"home", "tmp", "go-cache", "tagged-source", "ngn-source", "__pycache__"}
    for path in sorted(root.rglob("*")):
        relative = path.relative_to(root)
        if any(part in excluded_directories for part in relative.parts):
            continue
        if path.is_file() and path.name not in ("inventory.json", "terminal.json"):
            rows[str(relative)] = {"sha256": sha256(path), "bytes": path.stat().st_size}
    return rows


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--check-held", action="store_true")
    args = parser.parse_args()
    if sys.platform != "linux" or "microsoft" not in Path("/proc/version").read_text().lower():
        raise RuntimeError("execution restricted to WSL")
    manifest = json.loads(args.manifest.read_text())
    if manifest.get("schema") != SCHEMA:
        raise RuntimeError("wrong manifest schema")
    if manifest.get("driver_sha256") != sha256(Path(__file__).resolve()):
        raise RuntimeError("driver SHA mismatch")

    fixed = manifest["fixed_inputs"]
    fixed_paths = {name: checked(spec, name) for name, spec in fixed.items()}
    pending = pending_paths(manifest["ngn"])
    if args.check_held:
        print(json.dumps({"state": "HELD_STATIC_CHECK_COMPLETE", "fixed_inputs": len(fixed_paths), "pending": pending}, sort_keys=True))
        return
    if pending:
        raise RuntimeError(f"NGN inputs remain pending: {pending}")
    if manifest.get("release_state") != RELEASE_STATE:
        raise RuntimeError("manifest is HELD")
    ngn = manifest["ngn"]
    ngn_consumer = checked(ngn["consumer_test"], "ngn.consumer_test")
    source_root = Path(ngn["source_root"]).resolve()
    commit = subprocess.check_output(["git", "-C", str(source_root), "rev-parse", ngn["source_commit"] + "^{commit}"], text=True).strip()
    tree = subprocess.check_output(["git", "-C", str(source_root), "rev-parse", ngn["source_commit"] + "^{tree}"], text=True).strip()
    if commit != ngn["source_commit"] or tree != ngn["source_tree"]:
        raise RuntimeError("NGN commit/tree mismatch")

    run_root = Path(manifest["run_root"]).resolve()
    if run_root.exists():
        raise RuntimeError(f"immutable run_root exists: {run_root}")
    run_root.mkdir(parents=True)
    atomic_json(run_root / "frozen-manifest.json", manifest)
    inputs = run_root / "inputs"
    inputs.mkdir()
    copied_paths = {}
    for name, source in fixed_paths.items():
        destination_dir = inputs / name
        destination_dir.mkdir()
        destination = destination_dir / source.name
        shutil.copy2(source, destination)
        if sha256(destination) != fixed[name]["sha256"]:
            raise RuntimeError(f"copy changed {name}")
        copied_paths[name] = destination
    consumer_dir = inputs / "ngn_consumer_test"
    consumer_dir.mkdir()
    consumer_copy = consumer_dir / ngn_consumer.name
    shutil.copy2(ngn_consumer, consumer_copy)
    if sha256(consumer_copy) != ngn["consumer_test"]["sha256"]:
        raise RuntimeError("copy changed ngn.consumer_test")

    monitor = copied_paths["monitor"]
    resources = manifest["resources"]
    expected_resources = {
        "cpu": "4",
        "tagged": {"limit_seconds": 600, "memory_limit_kib": 2097152},
        "release": {"limit_seconds": 60, "memory_limit_kib": 1048576},
        "ngn": {"limit_seconds": 1800, "memory_limit_kib": 4194304},
    }
    if resources != expected_resources:
        raise RuntimeError("resource contract changed")
    cpu = resources["cpu"]
    results = {}

    tagged_stage = run_root / "01-tagged-mechanism"
    tagged_stage.mkdir()
    tagged_source_container = run_root / "tagged-source"
    tagged_source_container.mkdir()
    tagged_source = safe_extract(copied_paths["tagged_source_archive"], tagged_source_container)
    tagged_source_files = {
        "tagged_uci_go": "uci.go", "tagged_options_go": "options.go",
        "tagged_nnue_go": "nnue.go", "tagged_eval_go": "eval.go",
    }
    for key, relative in tagged_source_files.items():
        extracted = tagged_source / relative
        if not extracted.is_file() or sha256(extracted) != fixed[key]["sha256"]:
            raise RuntimeError(f"tag archive source mismatch: {relative}")
    shutil.copy2(copied_paths["tagged_harness"], tagged_source / "rodent_sliceb_oracle_test.go")
    tagged_output = tagged_stage / "tagged-oracle.json"
    tagged_env = [
        "/usr/bin/env", "-i", "PATH=/usr/local/go/bin:/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC",
        "GOMAXPROCS=2", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOAMD64=v3", "CGO_ENABLED=0",
        f"HOME={tagged_stage / 'home'}", f"TMPDIR={tagged_stage / 'tmp'}", f"GOCACHE={tagged_stage / 'go-cache'}",
        f"RODENT_V11_ANAND_MODEL={copied_paths['network']}",
        f"RODENT_V11_ANAND_TRANSITION_FIXTURES={copied_paths['transition_fixtures']}",
        f"RODENT_V11_ANAND_TAGGED_ORACLE_OUTPUT={tagged_output}",
        "/usr/local/go/bin/go", "test", "-v", "-count=1", "-tags", "rodentsliceboracle",
        "-run", "^TestRodentV11AnandSliceBTaggedTransitionOracle$", ".",
    ]
    for directory in (tagged_stage / "home", tagged_stage / "tmp", tagged_stage / "go-cache"):
        directory.mkdir(parents=True, exist_ok=True)
    results["tagged"] = supervise(monitor, tagged_stage, tagged_env, cpu,
                                   resources["tagged"]["limit_seconds"], resources["tagged"]["memory_limit_kib"],
                                   cwd=tagged_source)
    require_test_pass(tagged_stage / "stdout", "TestRodentV11AnandSliceBTaggedTransitionOracle")
    if not tagged_output.is_file():
        raise RuntimeError("tagged oracle output absent")

    release_stage = run_root / "02-release-raw"
    release_stage.mkdir()
    for directory in (release_stage / "home", release_stage / "tmp"):
        directory.mkdir()
    release_command = [
        "/usr/bin/env", "-i", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC", "GOMAXPROCS=1",
        f"HOME={release_stage / 'home'}", f"TMPDIR={release_stage / 'tmp'}",
        "/usr/bin/python3", str(copied_paths["release_raw_driver"]),
        "--fixtures", str(copied_paths["transition_fixtures"]), "--output", str(release_stage / "artifacts"),
    ]
    results["release"] = supervise(monitor, release_stage, release_command, cpu,
                                    resources["release"]["limit_seconds"], resources["release"]["memory_limit_kib"])
    release_output = release_stage / "artifacts" / "oracle.json"
    if not release_output.is_file():
        raise RuntimeError("release raw oracle absent")

    ngn_stage = run_root / "03-ngn-checks"
    ngn_stage.mkdir()
    ngn_source = run_root / "ngn-source"
    ngn_source.mkdir()
    archive_path = run_root / "ngn-source.tar"
    with archive_path.open("wb") as stream:
        archived = subprocess.run(["git", "-C", str(source_root), "archive", ngn["source_commit"]], stdout=stream, check=False)
    if archived.returncode != 0:
        raise RuntimeError("git archive failed")
    with tarfile.open(archive_path, "r:") as archive:
        archive.extractall(ngn_source)
    overlays = {
        "rodenteval/context.go": copied_paths["context_source"],
        "rodenteval/context_test.go": copied_paths["context_tests"],
        "rodenteval/context_oracle_test.go": consumer_copy,
    }
    for relative, source in overlays.items():
        destination = ngn_source / relative
        if not destination.parent.is_dir():
            raise RuntimeError(f"overlay parent absent: {destination.parent}")
        shutil.copy2(source, destination)
        if sha256(destination) != sha256(source):
            raise RuntimeError(f"overlay copy changed: {relative}")
    atomic_json(run_root / "source-assembly-receipt.json", {
        "schema": "ngn-rodenteval-slice-b-source-assembly-v1",
        "base_commit": commit,
        "base_tree": tree,
        "git_archive_sha256": sha256(archive_path),
        "overlays": {relative: sha256(source) for relative, source in overlays.items()},
    })
    ngn_config = run_root / "ngn-check-config.json"
    atomic_json(ngn_config, {
        "schema": "ngn-rodenteval-slice-b-check-config-v1", "source_root": str(ngn_source),
        "output": str(ngn_stage / "checks"), "unit_tests": ngn["unit_tests"], "oracle_test": ngn["oracle_test"],
        "oracle_environment": {
            "RODENT_V11_ANAND_MODEL": str(copied_paths["network"]),
            "RODENT_V11_ANAND_TRANSITION_FIXTURES": str(copied_paths["transition_fixtures"]),
            "RODENT_V11_ANAND_TAGGED_TRANSITION_ORACLE": str(tagged_output),
            "RODENT_V11_ANAND_RELEASE_TRANSITION_ORACLE": str(release_output),
        },
    })
    ngn_command = ["/usr/bin/env", "-i", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC",
                   "/usr/bin/python3", str(copied_paths["ngn_checks_runner"]), "--config", str(ngn_config)]
    results["ngn"] = supervise(monitor, ngn_stage, ngn_command, cpu,
                                resources["ngn"]["limit_seconds"], resources["ngn"]["memory_limit_kib"])
    summary = ngn_stage / "checks" / "summary.json"
    if not summary.is_file() or json.loads(summary.read_text()).get("state") != "COMPLETE":
        raise RuntimeError("NGN check summary absent/incomplete")

    inventory = {"schema": "ngn-rodenteval-slice-b-serial-inventory-v1", "files": file_inventory(run_root)}
    atomic_json(run_root / "inventory.json", inventory)
    terminal = {
        "schema": "ngn-rodenteval-slice-b-serial-terminal-v1", "state": "COMPLETE",
        "claim_boundary": "tagged lanes are mechanism evidence; exact release supplies raw score authority; no strength/performance claim",
        "manifest_sha256": sha256(run_root / "frozen-manifest.json"), "stages": results,
        "inventory_sha256": sha256(run_root / "inventory.json"),
    }
    atomic_json(run_root / "terminal.json", terminal)
    print(json.dumps({"state": "COMPLETE", "terminal": str(run_root / "terminal.json")}, sort_keys=True))


if __name__ == "__main__":
    main()
