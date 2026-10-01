# Assessment of the owned K4 NNUE results

**Bottom line:** Don't start a longer run yet. The -272 Elo gap to Rodent hasn't been attributed to any cause. Your offline metrics also failed to predict play across networks: owned K4 beats Rodent on overall MAE and still loses by 272. And the original run's early checkpoint selection suggests that more training passes over the same 20M positions is the least likely lever. Spend 1–2 days on the Windows/WSL box separating the causes first.

Policy note: the project CLAUDE.md records a September 13 direction deferring owned NNUE training and data generation. This branch suggests that has changed. Please confirm before committing box hours to Experiments 2 and 3 below.

## 1. What the results establish

**Established:**
- The owned pipeline produces a working network that is much stronger than NGN's hand-coded evaluator inside NGN at 30+0.3: +250 [217,286].
- With NGN's search held fixed at 10+0.1, Rodent V1.1 is far stronger than owned K4: -272 [-319,-230]. This compares network plus integration fit, not networks in isolation.
- In real games, owned K4 searches about 20% fewer nodes per second and is one ply shallower at the median.
- Owned K4 is calibrated to the WDL objective to within about 4%. The best multiplier is 0.96, and it barely helps.
- Annealing on the same data (the short cosine run) buys about 2% validation MSE.

**Not established:**
- Any absolute rating, for either network.
- Why owned K4 loses. Throughput, eval resolution, data volume or distribution, labels, architecture, and fit to search margins are all still open.
- Whether the short-cosine checkpoint is better or worse in play.
- That the losses come from misjudged near-equal positions.

**Corrections to weak inferences:**
- **Owned's better overall MAE (130 vs 133) is not evidence of a better eval.** Rodent wasn't trained on Stockfish 18's scale, and overall MAE is dominated by lopsided positions whose exact centipawn value barely affects play. The -272 result shows there is currently no validated offline metric for promoting one network over another.
- **The central-band slice is selected on the target.** Any predictor with residual noise looks bad in that band, and shrinking toward zero wins; that's why the zero predictor beats both networks. The informative part is the comparison: owned's spread near equality is about 1.8× Rodent's. Add bins conditioned on the prediction (when owned says +50, what does the teacher say?).
- **Offline metrics can't tell you how rescaling affects play.** A global multiplier keeps every static eval in the same order. It changes play only through NGN's centipawn-based search thresholds: reverse futility, futility, null-move and SEE margins, aspiration windows, draw scaling. Only games can answer it. The 0.96 WDL optimum also argues against "wrong scale". The central problem looks like resolution, not scale.
- **The one-ply median depth gap is not purely a speed effect.** At an effective branching factor of about 2, 20% fewer nodes costs only about 0.3 ply. The eval shapes the tree through move ordering and pruning, so a worse eval can also cost depth. NPS itself also depends on tree shape (qsearch share, accumulator refreshes), not only on inference cost. Medians of whole-number depths are coarse, so treat this as a hint. Rough bound: 0.804 speed is about 0.31 doublings, or roughly 30–40 Elo at bullet time controls. That's an estimate, not a measurement, and about 10–15% of the gap.
- **The 32-position panel (21 → 14) can neither reject nor validate the checkpoint.** Only the positions whose result changed count (McNemar test), and at n=32 that isn't enough. Stop using the panel as a gate.
- **The two matches can't be chained.** +250 at 30+0.3 and -272 at 10+0.1 used different time controls and opponents, so they give neither a Rodent-vs-hand-coded figure nor an absolute rating.
- **The holdout may be leaky.** If the 20M positions come from games and the 100k holdout was split by position rather than by game, the holdout metrics are optimistic. Check this.

## 2. Likely explanations, cheapest discriminating test first

1. **Integration or perspective bug.** Candidates: trainer and engine mapping features differently, black-perspective flip vs mirror, or a quantization mismatch. There's no specific evidence for this and the audits passed. It still has to be ruled out first, because it would look exactly like poor resolution.
   - *Test:* bit-exact engine vs trainer integer eval on the 100k holdout; color-flip symmetry; MAE split by side to move.
2. **Throughput.**
   - *For:* 0.804 NPS ratio and one ply less depth.
   - *Against:* the rough bound above is about 30–40 Elo.
   - *Test:* a fixed-node match against Rodent. The real-clock gap minus the fixed-node gap is the share due to throughput.
3. **Fit to NGN's search margins.**
   - *For:* the margins are in centipawns and were tuned under a different evaluator.
   - *Against:* owned K4 is WDL-calibrated.
   - *Test:* play owned ×0.82 and ×1.25 against owned ×1.0.
4. **Eval resolution caused by data or labels.** Data means volume or distribution. Label problems include depth noise and non-quiet positions labeled with tactics a static network can't see.
   - *For:* owned's central spread is 1.8× Rodent's. The checkpoint was chosen at 16% of the schedule; if validation loss rose after that, it points to overfitting on too little data.
   - *Offline tests:*
     - Split the central-band error into quiet vs non-quiet positions.
     - Measure label self-noise by rescoring 5k positions at 4× the nodes.
     - Check the train/validation curve.
   - *Then:* the 2×2 below.
5. **Architecture or capacity of K4.** There's no evidence either way. The 2×2 tests it.

**The 2×2.** Use the same 20M positions and the same short-cosine recipe (about 32k updates each):

| | Stockfish 18 labels | Rodent eval labels (distillation) |
|---|---|---|
| **K4** | have it (short-cosine checkpoint) | diagnostic only, never promotable |
| **Rodent architecture** | train | train |

How to read it:
- K4 trained on Rodent labels comes close to Rodent → the architecture is fine; the labels or target are the problem.
- Rodent architecture trained on Stockfish 18 labels comes close to Rodent → K4 is the problem.
- Rodent architecture trained on Rodent labels still falls well short of Rodent → your positions, volume or training recipe are the bottleneck, whatever the labels. Without Rodent's own training data, this is the cleanest data test available.

## 3. Start a much longer training run now? No.

- **Same data:** the original checkpoint was picked at 16% of the schedule, and annealing bought only 2%. More passes over 20M positions is the weakest lever.
- **No validated offline metric:** you'd need games to judge the output of a long run anyway.
- **Gap not attributed:** if the cause is architecture or labels, a long run on the current setup locks it in.

Start one only when all of these hold:
- The 2×2 places the gap in the data or its volume.
- A learning curve over 5M, 10M and 20M positions (same recipe) shows the last doubling of data still worth at least about +30 Elo at fixed nodes.
- A new data source beats the old one at equal size.

Then scale up the number of positions (at least 5–10×), not the number of passes, with the schedule length matched to the planned passes.

## 4. Would a Stockfish game corpus help?

- **Game results as labels are weak.** One result is shared across 100+ correlated positions, and Stockfish-vs-Stockfish games are overwhelmingly draws. That carries almost no information about small differences near equality, which is exactly your deficit. Results are not a substitute for scored positions.
- **Eval comments in the PGNs, if present:** they mix engine versions, depths and time controls. Use them only as weak secondary labels, and only after measuring their agreement with Stockfish 18 fixed-node rescoring on a sample.
- **The real value is as a source of positions.**
  - *Upside:* realistic openings and middlegames, and cheap diversity.
  - *Downside:* search mostly evaluates positions that strong games never reach, such as positions after bad moves and qsearch leaves. A network trained only on well-played positions can misjudge the positions it needs to refute.
  - *Mix:* corpus positions; corpus positions plus 1–2 random legal moves; and leaves sampled from NGN's own searches. Score all of them with Stockfish 18 at fixed nodes.
- **Hygiene:**
  - Deduplicate and cap the positions taken from each game.
  - Filter out in-check and non-quiet positions, or label the position qsearch resolves to.
  - Split the holdout by game.
- **Balanced sampling:** balance on phase, material and source, not on teacher score. Balancing on the target shifts the network's average prediction and biases it toward or away from zero. If you want more weight on near-equal positions, use loss weights and report both the target-binned and prediction-binned slices.
- **A move-quality loss: not yet.**
  - Add one only if Experiment 1 shows owned K4's deficit is in ranking sibling moves while its calibration holds.
  - If so, use an auxiliary pairwise loss over sibling positions, labeled by Stockfish 18 MultiPV, and keep the value loss primary.
  - Search compares values across subtrees and relies on centipawn margins, so a ranking-only objective can break calibration.
  - It also costs several times more labeling per root position.

## 5. Next experiments and gates

**Experiment 1: attribution, no training (about 1–2 days on the box)**
- **a. Parity and symmetry checks** (item 1 above). Gate: exact match. Any mismatch halts everything else.
- **b. Fixed-node match, owned vs Rodent.**
  - Same 100 paired openings, 200 games.
  - Both sides get the same node budget, roughly Rodent's median nodes per move from the audited PGNs.
  - Predeclared rule: if the fixed-node gap is within about 50 Elo of -272, throughput is minor, and inference speed work waits.
- **c. Scale test.**
  - Owned ×0.82 and owned ×1.25, each against owned ×1.0, 200 games each at 10+0.1.
  - Predeclared rule: if neither moves more than about ±40 Elo, scale fit is not material to the gap.
- **d. Offline decision metrics, owned vs Rodent:**
  - calibration binned by prediction;
  - central-band error split into quiet and non-quiet positions;
  - label self-noise;
  - sibling ranking on about 2k quiet positions (Kendall tau and centipawn regret of the top child, against Stockfish 18 MultiPV).

  If sibling ranking separates the two networks in the direction of the -272, it becomes a pre-filter that can reject candidates but never promote them.
- **e. Loss autopsy on the 140 losses.** Find the first move where Stockfish 18's eval swings more than 100cp against owned, classify it as quiet or tactical, and record owned's static eval there.

**Experiment 2: the 2×2 above.** Write down the reading rules before training. Gate for each cell: a 200-game fixed-node match against Rodent.

**Experiment 3, only if Experiment 2 points at data:** the learning curve plus a comparison of new-source vs old-source 20M sets at equal size. Gate: fixed-node games, then real-clock games, against the current owned network.

**Promotion ladder for any candidate network:**
- **P0:** parity and symmetry are exact.
- **P1:** holdout WDL MSE and the Experiment 1 decision metric are no worse than the current network beyond bootstrap noise. This step can only reject.
- **P2:** a 200-game fixed-node paired match against the current owned network, with the lower 95% bound above 0.
- **P3:** 400 paired real-clock games at 10+0.1 and at 30+0.3 against the current owned network, with a predeclared rule.
- **The gap to Rodent-in-NGN is a benchmark to track, not a gate.** The short-cosine checkpoint goes through P2 and P3 like any other candidate. It's low priority, but useful for checking whether a 2% MSE gain turns into measurable Elo.

**Absolute rating:** run an anchored gauntlet against several engine families on Rodent-in-NGN now. If NGN with a Rodent-level eval falls well short of 3000, matching Rodent is necessary but not enough, and the search is also a limit. That changes how much to invest in the network versus the search.
