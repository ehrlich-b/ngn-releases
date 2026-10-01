# WSL accepted-source baseline — 2026-09-05

Accepted playing source remains 53e4d1b9bf4006a39c0327a7016faba7f2b8f82e. Sol built and tested the clean detached documentation descendant fb008142cab267fdb4d91796580c64691da646eb in /home/ehrli/repos/ngn-b0-baseline. Root confirmed no Go/module source differences from the accepted source.

The frozen Linux binary is /home/ehrli/repos/ngn-next/output/nnue-smp-b0-20260905/baseline/ngn_fb00814_linux_amd64_v3, SHA256 5d834a0b3246ac73dacb892ef36e136cf5376dc1e44f659d6b60546e9b7f6a70. Build uses Go1.25.5, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly, CGO_ENABLED=0, GOOS=linux, GOARCH=amd64, GOAMD64=v3, -trimpath and -buildvcs=true. The actual complete command is retained in run-baseline.sh; the environment display abbreviates its absolute output path. Binary metadata omits VCS despite the requested flag, so the external clean HEAD/source-file hashes and binary hash supply that binding.

All required checks completed with exit0: short engine, short race engine, full short ./..., and full short race ./.... Their command, stdout/stderr log and explicit exit files are retained. The driver is terminal DONE_EXIT_0. Root independently inspected exits/logs and wrote root-review.json in the baseline directory.

Pre/post SHA256 inventories prove the accepted deployed engine and rollback binaries were not changed. This baseline is separate from historical native-Windows games and does not establish a new rating.

The mature-runner fixtures and prospective fixed200-game A/A remain separate gates. No A/A verdict is claimed here. Use the evidence in output/nnue-smp-b0-20260905 rather than inferring completed work from a RUNNING manifest.
