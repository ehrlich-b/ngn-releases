#!/usr/bin/python3
from __future__ import annotations

import json
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))

from common import ActiveSupervisor, install_termination_handlers, restore_signal_handlers, safe_environment, supervisor_command

output = Path(sys.argv[1]).resolve()
output.mkdir(parents=True, exist_ok=False)
stage_dir = output / "stage"
active = ActiveSupervisor()
previous = install_termination_handlers()
try:
    argv = supervisor_command(
        HERE.parent / "process_supervisor.py", "signal-fixture", stage_dir, 30.0,
        1048576, "12,14", [sys.executable, str(HERE / "fixture_signal_stage.py"), str(output / "ready.json")],
        1.0, 0.02,
    )
    active.run(argv, output, safe_environment())
    raise RuntimeError("signal fixture unexpectedly completed")
except BaseException as error:
    restore_signal_handlers(previous)
    cleanup = active.terminate(2.0)
    terminal = {"state": "FAILED_SIGNAL", "error": str(error), "cleanup": cleanup}
    (output / "terminal.json").write_text(json.dumps(terminal, sort_keys=True) + "\n", encoding="utf-8")
    raise SystemExit(1)
finally:
    restore_signal_handlers(previous)
