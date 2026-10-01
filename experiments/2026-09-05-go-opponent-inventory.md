# Public Go opponent release inventory — 2026-09-05

Purpose: freeze identified release candidates for the active NNUE/SMP goal's later matched-condition comparison. This is an inventory, not a rating or runtime-validation result. Metadata and downloads were obtained on WSL; no opponent was executed.

| Engine | Public release tag | Source revision | Linux preparation |
|---|---|---|---|
| [Counter 5.5](https://github.com/ChizhovVadim/CounterGo/releases/tag/v1.55.0) | v1.55.0 | 63c487ca724c620f71c129d62129c6fb9109c872 | Official Linux artifact frozen |
| [Zahak 10.0](https://github.com/amanjpro/zahak/releases/tag/10.0) | 10.0 | 72ff8066e2d8348d8d3847fab62c4a6b8344240d | Official Linux artifact frozen |
| [Chess-3 4.0](https://github.com/paulsonkoly/chess-3/releases/tag/v4.0) | v4.0 | a33531629cbe82eef6810f982ec814e79d52a3b3 | Build exact release source on WSL |
| [Blunder 8.5.5](https://github.com/deanmchris/blunder/releases/tag/v8.5.5) | v8.5.5 | 89230a74a966b3400610271eca49c4be241e94cd | Official archive frozen; select Linux member |
| [Rodent V1.1](https://github.com/nescitus/Rodent-V/releases/tag/Rodent_v_1_1) | Rodent_v_1_1 | 5689d0babebe95d87592eaaaee73ea555ef9345c | Anand/testers Linux artifact frozen as publicly rated anchor |
| [Rodent V1.2](https://github.com/nescitus/Rodent-V/releases/tag/Rodent_v_1.2) | Rodent_v_1.2 | b53ffaf670590932957cb63b7b6d871f6f33b7d8 | Non-Tal testers Linux artifact frozen as latest stable target |

Counter's public 5.5 release uses the unusual tag v1.55.0; do not substitute the newer development default branch. Zahak's 10.0 release resolves to the same source revision already inspected in research. Blunder's former algerbrex repository redirects to deanmchris. Chess-3 publishes a Windows asset only, so its Linux comparator requires a source build.

Artifact identities are recorded in /home/ehrli/repos/ngn/output/next-stage-20260905/opponents/manifest.json. Full release metadata is in the adjacent go-release-inventory.json in the parent directory. Downloads were checked against advertised asset sizes and assigned full SHA256 digests; these are frozen observed artifacts, not a claim of independently signed provenance.

Before matches: inspect archive members before extraction, validate network loading/UCI options and legal output, record effective Threads/Hash/ponder settings, and freeze runtime artifacts. Build or engine execution must use the coordinator's WSL CPU window. Keep total hash and thread comparisons explicit; do not silently compare a single-thread-only opponent as though it supported eight workers.

Rodent V1.1 and V1.2 have separate roles: V1.1 is the September 5 publicly rated anchor, while V1.2 is the current stable target. The intended pool is the strongest identified public Go release set plus diverse stronger reference engines. Recheck for omitted stronger public Go releases before any leadership claim. No published rating from different lists is added to NGN's historical result, and no completed head-to-head result exists for these new WSL artifacts yet.
