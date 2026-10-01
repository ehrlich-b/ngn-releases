#!/usr/bin/env python3
import importlib.util
from pathlib import Path
import tempfile

DRIVER = Path("/home/ehrli/repos/ngn-counter-fusion-gate-20260913/driver-held-v1.py")
spec = importlib.util.spec_from_file_location("fusion_gate", DRIVER)
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)


def row(benchmark, fixture, nodes, persistent=False):
    extra = "  2 searches/op" if persistent else ""
    return f"{benchmark}/{fixture}-8  1  123.5 ns/op  {nodes} minimum_nodes/op{extra}  64 B/op  2 allocs/op"


with tempfile.TemporaryDirectory() as temporary:
    path = Path(temporary) / "stdout.txt"
    path.write_text("\n".join([row("BenchmarkCounterProfileCold", name, 400000)
                               for name in gate.COLD_FIXTURES] + ["PASS", ""]))
    assert list(gate.parse_benchmark_family(path, "cold")) == gate.COLD_FIXTURES

    path.write_text("\n".join([row("BenchmarkCounterProfilePersistent", name, 800000, True)
                               for name in gate.PERSISTENT_FIXTURES] + ["PASS", ""]))
    assert list(gate.parse_benchmark_family(path, "persistent")) == gate.PERSISTENT_FIXTURES

    path.write_text("=== RUN   TestUnexpected\n--- PASS: TestUnexpected (0.00s)\nPASS\n")
    try:
        gate.parse_benchmark_family(path, "cold")
    except SystemExit:
        pass
    else:
        raise AssertionError("ordinary test output was accepted")

print("PASS parser smoke")
