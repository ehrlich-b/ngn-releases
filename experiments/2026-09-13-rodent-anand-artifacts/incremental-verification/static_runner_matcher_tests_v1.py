#!/usr/bin/env python3
"""Static tests for the required-parent receipt matcher; runs no Go code."""

import importlib.util
from pathlib import Path
import tempfile


RUNNER = Path("/home/ehrli/rodent-v1.1-anand-sliceb-gate-20260913/source-v4/ngn_checks_v1.py")
SAVED = Path("/home/ehrli/rodent-v1.1-anand-sliceb-gate-20260913/run-001/03-ngn-checks/checks/unit.log")


def load_runner():
    spec = importlib.util.spec_from_file_location("ngn_checks_v4", RUNNER)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def expect_rejected(module, label, text):
    with tempfile.TemporaryDirectory() as directory:
        output = Path(directory)
        command = ["/usr/bin/printf", "%s", text]
        try:
            module.run_one(label, command, output, {}, output, ("TestParent",))
        except RuntimeError:
            return
        raise AssertionError(f"{label} unexpectedly accepted")


def main():
    module = load_runner()
    saved = SAVED.read_text()
    parents = (
        "TestSearchContextMoveFamiliesMatchFullRefreshAllLanes",
        "TestSearchContextAllPromotionsMatchFullRefreshAllLanes",
        "TestSearchContextKingMirrorBoundaryBothDirectionsAndColors",
        "TestSearchContextMultiPlyUnwindAndSiblingReuse",
        "TestApplyUpdatesRefreshesOnlyCrossingKingPerspective",
        "TestSearchContextIncrementalArithmeticWrapsInt16",
        "TestSearchContextRejectsTransitionsTransactionally",
        "TestSearchContextNullPopResetAndCapacityGrowth",
        "TestSearchContextsOwnIndependentFrames",
        "TestSearchContextRejectsUnvalidatedModelAndPosition",
    )
    for parent in parents:
        assert module.exact_parent_event_count(saved, "=== RUN   ", parent) == 1
        assert module.exact_parent_event_count(saved, "--- PASS: ", parent) == 1

    expect_rejected(module, "missing", "PASS\n")
    expect_rejected(module, "duplicate", "=== RUN   TestParent\n=== RUN   TestParent\n--- PASS: TestParent (0.00s)\n")
    expect_rejected(module, "child_only", "=== RUN   TestParent/child\n--- PASS: TestParent/child (0.00s)\n")
    expect_rejected(module, "required_skip", "=== RUN   TestParent\n    --- SKIP: TestParent/child (0.00s)\n--- PASS: TestParent (0.00s)\n")
    print("PASS saved_parent_counts=10 synthetic_rejections=4")


if __name__ == "__main__":
    main()
