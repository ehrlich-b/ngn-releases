#!/usr/bin/env bash
# Build the CCRL-rated Blunder opponent ladder for the rating gauntlet.
#
# Blunder (github.com/algerbrex/blunder) is an open-source Go engine with
# published CCRL ratings, so it anchors NGN's absolute rating far better than
# Stockfish's UCI_Elo slider (which we measured ~+400 inflated vs CCRL). This
# clones Blunder once, checks out tagged releases, and builds each natively into
# opponents/. The built binaries are gitignored; opponents/ratings.json (the
# version->CCRL map) is the checked-in, reproducible spec.
#
# Usage: scripts/build-anchors.sh   (idempotent; re-run to rebuild from the tags)
set -euo pipefail
cd "$(dirname "$0")/.."

OUT=opponents
SRC="$OUT/.src"
mkdir -p "$OUT"

if [ ! -d "$SRC/.git" ]; then
	git clone https://github.com/algerbrex/blunder.git "$SRC"
fi
git -C "$SRC" fetch --tags --quiet

build() { # <tag> <outname>
	git -C "$SRC" -c advice.detachedHead=false checkout --quiet "$1"
	( cd "$SRC" && go build -o "../../$OUT/$2" blunder/main.go )
	echo "built $OUT/$2 from $1"
}

# version -> CCRL Blitz rating (kept in sync with opponents/ratings.json)
build v6.1.0 blunder_610   # 2155 — lower bracket
build v7.2.0 blunder_720   # 2425
build v7.4.0 blunder_740   # 2532 — the anchor that exposed the SF inflation
build v5.0.0 blunder_500   # 2080 — fallback bracket (NGN favored)
build v8.0.0 blunder_800   # 2674 — fallback bracket (NGN underdog)

git -C "$SRC" checkout --quiet - 2>/dev/null || true

# Cross-compile the Blunder rungs for linux/arm64 (Graviton) so scripts/cloudsprt.sh
# can ship them to the cloud gauntlet boxes. Go cross-compiles trivially; Fruit (C++)
# is omitted from the cloud set. Gitignored like the native binaries (reproducible here).
mkdir -p "$OUT/linux-arm64"
lbuild() { # <tag> <outname>
	git -C "$SRC" -c advice.detachedHead=false checkout --quiet "$1"
	( cd "$SRC" && GOOS=linux GOARCH=arm64 go build -o "../linux-arm64/$2" blunder/main.go )
	echo "built $OUT/linux-arm64/$2 (linux/arm64) from $1"
}
lbuild v5.0.0 blunder_500
lbuild v6.1.0 blunder_610
lbuild v7.2.0 blunder_720
lbuild v7.4.0 blunder_740
lbuild v8.0.0 blunder_800
git -C "$SRC" checkout --quiet - 2>/dev/null || true

# CounterGo 3.8 (Vadim Chizhov) — pure-Go HCE, CCRL Blitz ~2994. A strong NON-Blunder
# cross-family TOP anchor that un-saturates the ladder (NGN scores well below it). "3.8"
# is a release COMMIT (b172b99), NOT a git tag (the 3.6-4.0 version bumps were committed
# untagged); pre-NNUE so it's pure classical. Pure Go => native + linux/arm64 like Blunder.
CTRSRC="$OUT/.src-counter"
if [ ! -d "$CTRSRC/.git" ]; then
	git clone https://github.com/ChizhovVadim/CounterGo.git "$CTRSRC"
fi
git -C "$CTRSRC" -c advice.detachedHead=false checkout --quiet b172b99
( cd "$CTRSRC" && go build -o "../../$OUT/counter_38" ./counter )
( cd "$CTRSRC" && GOOS=linux GOARCH=arm64 go build -o "../linux-arm64/counter_38" ./counter )
echo "built $OUT/counter_38 (+linux-arm64) from CounterGo 3.8 (b172b99)"

# Fruit 2.1 (Fabien Letouzey, 2005) — an independent NON-Blunder anchor: a different
# engine family that exposes anchor non-transitivity the Blunder ladder hides, and
# with no movetime-overshoot bug. CCRL Blitz 2685. Old C++, so build with an old
# standard under clang (the arm64 host has no GNU g++).
FRUITSRC="$OUT/.src-fruit"
if [ ! -d "$FRUITSRC/.git" ]; then
	git clone --depth 1 https://github.com/Warpten/Fruit-2.1.git "$FRUITSRC"
fi
clang++ -std=gnu++98 -w -O3 -fno-exceptions -fno-rtti -fstrict-aliasing -fomit-frame-pointer \
	-o "$OUT/fruit_21" "$FRUITSRC"/src/*.cpp -lm
echo "built $OUT/fruit_21 from Fruit-2.1 src"

echo "done: $(ls "$OUT" | grep -E '^(blunder_|fruit_)' | tr '\n' ' ')"
