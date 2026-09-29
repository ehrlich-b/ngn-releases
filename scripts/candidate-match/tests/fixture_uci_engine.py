#!/usr/bin/python3
from __future__ import annotations

import sys

variant = sys.argv[1] if len(sys.argv) > 1 else "good"
debug = False
backend = "hce"
ownbook = True

for raw in sys.stdin:
    line = raw.strip()
    if line == "uci":
        print("id name CandidateFixture 1")
        print("id author fixture")
        hash_max = 32 if variant == "narrow-hash" else 1024
        hash_default = min(128, hash_max)
        print(f"option name Hash type spin default {hash_default} min 1 max {hash_max}")
        print("option name Threads type spin default 1 min 1 max 64")
        print("option name Move Overhead type spin default 100 min 0 max 5000")
        print("option name OwnBook type check default true")
        if variant == "missing-backend-var":
            print("option name EvalBackend type combo default ngn-v1 var ngn-v1")
        else:
            print("option name EvalBackend type combo default hce var hce var ngn-v1")
        print("option name EvalFile type string default <empty>")
        print("uciok", flush=True)
    elif line == "debug on":
        debug = True
    elif line == "isready":
        print("readyok", flush=True)
    elif line.startswith("setoption name "):
        body = line[len("setoption name "):]
        name, value = body.split(" value ", 1)
        if name == "EvalBackend":
            backend = value
        elif name == "OwnBook" and variant != "ignore-ownbook":
            ownbook = value.lower() == "true"
        if debug and name in {"Threads", "OwnBook", "Hash", "Move Overhead"}:
            print(f"info string option set: {name} = {value}", flush=True)
    elif line == "eval":
        print(f"info string eval backend {backend} score_cp 0 pov side-to-move policy base-SearchSTM rule50-and-backend-adapter correction-history excluded", flush=True)
    elif line == "ucinewgame":
        if variant == "reset-ownbook":
            ownbook = True
    elif line == "go depth 2":
        if variant == "error-fallback":
            print("info string error injected search failure")
        if ownbook:
            print("info depth 1 score cp 50 nodes 1 time 0 nps 0 pv e2e4")
        else:
            print("info depth 1 score cp 0 nodes 20 time 1 nps 20000 pv e2e4")
            print("info depth 2 score cp 3 nodes 81 time 2 nps 40500 pv e2e4 e7e5")
        print("bestmove e2e4", flush=True)
        if variant == "late-error":
            print("info string error injected after bestmove", flush=True)
    elif line == "quit":
        raise SystemExit(0)
