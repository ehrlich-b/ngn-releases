# Owned K4 NNUE pilot selection and strict parity

The frozen one-million-position pilot completed 1,024 updates and checkpoint
selection used only the finalized validation BF.  The selection command had no
calibration or reserved-test input.  The finalized manifest SHA-256 was
`753e8ed1fbe09bef290fcba7209dfb77dce0170e80a5a09a22a1c3c12599755e` and
the validation BF SHA-256 was
`4aacc29dc10e4fcf4bb31a06a91442b116cce64e9f24229ff0f0865895c0960c`.

The no-clobber selection receipt SHA-256 was
`5ac6a2aa689a9c2dfe1d60ef12f8b7a008c680820dee83c701e5f4d631369364`.
All eight trained checkpoints were eligible: their raw/quantized files and all
three optimizer-state files passed independent receipt, SHA-256, layout and
finite-value validation; every output head passed the fast-arithmetic check;
and no output tensor value was saturated.  Early stopping did not fire.
Checkpoint 1,024 won by the predeclared lowest-integer-validation-MSE rule:

| checkpoint | integer MSE | raw MSE | quantization mean/p99/max (cp) |
| ---: | ---: | ---: | ---: |
| 0 (diagnostic) | 0.0269574337493 | 0.0269602457952 | 0.886 / 3.003 / 6.325 |
| 1,024 (selected) | 0.0113931967397 | 0.0113791865485 | 5.404 / 19.786 / 35.656 |

The selected checkpoint receipt SHA-256 was
`e9bd9a126acebb26eca84336327f8e9757de494909649de5b57c61498df35a17`.

Strict conversion first exposed a stale bridge contract: the pinned Bullet
trainer's probe pass line includes its patch SHA, while the old Go reader
expected only the upstream commit.  That failed attempt remains under
`selected-candidate-v1`.  The reader was tightened to require both identities
and the fresh no-clobber v2 run passed:

- Bullet commit: `629ee50000b2afb7b3337595401c830d3b1e0f42`
- Bullet patch SHA-256: `f7f5e0dd02695fe57ec58a630b63006da96cb1ec6b8fd4769a4e6308a8121e54`
- strict model SHA-256: `034559653a83a7e64d4407badff334f66e3eae6a3a1f33147e3516b3c88c3e69`
- tensor payload SHA-256: `68986f0fd6679dc2da3ab734f8ce24115813c8561f7f648176ca8160b37a323d`
- conversion receipt SHA-256: `6c0a5c6030101abffcd7d664bb250c479eb76cfb83442090a9309a79072c9f57`
- Bullet probe SHA-256: `ed2c49c50c35db9cea65a4159d5892f91f0d0901de5cc5cd5340bc766e2a5b45`
- parity receipt SHA-256: `fcf669fcbb025d2b5e57c5185c6a1339a43c98e92dddb517d804f4f51ce0873b`
- maximum Bullet/independent-Go float delta: `2.98e-7` z
- strict-parity quantization error: mean `4.568` cp, p99/max `13.118` cp
- all 12 parity fixtures: pass

Focused `ngnk4bridge` and `ngnk4` tests and vet passed.  The repository-wide
test run reached these packages cleanly but failed separately in the existing
engine stress suite: `TestConcurrencyStress` reported empty move lists and
`TestNumericalLimits/Depth_100` hit the suite's ten-minute timeout.  Those
failures were not changed or waived by this result.
