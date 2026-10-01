# Lazy SMP transposition-table read publication

Status: **completed / SHELVE** against `df99782`. The read-publication
candidate removed synchronized probe blocking but failed its predeclared
whole-search speed screen. No game gate ran and no production source changed.

## Basis

The post-Rodent, width-eight profile used the accepted source, exact Anand
network and the established physical CPU mask while the user-directed Lean
hopper remained live. Six one-second searches completed under supervision at
each of widths one and eight. Width eight averaged 9,853,167 aggregate nodes
per search versus 1,469,454 at width one (6.705x), so this is not evidence that
Lazy SMP is generally broken.

At width eight, `Cache.Get` accounted for 5.33 of 54.26 sampled CPU seconds
(9.82% cumulative). Its stripe `Mutex.Lock` accounted for 1.22 seconds. The
block profile attributed 0.20 seconds to probe locks and 0.28 seconds to store
locks across the six searches. Within stores, 0.24 seconds was the global age
lock and only 0.04 seconds the stripe lock. The age-lock-only ceiling is too
small to justify its own candidate. This experiment instead addresses the
measured per-probe synchronization cost.

The raw profiles and supervisor evidence remain under
`/home/ehrli/repos/ngn-tt-profile-20260919/output/tt-contention-profile-20260919`
on WSL. The profile is a prioritization measurement, not a speed or Elo result.

## Candidate boundary

Change synchronized TT publication only:

- keep the existing direct-mode fields and plain one-worker access path;
- keep one writer mutex per existing stripe, the global age lock, all packing,
  hashing, replacement and age-width behavior;
- add one atomic sequence counter per stripe. A replacing writer publishes an
  odd sequence, atomically writes the key/data words in the existing order,
  then publishes the next even sequence before releasing the writer mutex;
- synchronized probes take one atomic key/data snapshot bracketed by sequence
  reads. An odd or changed sequence is a miss, with no spin or retry;
- synchronized diagnostics use the same stable-snapshot rule; and
- do not change stripe count, search policy, helper scheduling/result ownership,
  time management, evaluator code or one-thread behavior.

This is memory-model synchronization, not reliance on the XOR checksum alone.
The deliberate busy-write miss means the candidate is not node-identical under
Lazy SMP and must pass a playing-strength gate before adoption.

## Required evidence and decision rule

1. Add adversarial synchronized-record tests for coherent reads under competing
   writers, odd/in-flight rejection and stable post-publication reads. Run the
   focused TT/Lazy-SMP tests, full short engine suite, full repository short
   suite and full short race suite on WSL.
2. Confirm exact one-worker fixed-search move, score, nodes and PV identity on
   the established deterministic fixtures. Confirm direct-mode TT replacement
   and packing behavior remains exact.
3. Build clean isolated base and candidate Linux/amd64-v3 artifacts with
   identical flags. Run an alternating paired width-eight screen on the same
   six Rodent fixtures and physical CPU mask. Continue only if the candidate's
   median aggregate real-clock throughput improves by at least 1.0%, at least
   four of six fixture medians improve, and no fixture median regresses by more
   than 1.0%. Also confirm the probe lock disappears from a bounded CPU/block
   profile without a new material hotspot.
4. If and only if the speed screen passes, freeze a fresh width-eight paired
   real-clock game manifest against the immediate base and run a same-binary
   A/A first. Retain the validated Rodent V1.1 Anand, Threads/GOMAXPROCS 8,
   Hash 128, physical CPUs `0,2,4,6,8,10,12,14`, concurrency one, paired
   six-ply openings and `10+0.1` settings. Shared-host CPU is disclosed, not a
   standalone rejection condition, and the Lean hopper must not be paused.
5. Adopt only if the completed candidate gate has a positive paired-bootstrap
   95% Elo lower bound and every legal/process/protocol/effective-width audit
   passes. Otherwise shelve with no extension or provisional keep. Do not pool
   this result with either completed-helper SMP experiment.

Any correctness failure, direct-mode identity failure or speed-screen miss
ends the candidate before games. A speed pass alone is not an Elo or 3300 claim.

## Frozen implementation and checks

The exact rejected source-and-test diff is retained on WSL as
`output/tt-contention-profile-20260919/rejected-candidate.patch`, SHA-256
`77de806e2500d237150b8741a7414754135c4f53fe3fd9da4267647c04138f7a`.
It followed the boundary above: direct mode, replacement, packed data, age
locking, stripe count and search code were unchanged. Synchronized replacement
held the existing writer mutex and bracketed atomic key/data publication with
one per-stripe sequence; a probe returned a miss on an odd or changed sequence.

The following WSL checks passed before the speed decision:

- focused synchronized-cache race tests, including competing writers and the
  new in-flight/stable-publication witness;
- focused Lazy-SMP race tests;
- the complete short engine suite; and
- the complete repository short suite.

The full repository race suite and separate one-worker artifact-identity pass
were not run after the speed gate rejected the candidate. They cannot rescue a
failed speed candidate and are not claimed as completed acceptance evidence.

## Profile screen

An initial matched candidate profile removed `Cache.Get` from the block profile
and reduced its sampled cumulative CPU share from 9.82% to 8.02%. That isolated
sample nevertheless moved aggregate nodes from 9,853,167 to 9,822,651 per
one-second search (−0.31%). This was only a warning; the frozen paired screen
below made the decision.

The six-fixture probe was byte-identical in the isolated base and candidate
trees, SHA-256
`9702ae5b30e172fa0e19faea868eb7009a689361242c99f83e4380f5c40dde55`.
The runner SHA-256 was
`0abdd8c48d595f91db50596aadf9e3902e279eafd83fe126b2da6b516f066061`.
Identical Linux/amd64-v3 build commands produced:

- base test artifact:
  `75aa85dc852e2e9bf2bbf7c66378c114214c65761712299c0002a5e1765c494c`;
- candidate test artifact:
  `1549c7f1e0fcf692cedffd4507b4502c06a2584abd6cb6fa9d74dcc658ac2b70`.

The fixed screen ran ten paired blocks, alternating which role ran first in
each block. Every role/block searched all six fixtures for one second with the
exact Anand model, Threads/GOMAXPROCS 8 and physical CPUs
`0,2,4,6,8,10,12,14`: 60 searches per role, 120 total. Per-fixture median
candidate/base throughput ratios were:

| Fixture | Median ratio | Change |
|---|---:|---:|
| asymmetric black | 0.992461 | −0.754% |
| asymmetric white | 0.982450 | −1.755% |
| Kiwipete black | 0.975375 | −2.463% |
| Kiwipete white | 0.982554 | −1.745% |
| start black | 1.002947 | +0.295% |
| start white | 0.998338 | −0.166% |

The overall paired median was **0.985360 (−1.464%)**. Only one of six fixture
medians improved and the worst was 0.975375. The candidate therefore missed all
three predeclared requirements: overall ≥1.01, at least four improving fixture
medians, and every fixture ≥0.99.

The supervisor completed in 121.33 seconds with return code zero, empty stderr,
240 valid process samples, peak sampled tree RSS 609,232 KiB, no timeout,
memory violation, orphan or survivor. The result JSON SHA-256 is
`2e56c55d5cf1dc81f7c996e5b04407e860cf2a3febe990f580c64d873627cfd4`;
the terminal receipt SHA-256 is
`0858e17d0b51486c7f3c7a93f02880e016f1b31d371e67d8c264f214c7f1ff75`.
Exact raw evidence remains in the WSL directory named above. Hopper PID
3099092 was alive before and after both profiling and the paired screen.

## Verdict

**SHELVE.** Removing the probe mutex did not buy useful whole-search throughput;
the sequence checks and deliberate busy-write misses outweighed the lock cost on
this workload. The source and test change were removed from both the main and
isolated working trees. No A/A or candidate games ran, there is no Elo evidence,
and this result must not be pooled with the two helper-policy experiments.

Do not repeat this per-stripe single-snapshot publication design or split out
the much smaller age-lock-only patch. A future TT lane needs a materially
different coherent-record protocol and new profile evidence of a larger
whole-search ceiling.
