#!/usr/bin/env python3
import importlib.util
from pathlib import Path
import tempfile


spec = importlib.util.spec_from_file_location("gate", "/home/ehrli/repos/ngn-counter-output-gate-20260913/driver-v2.py")
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)


def must_fail(call):
    try:
        call()
    except SystemExit:
        return
    raise AssertionError("expected fail-closed SystemExit")


with tempfile.TemporaryDirectory() as raw:
    root = Path(raw)
    direct = root / "direct.txt"
    direct.write_text("goos: linux\nBenchmarkCounterOutputDotCapturedCorpus/portable-16\t1234567\t347.25 ns/op\t0 B/op\t0 allocs/op\nPASS\n")
    assert gate.parse_direct(direct, "portable") == 347.25
    direct.write_text("BenchmarkCounterOutputDotCapturedCorpus/portable-16 123 347.25 ns/op 8 B/op 1 allocs/op\n")
    must_fail(lambda: gate.parse_direct(direct, "portable"))

    search = root / "search.txt"
    rows = [f"BenchmarkCounterTransitionFixedNodes/{fixture}-16 1 {100000000 + index}.0 ns/op 400000 minimum_nodes/op 135409280 B/op 9 allocs/op"
            for index, fixture in enumerate(gate.FIXTURES)]
    search.write_text("\n".join(rows) + "\n")
    parsed = gate.parse_search(search)
    assert list(parsed) == gate.FIXTURES
    search.write_text("\n".join(rows + [rows[0]]) + "\n")
    must_fail(lambda: gate.parse_search(search))

    correctness = root / "correctness.txt"
    lines = []
    for name in sorted(gate.CORRECTNESS_NAMES):
        lines += [f"=== RUN   {name}", f"--- PASS: {name} (0.00s)"]
    lines.append("PASS")
    correctness.write_text("\n".join(lines) + "\n")
    gate.verify_correctness_stdout(correctness)
    correctness.write_text(correctness.read_text().replace("--- PASS:", "    --- SKIP:", 1))
    must_fail(lambda: gate.verify_correctness_stdout(correctness))

print("STATIC_PARSER_TESTS_PASS")
