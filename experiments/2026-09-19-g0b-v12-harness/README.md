# G0b — Rodent V1.2 harness admission

## Verdict

**COMPLETE.** The narrow candidate-match preflight now admits
`rodent-v1.2-default` and proves the selected evaluator through the same exact
diagnostic contract already used for Counter 5.5 and Rodent V1.1 Anand. The
generic HCE/NGN-v1 manifest schema was not broadened. This is harness admission,
not a strength result; E0 games have not started.

## Change

- Added `rodent-v1.2-default` to the preflight backend allowlist and exact
  evaluator-diagnostic regular expression.
- Extended the focused fixture to require that V1.2 is advertised and that its
  selected-backend diagnostic is exact.
- Kept the negative selected-backend mismatch witness, now exercised with
  V1.2.
- Made the match-stage success fixture mirror production by giving its two
  roles distinct immutable launcher paths. The former shared fixture path could
  transiently produce two launcher-role matches when the monitor observed the
  parent before both children; production already uses a distinct launcher per
  role.

EvalFile-before-EvalBackend staging, acknowledgement barriers, OwnBook=false
search proof, failure checks, and the existing generic manifest contract are
unchanged.

## Validation

Authoritative validation ran in WSL from
`/home/ehrli/repos/ngn-sol-g0b-20260919` on source base `5964175` plus the three
ticket files:

```text
cd scripts/candidate-match
/usr/bin/python3 -m unittest discover -s tests -v
Ran 29 tests in 3.351s
OK
```

`python3 -m py_compile` also passed for all three changed Python files.

The first full-suite run before the fixture correction reproduced the inherited
launcher ambiguity in the nominal success witness. The correction was confined
to that test: two copied role launchers now match the production contract. The
complete suite then passed twice, including the final recorded run above.

## Real configured proof

The configured proof is retained at
`/home/ehrli/e0-v12-v11-preflight-20260919-001`. Both roles used the same
GOAMD64=v3 production executable, SHA-256
`dcde045792bba6b6d4f3fc83c9153763b94ff1ba1cba1d9df564a1ad69eb15aa`.

- Rodent V1.1 Anand selected `rodent-v1.1-anand` with model SHA-256
  `5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb`.
  Its receipt is `COMPLETE`, reports the exact backend diagnostic, and its
  OwnBook=false depth-two search completed with 154 nodes and best move `e2e3`.
- Rodent V1.2 selected `rodent-v1.2-default` with model SHA-256
  `c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053`.
  Its receipt is `COMPLETE`, reports the exact backend diagnostic, and its
  OwnBook=false depth-two search completed with 162 nodes and best move `d2d4`.

Both receipts record the exact option order `Threads`, `OwnBook`, `EvalFile`,
`EvalBackend`, `Hash`, `Move Overhead`; both advertise all five supported
backend values. Artifact hashes and structured results are in
[`result.json`](result.json).

## Next gate

Freeze a separate Rodent-specific E0 driver and manifest, review them before
scores exist, then run fresh Anand/Anand A/A100 followed by the capped
V1.2/Anand candidate400 gate. Passing this compatibility checkpoint does not
authorize changing the default evaluator or installing a binary.
