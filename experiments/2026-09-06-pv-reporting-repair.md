# PV reporting correction

Accepted for integration as a proved reporting correction; no new strength claim. Sol implemented the actual-history PV repair and independent Sol review added terminal-root coverage. Root reviewed the full source and integrated it with the existing NNUE/SMP foundations.

ExtractPVFromTT snapshots actual occurrence counts under the position mutex, adds hypothetical occurrences privately, and retains the move that creates mate, stalemate, repetition, fifty-move draw or existing insufficient material before stopping. Already-terminal roots return no PV. Deferred undo restores the supplied position and never mutates played history. TT layout, hash identity, replacement and search policy are unchanged.

The preserved failed-control witness has43 actual plies and root occurrence2. Its old displayed line d8d7 d5b6 d7d8 b6d5 d8d7 crossed threefold at ply4; the corrected line contains the first4 legal moves. Sparse/nil histories, terminal first moves and roots, illegal/missing/bound TT entries, length limits and complete board/history restoration are covered.

Corrected standalone B0 is b134e21a569fd12fd5fc767b48c9d8e0de5e14b0, exact parent fb00814, in /home/ehrli/repos/ngn-b0-pv-corrected. Binary SHA2562eb616ea1378dddf4c8358b79cc0873bb3306955ef2d8e5e80f1f949bb132ec2. Its UCI option list is byte-identical to original B0.

The old accepted-v2 strict comparison correctly fails on exactly8 PV arrays in the named threefold-root-history fixture: they become empty. A separate reviewed checker permits only those changes and requested source metadata; all moves/scores/nodes/diagnostics/state and other fixtures match. The original failure and reference remain preserved. Corrected reference SHA25613151fdc0eb1dace72e9619c749fa12a527da517a48752f03b6dd8a869aa6c89 is the reference for combined integration.

Root integration passes focused tests, full short, full short race, vet, build, and exact comparison with corrected B0. Commands/logs/exits and copied reference are in output/pv-reporting-integration-v2-20260906 with DONE_EXIT_0. Isolated source/evidence are in ngn-b0-pv-corrected/output/pv-reporting-v2-20260905.

The internal pseudo-legal en-passant hash identity issue remains separate. The failed original A/A is not resumed or pooled. A fresh strict two-game diagnostic and separately frozen200-game control are prepared; accepted deployment remains at53e4d1b with rollback intact.

Fresh strict diagnostic completed:2games/484 independently legal plies, both final checkmates confirmed, fastchess/auditor exit0, empty stderr and terminal worker receipt, DONE_EXIT_0. Run: output/nnue-smp-b0-pv-corrected-20260905/diagnostic-2-tc30. The200game control remains held until other computation is terminal and final inputs are verified.
