# Move Overhead option audit — September 6

Source inspection of the exact corrected B0 source b134e21 confirms a pre-existing no-op UCI option:

- engine/uci.go:266 advertises Move Overhead, default 100 ms, range 0..5000.
- handleSetOption has no Move Overhead case. Its generic search-tunable registry does not contain that name.
- engine/time.go:89 initializes the private TimeManager field to 50 ms. The only production references consume this value; no production setter exists.
- ucinewgame constructs a new TimeManager, which would also reset a future setter unless persistence is implemented.

The corrected B0 A/A manifest accurately records the value sent to both engines as 100 ms. Its effective overhead is the compiled 50 ms because the command is ignored. Both sides use the identical binary and commands, so the symmetric runner control is unaffected. Preserve the frozen manifest, logs, and binaries; attach this source finding to the control verdict rather than rewriting its inputs.

The active M3/N3 ownership candidates preserve this behavior. A separate P1 UCI policy correction is proposed: honor the advertised 100 ms default, validate/apply the 0..5000 ms setter while idle, and preserve configured overhead through ucinewgame. Standalone NewTimeManager's current 50 ms API default can remain unchanged. Validate consecutive searches, explicit option updates, invalid input, and new-game persistence. Do not hide the changed time policy inside NNUE inference or ownership parity claims.

For later engine-strength comparisons, both HCE and NNUE must use the same implemented overhead policy and effective value. Any comparison directly against corrected B0 must identify the old ignored-option behavior and account for it in the prospective test design.
