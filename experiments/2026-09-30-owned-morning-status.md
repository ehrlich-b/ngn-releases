# NGN morning assessment — September 30, 2026

The bounded morning work is finished. WDL25-e10 at K4EvalScale 60 remains
the selected owned evaluator. The released engine now has two audited external
wins and a proved two-core improvement. Four-core strength remains unresolved.
The public CCRL request is visible; no reply or official rating is available.

## Completed evidence

| Comparison | Fixed games | Clock / hash | Relative Elo, paired 95% interval |
| --- | ---: | --- | --- |
| Released NGN / native Counter 5.5, one CPU | 400 | 60+0.6 / 128 MiB | +167.31 [144.10,192.01] |
| Released NGN / pinned-source Maelstrom 3.3.0, one CPU | 400 | 10+0.1 / 128 MiB | +250.49 [223.69,280.11] |
| NGN two cores / one core | 800 | 10+0.1 / 256 MiB | +78.16 [64.57,91.52] |

The [new Maelstrom result](2026-09-30-owned-maelstrom-result.json) is
261 wins, 125 draws and 14 losses over 200 reversed-color pairs. Fresh
procedure and 128-game A/A controls passed before the benchmark. All 58,590
final legal plies, operational/process audits, independent WSL recounts and
182 off-worker record hashes verify. The exact-artifact draw-PV compatibility
checker has independent positive and negative controls; other warnings remain
fatal. No earlier rejected or stopped game was pooled.

The fresh four-core A/A passed over 18,920 plies. Its fixed 800-game comparison
against accepted width two [stopped unscored](2026-09-30-owned-smp-morning-capacity-failure.json)
at 08:31 UTC on the physical Windows capacity guard. Clean termination has
no surviving processes. All 122 copied record hashes and both complete
lossless traces verify. Retain Threads=2; the released default remains Threads=1.
The multicore result uses a separate hash configuration and does not add to
the external-anchor Elo measurements.

## A more useful goal

The next ranking goal should be **independently establish a top-three position
among Go engines on a specified CCRL list**, using one latest version per
engine and comparable thread counts. Independent tester admission and a
rating cannot be promised by a morning deadline.

The controllable morning milestone was a completed fixed 400-game comparison
against a second leading Go opponent, audited records, and a usable validated
thread setting. The external comparison is complete, and width two remains
the accepted setting. A new stronger network was not promoted overnight.

Among the identified Go families on the September 28 CCRL 40/15 list, using
each family's best listed single-CPU version, Donna 4.0 is tenth at 2594.
The identified families above it are Counter, Maelstrom, Zahak, Combusken,
Chess-3, Zurichess, Aconcagua, Mess and Blunder. This is a filtered list,
not an exhaustive world ranking. Sources: [CCRL 40/15](https://computerchess.org.uk/4040/rating_list_all.html)
and [Donna's Go source](https://github.com/michaeldv/donna).
The completed local wins support pursuing the stronger target; they do not
establish an official rating or world rank.

## Submission and host

The approved [CCRL testing request](https://kirill-kryukov.com/chess/discussion-board/viewtopic.php?t=19325)
is public. A fresh guest check at 11:16 UTC found one post and zero replies.
It discloses AI tools used in engine development, testing and training tooling.
No new external reply was sent.

All chess jobs are stopped. [Approved VHDX compaction and restoration](2026-09-30-owned-morning-storage-restoration.json)
succeeded; Docker, both hopper workers, the relay and keepalive are restored.
Fresh physical Windows free space is 24.448 GiB, below the unchanged 25 GiB
admission floor. New matches and training remain held until capacity is
available. All engine execution and validation used WSL; no new GPU training
run was justified or launched. Finished evidence is preserved off-worker.
