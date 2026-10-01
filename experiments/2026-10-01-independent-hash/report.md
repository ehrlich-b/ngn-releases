# Independent position hash replacement — 2026-10-01

Baseline: f9b44cb614e81f2ce84326f97aefcb8b8eb6f0e6 on the isolated HCE task
branch. The existing benchmark owner, default branches and released history
are preserved. No new strength games ran.

A fresh personal native Luna max thread was given the typed key-stream and
incremental/EP behavior contract, independently generated primitives, and a
test adapter. No previous hashing/position/board implementation or hidden
controller oracle was supplied. The same context was reused for corrections.
This records the generation process, not formal legal clean-room certification.

The original adapter passed a temporary probe pointer to a configurable
callback, artificially forcing heap escape. The unaccepted unsafe workaround
and native logs were preserved outside the module. The controller corrected
the adapter callback to take a value; the generator removed unsafe/runtime
escape bypasses and constructed a board-only Position probe rather than copying
potential synchronization/ownership fields. The final standalone tests, race,
checkptr and vet passed. The final native turn completed at 18:55:04 UTC.

Earlier turns stopped on a provider usage HTTP 504 and a controller timeout;
neither was a quota/payment denial. Re-reading authoritative quota notifications
was creating redundant control calls. The controller now checks the supplied
provider update payload directly, with personal account/allowance/model checks
before each turn. The last verified included weekly usage was 74%; no paid
fallback, purchase or reset was used. Cumulative native goal usage reported
153,338/600,000 tokens; that goal bound is not a credit or spending allowance.

The independently prepared proof compares all 789 key-stream values and
eight legal position histories: ordinary moves, captures, promotion, castling,
legal and pinned EP, null moves and undo, plus deterministic legal walks up to
160 plies each and full restoration. Ordinary hashes, cached/incremental hashes,
freshly generated keys, Polyglot keys, eligibility metadata and FENs match the
baseline exactly: 229,574 bytes, SHA-256
c21d87035a2b7023b323067ac3744c5339302ab7d991f816301cbfc87f242b64.
The existing independent EP/repetition and Polyglot oracle tests are preserved.

The six HCE score/move/node/PV/perft scenarios remain byte-identical, SHA-256
4b9dafa94087ae370725438b14ad53bd648f4b1a942d6add7dc27f80c5fed596. Five real-engine
EP probes also proved zero heap allocations and no source mutation, including
two candidates and illegal opened-line controls. Required engine and all-package
short/race checks passed after integration. Commands and exit codes: checks.json.
Two initial independent-probe compile errors were controller test mistakes
(undo argument order and Move formatting); they were corrected, and failed
logs retained before the actual proof. No engine failure was concealed.

All heavy work used the coordinated shared CPU0/2 50% NGN quota, nice10,
4GiB and serial Go. No timing or Elo claim follows from these results.
Historical notices remain. Two credited Zahak core implementations remain:
board and position. PeSTO-derived values and the other recorded research/data
provenance questions remain unresolved. No release, merge or CCRL resubmission
is included in this private implementation stage.
