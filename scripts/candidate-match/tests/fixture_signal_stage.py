#!/usr/bin/python3
from __future__ import annotations

import json
import os
import subprocess
import sys
import time
from pathlib import Path

marker = Path(sys.argv[1])
child = subprocess.Popen(["/usr/bin/sleep", "60"], start_new_session=True)
marker.write_text(json.dumps({"stage_pid": os.getpid(), "child_pid": child.pid}) + "\n", encoding="utf-8")
time.sleep(60)
