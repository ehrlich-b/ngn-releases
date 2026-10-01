# Release confirmation after the pretrained control

The completed Counter-versus-HCE match isolates the evaluator with identical accepted source, one thread per side, and a fixed 30+0.3 paired SPRT. It accepted H1 after 166 games / 83 complete pairs: Counter 89 wins, 37 losses and 40 draws, with the first upper-bound crossing at pair 83. Its opening set is the audited 200-line, six-ply development corpus. This establishes the pretrained evaluator's one-thread gain under that frozen test; neither it nor the accepted SMP gain is a fresh absolute rating.

## Decisions after the current test

- H1 was accepted with clean audits: keep the exact Counter network and adapter as the candidate. Confirm it with eight workers at a longer clock against HCE with eight workers, keeping code, total hash, CPU mask, and time controls matched. This tests the interaction between the two accepted features.
- H0 accepted: preserve the negative result and profile the exact candidate before changing arithmetic, score policy, or search. A new architecture or more training data is not the default response. The known upstream network separates learned-network quality from NGN integration and runtime cost.
- Cap without boundary crossing: report inconclusive. Preserve the fixed test; do not extend it opportunistically or turn a favorable interim score into acceptance.
- Operational failure: preserve the failure, identify its cause, and distinguish recovered game evidence from a clean completed prospective run.

The reviewed HCE2v1 and HCE4v1 tests follow sequentially after the Counter compute window. Their accepted manifests use the original SMP binary, so evaluator work does not confound width scaling.

## Independent opening confirmation

The later longer-clock test should use a second, fixed opening source selected before its results are observed. Prefer the official Stockfish books repository's historical `8moves_v3.pgn` for the first independent confirmation: it provides 34,700 lines of 16 plies, and its PGN format preserves the complete opening move history. Pin the repository commit, archive digest, extraction, deterministic selection seed and ordered identities. Deduplicate selected positions after legal replay, not only identical move strings. [Official book inventory](https://github.com/official-stockfish/books/blob/master/README.md)

This is a recommendation for testing generalization, not a claim that a balanced book necessarily gives more statistical power. Fishtest documents opening-selection bias and evaluates book efficiency with fixed-game time-odds controls; its guidance favors unbalanced books for efficient engine testing. A second book must not be selected because the candidate wins on it. [Fishtest FAQ](https://github.com/official-stockfish/fishtest/wiki/Fishtest-faq)

Stockfish's current progression comparisons use 60+0.6 at both one and eight threads and the UHO_Lichess_4852_v1 EPD book. That is useful precedent for separating thread counts and checking a longer clock, not an assurance that its ratings transfer to NGN. NGN's current auditor deliberately requires start-position PGNs; adding arbitrary-FEN openings would need history/repetition and terminal-audit support. Using an established PGN corpus first keeps that change bounded. [Stockfish progression-test conditions](https://official-stockfish.github.io/docs/stockfish-wiki/Regression-Tests.html)

Before any confirmation game: extend the runner's six-ply-only contract deliberately to the selected fixed opening depth, retain exact Round/color/EPD association, audit every selected opening legally, and test negative prefix/depth cases. Freeze the complete prospective contract. Proposed confirmation clock is 60+0.6, one game at a time, equal eight-worker physical-core masks and 128 MiB total hash per engine, no adjudication, the same normalized 0/20 hypotheses and 400-game cap. This is a plan, not an approved or launched match.

## Opponents and deployment

Counter 5.5 remains the first external calibration because its in-process evaluator is now a controlled reference, with a fresh standalone handshake under the actual CPU mask and matched threads/hash/clock. Rodent V 1.1 Anand/testers remains the strongest publicly rated anchor in the September 5 CCRL Blitz list; CCRL does not identify the personality behind its row. Rodent V 1.2 is the current stable release and its non-Tal testers artifact is the current full-strength target inferred from the official release notes. Keep both exact identities: V1.1 for the published rating anchor, V1.2 for the latest-release comparison. [Pinned Rodent preflight and caveats](2026-09-06-rodent-v1.1-release-confirmation.md)

Zahak 10, Blunder 8.5.5 and Chess-3 v4.0 remain provisioned opponents, with no completed external games yet. Each Rodent version needs its own sequential UCI/network/thread/book admission under the match CPU mask; the corrected V1.2 asset replaced an initial testers build with multithreading disabled. A latest-release refresh and provenance check precedes any world-ranking claim.

Publish per-opponent completed results and uncertainty under the actual WSL conditions. Do not add internal gain estimates to the historical native-Windows 2884.14 estimate. Once the required isolated, combined and external checks pass, build the final integrated source, verify its protocol and stop behavior on WSL, archive deployment and rollback artifacts, then replace the preserved 53e4d1b release under the user's existing authorization.

Research checked 2026-09-06. This plan changes no active run, source arithmetic, or deployment.
