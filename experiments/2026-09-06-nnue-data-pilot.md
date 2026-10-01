# Verified T80 data pilot — September 6

The minimal real-data conversion pipeline is accepted. Sol ran the frozen reviewed wrapper exclusively on WSL. Root verified all3283 final listed artifact hashes, read the actual converter/verifier/mutation outputs and terminal supervisor receipts, and recorded output/nnue-public-preflight-20260906/root-data-pilot-v2-review.json.

From the pinned10,809,713,086-byte Stockfish T80 payload, the first qualifying complete-chain prefix decoded170,489 positions in3113 chains. Filters retained100,038 eligible positions before quarantine. Cross-split duplicate-input quarantine removed2 chains/4 eligible positions. Final retained100,034 positions (overshoot34):80,311 training,9518 validation,10,205 sealed test. This proves chain and model-input split isolation for the selected prefix, not original-game disjointness.

The independent verifier re-decoded the source and reconstructed canonical Bullet bytes from literal FEN, feature mapping and STM labels. It passed every decoded record and selected output. Paired mutations altered both the binary and sidecar then regenerated their receipts: score sign, piece color and king metadata all failed at independent semantic conversion, rather than merely failing a checksum.

The locked/offline source/vendor snapshot passed6 converter and2 verifier tests. Supervisor negative controls proved orphan, timeout/SIGKILL, monitor-failure, nonzero and panic handling with no survivors. Conversion took5.37s (sampled tree peak182,944KiB), verification6.68s (185,624KiB), and all three mutation controls22.50s (164,364KiB). These are bounded pipeline measurements, not engine-speed measurements.

Run: /home/ehrli/nnue-public-toolchain-20260906/runs/pilot-data-v2-attempt1. Final manifest SHA2567e1260458c02b01d36073f71822c5110d7c0b4d7e194f7caf207e01b75469768. All stages exited zero and recorded no surviving descendants. The sampled12GiB memory limit is not a cgroup hard cap.

Next is the small fixed-seed training/export/validation run, with validation-only checkpoint selection and a sealed final test. A100k-position net is a pipeline pilot; a separately reviewed larger corpus may be necessary for a competitive playing candidate. No real-data training or playing-strength verdict is established here.
