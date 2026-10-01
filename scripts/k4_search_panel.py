#!/usr/bin/env python3
"""Run a frozen K4/HCE/Rodent UCI search panel on one shared-host CPU lane."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import queue
import subprocess
import threading
import time


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


class UCI:
    def __init__(self, command: list[str], options: dict[str, str], name: str):
        self.name = name
        self.process = subprocess.Popen(
            command, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, text=True, bufsize=1,
        )
        self.lines: queue.Queue[str | None] = queue.Queue()
        self.errors: list[str] = []
        threading.Thread(target=self._read_stdout, daemon=True).start()
        threading.Thread(target=self._read_stderr, daemon=True).start()
        self.send("uci")
        self.wait_exact("uciok", 10)
        for key, value in options.items():
            self.send(f"setoption name {key} value {value}")
        self.ready(10)

    def _read_stdout(self) -> None:
        assert self.process.stdout is not None
        for line in self.process.stdout:
            self.lines.put(line.strip())
        self.lines.put(None)

    def _read_stderr(self) -> None:
        assert self.process.stderr is not None
        for line in self.process.stderr:
            if len(self.errors) < 100:
                self.errors.append(line.rstrip()[:500])

    def send(self, line: str) -> None:
        assert self.process.stdin is not None
        if "\n" in line or "\r" in line:
            raise ValueError("multiline UCI command")
        self.process.stdin.write(line + "\n")
        self.process.stdin.flush()

    def next_line(self, deadline: float) -> str:
        try:
            line = self.lines.get(timeout=max(0.001, deadline - time.monotonic()))
        except queue.Empty as exc:
            raise TimeoutError(f"{self.name} UCI timeout; stderr={self.errors[-5:]}") from exc
        if line is None:
            raise RuntimeError(f"{self.name} stdout closed; stderr={self.errors[-5:]}")
        return line

    def wait_exact(self, wanted: str, seconds: float) -> None:
        deadline = time.monotonic() + seconds
        while True:
            if self.next_line(deadline) == wanted:
                return

    def ready(self, seconds: float) -> None:
        self.send("isready")
        self.wait_exact("readyok", seconds)

    @staticmethod
    def parse_info(line: str) -> dict | None:
        fields = line.split()
        if not fields or fields[0] != "info" or "score" not in fields:
            return None
        if "lowerbound" in fields or "upperbound" in fields:
            return None
        info = {}
        for key in ("depth", "seldepth", "nodes", "time", "nps"):
            if key in fields:
                index = fields.index(key)
                if index + 1 < len(fields):
                    info[key] = int(fields[index + 1])
        index = fields.index("score")
        if index + 2 >= len(fields) or fields[index + 1] not in ("cp", "mate"):
            return None
        info["score_kind"] = fields[index + 1]
        info["score"] = int(fields[index + 2])
        if "pv" in fields:
            index = fields.index("pv")
            if index + 1 < len(fields):
                info["pv_move"] = fields[index + 1]
        return info

    def search(self, fen: str, mode: str, budget: int) -> dict:
        self.send("ucinewgame")
        self.ready(10)
        self.send("position fen " + fen)
        command = "go nodes" if mode == "nodes" else "go movetime"
        self.send(f"{command} {budget}")
        started = time.monotonic()
        deadline = started + (45 if mode == "nodes" else 8)
        last_info = None
        stopped = False
        while True:
            try:
                line = self.next_line(deadline)
            except TimeoutError:
                if stopped:
                    raise
                self.send("stop")
                stopped = True
                deadline = time.monotonic() + 2
                continue
            info = self.parse_info(line)
            if info is not None:
                last_info = info
            if line.startswith("bestmove "):
                best = line.split()[1]
                if best in ("(none)", "0000") or last_info is None:
                    raise RuntimeError(f"{self.name} no legal bestmove or score for {fen}: {line}")
                elapsed = time.monotonic() - started
                nodes = last_info.get("nodes", 0)
                return {
                    "best_move": best,
                    "elapsed_ms": round(elapsed * 1000, 3),
                    "wall_ns_per_reported_node": round(elapsed * 1e9 / nodes, 3) if nodes else None,
                    **last_info,
                }

    def close(self) -> None:
        if self.process.poll() is not None:
            return
        try:
            self.send("quit")
            self.process.wait(timeout=3)
        except (BrokenPipeError, subprocess.TimeoutExpired):
            self.process.kill()
            self.process.wait(timeout=3)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--panel", type=Path, required=True)
    parser.add_argument("--panel-sha256", required=True)
    parser.add_argument("--engine", type=Path, required=True)
    parser.add_argument("--k4-model", type=Path, required=True)
    parser.add_argument("--rodent-model", type=Path, required=True)
    parser.add_argument("--teacher", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.output.exists():
        raise FileExistsError(args.output)
    if sha256(args.panel) != args.panel_sha256:
        raise ValueError("frozen panel SHA mismatch")
    panel = json.loads(args.panel.read_text())
    if panel["schema"] != "ngn-k4-search-panel-v1" or len(panel["positions"]) != 32:
        raise ValueError("wrong frozen panel contract")
    for path, key in ((args.engine, "engine_sha256"), (args.k4_model, "k4_model_sha256"),
                      (args.rodent_model, "rodent_model_sha256"), (args.teacher, "teacher_sha256")):
        if sha256(path) != panel[key]:
            raise ValueError(f"{key} mismatch: {path}")
    config = panel["search"]
    if config != {"teacher_nodes": 200000, "equal_nodes": 100000,
                  "equal_movetime_ms": 250, "threads": 1, "hash_mib": 128,
                  "move_overhead_ms": 100, "concurrency": 1}:
        raise ValueError("unexpected search budget")
    sessions = {}
    try:
        sessions["teacher"] = UCI(
            [str(args.teacher)], {"Threads": "1", "Hash": "16", "MultiPV": "1",
                                  "SyzygyPath": "<empty>"}, "teacher")
        options = {"Threads": "1", "Hash": "128", "Move Overhead": "100"}
        sessions["hce"] = UCI([str(args.engine)], options, "hce")
        sessions["k4"] = UCI([str(args.engine), "-eval-backend", "ngn-k4-768-v1",
                              "-eval-file", str(args.k4_model)], options, "k4")
        sessions["rodent"] = UCI([str(args.engine), "-eval-backend", "rodent-v1.1-anand",
                                  "-eval-file", str(args.rodent_model)], options, "rodent")
        rows = []
        for index, position in enumerate(panel["positions"]):
            row = {"id": position["id"], "fen": position["fen"]}
            row["teacher"] = sessions["teacher"].search(position["fen"], "nodes", config["teacher_nodes"])
            for mode, budget in (("equal_nodes", config["equal_nodes"]),
                                 ("equal_time", config["equal_movetime_ms"])):
                row[mode] = {}
                order = ("hce", "k4", "rodent")
                order = order[index % 3:] + order[:index % 3]
                for backend in order:
                    row[mode][backend] = sessions[backend].search(
                        position["fen"], "nodes" if mode == "equal_nodes" else "movetime", budget)
            rows.append(row)
        summaries = {}
        for mode in ("equal_nodes", "equal_time"):
            summaries[mode] = {}
            for backend in ("hce", "k4", "rodent"):
                searches = [row[mode][backend] for row in rows]
                summaries[mode][backend] = {
                    "teacher_move_agreement": sum(
                        row[mode][backend]["best_move"] == row["teacher"]["best_move"] for row in rows
                    ),
                    "positions": len(rows),
                    "mean_depth": sum(search.get("depth", 0) for search in searches) / len(searches),
                    "mean_nodes": sum(search.get("nodes", 0) for search in searches) / len(searches),
                    "mean_elapsed_ms": sum(search["elapsed_ms"] for search in searches) / len(searches),
                    "mean_wall_ns_per_node": sum(search["wall_ns_per_reported_node"] for search in searches) / len(searches),
                }
        result = {
            "schema": "ngn-k4-search-panel-result-v1",
            "panel_sha256": args.panel_sha256,
            "script_sha256": sha256(Path(__file__)),
            "binary_sha256": {"engine": panel["engine_sha256"], "k4": panel["k4_model_sha256"],
                              "rodent": panel["rodent_model_sha256"], "teacher": panel["teacher_sha256"]},
            "cpu_affinity": sorted(os.sched_getaffinity(0)),
            "nice": os.nice(0),
            "summaries": summaries,
            "rows": rows,
        }
        with args.output.open("x") as destination:
            json.dump(result, destination, indent=2)
            destination.write("\n")
        print(json.dumps(summaries, indent=2))
    finally:
        for session in sessions.values():
            session.close()


if __name__ == "__main__":
    main()
