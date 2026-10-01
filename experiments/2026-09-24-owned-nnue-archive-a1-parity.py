#!/usr/bin/env python3
"""Explain calibration BF boards not found among the archive probe boards."""
import csv, glob, hashlib, json, struct, sys

probe_csv, labels_glob, calib_bf = sys.argv[1:4]
KB = [1, 1, 0, 0, 0, 0, 1, 1] + [2] * 8 + [3] * 48

def feature(color, piece, sq, ksq, persp):
    s, k = sq, ksq
    if persp == 1:
        s ^= 56
        k ^= 56
    if ksq % 8 > 3:
        s ^= 7
    return KB[k] * 768 + (color ^ persp) * 384 + piece * 64 + s

def decode(rec):
    occ = struct.unpack_from("<Q", rec, 0)[0]
    pieces = []
    idx = 0
    while occ:
        sq = (occ & -occ).bit_length() - 1
        occ &= occ - 1
        nib = (rec[8 + idx // 2] >> (4 * (idx & 1))) & 15
        pieces.append((nib >> 3, nib & 7, sq))
        idx += 1
    return pieces  # (color, piece, square), side to move is white

def key(pieces):
    kings = {c: sq for c, p, sq in pieces if p == 5}
    feats = [sorted(feature(c, p, sq, kings[0], 0) for c, p, sq in pieces),
             sorted(feature(c, p, sq, kings[1], 1) for c, p, sq in pieces)]
    h = hashlib.sha256(b"ngn-k4-input-v1\0")
    for fl in feats:
        h.update(struct.pack("<H", len(fl)))
        for f in fl:
            h.update(struct.pack("<H", f))
    h.update(bytes([min(7, max(0, len(pieces) - 2) // 4)]))
    return h.hexdigest()

rows = {}
with open(probe_csv) as f:
    for row in csv.DictReader(f):
        rows.setdefault(row["k4_input_sha256"], []).append(bytes.fromhex(row["board_hex"]))
boards = {b[:24] + b[27:29] for bs in rows.values() for b in bs}
bf = open(calib_bf, "rb").read()
missing = key_found = key_not_found = any_occ_match = 0
examples = []
for i in range(len(bf) // 32):
    rec = bf[32 * i: 32 * i + 32]
    if rec[:24] + rec[27:29] in boards:
        continue
    missing += 1
    k = key(decode(rec))
    if k in rows:
        key_found += 1
        if len(examples) < 3:
            examples.append({"go": rec.hex(), "ours": [b.hex() for b in rows[k]]})
    else:
        key_not_found += 1
sane = sum(1 for i in range(2000) if key(decode(bf[32*i:32*i+32])) in rows)
print("key_function_sanity_first_2000", sane)
print(json.dumps({"missing_boards": missing, "key_recomputed_and_in_probe": key_found,
                  "key_not_in_probe": key_not_found, "examples": examples}, indent=1))
