# NGN 0.4.0

NGN now embeds an NGNN3 H768 network with mirrored king-context inputs and
cached accumulator refreshes, replacing 0.3.0's H512 NGNN2 network.
Search and time management incorporate the accepted changes since 0.3.0.
Quiet promotions are considered at the quiescence horizon, and legal-move
queries preserve the caller's position and evaluation state.
The engine reports build identity through `--build-info` and exposes current
search tuning parameters through UCI, including W2ScoreTime.
The public source includes the trainer, data-method documentation, license
history and updated notices. No sidecar network is required.
The README retains the existing strength record; this release adds no new
independent rating claim.
