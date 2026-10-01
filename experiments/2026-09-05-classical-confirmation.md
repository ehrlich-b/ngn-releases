# Next classical candidate: baseline preflight and bounded game gate

Status: **completed and shelved at the inconclusive cap**. The
[one grouped fit](2026-09-05-classical-model-v2.md) passed its offline gates,
but `r0905texelv2` did not reach its required H1 in 1,600 games. No candidate
weights entered the playing defaults. The accepted qcap base is unchanged.

## Baseline A/A: r0905qbaseaa

Use the exact accepted Windows qsearch binary on both sides:
`ngn_20260904_qcap.exe`, SHA-256
`bc84fe0d298a32ff644775b2892ee43356bd94f9e22b6a527dc99cdbb29556f2`.
Runtime correction committed at `ec9d264`; later offline source/weights are
unchanged. Native Windows 9800X3D, default affinity, Go1.26.2 amd64/v3,
10+0.1, concurrency8, lowpower=false. Native workers are idle after deployment;
ongoing corpus generation/fitting is on the Mac only.

Frozen harness `sprt_20260904.exe` SHA-256
`63ef86d9b986fdb2c260782149af259e278845d89595d010df3970e6698cc9ed`;
5,000-opening book SHA-256
`974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`.

Fixed **200 games / 100 pairs**, no early stop or score-based extension.
Require zero operational failures/flags and a paired 95% interval containing
zero. Failure holds candidate games for investigation; no automatic replication.
This preflight validates the newly accepted baseline for its next candidate,
even if the grouped fit fails its offline gate and a later candidate is selected.

```text
sprt_20260904.exe -new .\ngn_20260904_qcap.exe -base .\ngn_20260904_qcap.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 200 -mingames 201 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

## Fitted-model gate, only after root review and filled artifact identities

If the fixed fit passes both offline comparisons, freeze its exact model JSON,
source diff and binary hashes before any candidate game. Independently verify
that source import/rebuild reproduces every exported weight. Baseline remains
the accepted frozen qsearch binary above. No extra search feature or T8 vector.

Candidate SPRT **[0,+10]**, alpha/beta .05, at least200 and at most1600 games,
same TC/concurrency/harness/book/adjudication as A/A. At observed throughput
this is about100 minutes at cap, plus13 minutes for A/A, within the 1–2 hour
confirmation budget. Uses at most800 opening pairs, all from the first1000
book lines reserved from the new training corpus. Do not extend on sign.

Accept only H1 and zero operational failures/flags. H0 or capped inconclusive
shelves this speculative fitted vector, retaining the full result. No default
or speculative-weight commit before that verdict. A passed self-play gate still
does not establish an absolute2800 rating; a fresh external pin must verify it.

## Frozen candidate r0905texelv2

The corpus and the single fit pass the predeclared offline gates. Root and an
independent reviewer checked the corpus identities, split/calibration/selection
flow, coefficients and reports. Integer validation MSE0.08457950→0.08406505;
final test0.07739660→0.07706922. This supports testing, not a strength claim.

Source: isolated `output/classical-v2-candidate-20260905`, parent `fd14d4d`,
with only fitted declarations in `engine/eval.go` and `engine/eval_pesto.go`.
Runtime patch `output/recovery-2026-09-04/classical-v2-candidate.patch` SHA-256
`05c5ab164fc8947ca79fd010e1a6f71a2fa9aaf469aeb9c05d6afac904d8d38b`.
No speculative coefficients enter main before a successful game verdict.

- Exact fitted model SHA-256:
  `6057944aea194e815a64985727a27785c46db03194c04f18fc93007884fe35a2`.
- Windows Go1.26.2 amd64/v3 `build/ngn_20260905_texelv2.exe`:
  `4feb83b08aacfa593d6d6cd68c7e3da61e20929a6364590fc491fb28c436b2c8`.
- Mac arm64 `build/ngn_20260905_texelv2`:
  `a98c654f425539b520d9aa0449b31875e60776a2ca74f9b3208322198516fc0b`.

All936 names/values/metadata equal the fitted export after actual source import
and compilation. Candidate EP/castling/fifty-move-mate UCI smoke passes. The first
short run flags an expected numerical snapshot change: one middlegame score
50→62 exceeds the old ±10 fixture tolerance. The test's existing explicit
baseline-generation command refreshed only this isolated candidate's snapshot;
all12 resulting scores were reviewed and the first failure log is retained.
No assertion/tolerance was weakened and no main fixture changed. This fixture
update validates repeatability under intentional new weights, not chess quality.
Full short/race/vet then pass; the independent SF oracle passes2293 positions,
48926 divides and128 internal search choices. Logs are `classical-v2-candidate-*`
under the recovery output directory.

Exact game command after a passing200-game baseline A/A and idle native box:

```text
sprt_20260904.exe -new .\ngn_20260905_texelv2.exe -base .\ngn_20260904_qcap.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 0 -elo1 10 -alpha 0.05 -beta 0.05 -maxgames 1600 -mingames 200 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

## Completed preflight and candidate launch

`r0905qbaseaa` completed200 games,51W/107D/42L, penta3/25/39/26/7,
paired+15.65 [−16.71,+48.28], zero errors/flags, `DONE_EXIT_0`. Its original
zero-in-interval control passes. Raw SHA-256
`d01bf1003dc73c6832c2dd9d08ecbd024825beaf37ff8d019fc8683287514661`;
audit `output/recovery-2026-09-04/r0905qbaseaa-audit.json`.

The controller checked completed preflight, idle workers and remote artifact
hashes, then launched the exact candidate command at01:15:09 local September5.
Root independently observed one expected versioned harness plus eight baseline
and eight candidate workers. The controller collects the completed result but
never deploys or commits weights. Root reviews H1/H0/capped verdict separately;
partial scores neither accept nor reject the vector.

## Completed result and root decision

`r0905texelv2` completed all1,600 games in1h42m9s: **480W/654D/466L**,
penta57/190/299/190/64 over800 pairs. Independently recomputed paired result:
**+3.040 Elo [−9.471,+15.559]**, pLLR−0.480748. Zero operational failures or
flag-outs;946 adjudicated wins,334 adjudicated draws,193 max-move draws and127
rule draws. Single final `DONE_EXIT_0`; the collector observed no remaining
native chess workers. This was a completed capped-inconclusive result, not H0.

Under the original rule, **shelve this fitted vector**. No extension, provisional
keep, refit on the holdout or default replacement. The improved static loss did
not establish improved play. This result does not prove that classical tuning
cannot work or that every historical failed tune had the same cause.

Root reconciled every consecutive game/WDL transition, reported counts, terminal
reasons, settings, penta score, paired interval and LLR with the completed log.
The audit rejects malformed or ambiguous completion and operational failures;
it passed the completed A/A control and rejected eight damaged-log probes.
Individual pairing identities/moves are not exposed by this SPRT log, so this
is not an independent move replay. The frozen tested harness supplies pairing.

Raw `output/recovery-2026-09-04/r0905texelv2_out.txt` SHA-256:
`0400348d8bdeaabed713d261f42e35a4364e8214933c5af28fb7db1351395959`.
Audit: `r0905texelv2-audit.json`; reproducer:

```sh
python3 output/recovery-2026-09-04/audit_frozen_sprt_20260905.py output/recovery-2026-09-04/r0905texelv2_out.txt ngn_20260905_texelv2.exe 0 10
```

The unchanged qcap base permits the queued T17+T8 confirmation to use its
already frozen artifacts and completed same-configuration baseline A/A.
