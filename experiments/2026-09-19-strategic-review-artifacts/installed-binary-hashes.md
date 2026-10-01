# Recorded installation paths still contain the classical release

On September 19, 2026, the review read SHA-256 hashes at the three exact
installation paths recorded in the [September 5 deployment](../2026-09-04-deployment.md)
and [September 12 release audit](../2026-09-12-release-state-audit.md).
The successful remote command took **0.414 seconds**. It did not launch an
engine, build, match, or hopper operation, and it changed no files.

```sh
ssh ehrli@192.168.4.108 'wsl -d Ubuntu -- sha256sum /home/ehrli/repos/ngn/build/ngn /mnt/c/Users/ehrli/ngn/ngn.exe /mnt/c/Users/ehrli/ngn/repin/ngn.exe'
```

Exact result:

```text
0c067627404ab67a5f4a57fef63be024fd84e9248423286ef3dfc103041d266e  /home/ehrli/repos/ngn/build/ngn
2c29f19652ce0bfa2d068af093e83c4eeca4f5f1907bc5dba7475162439a3c33  /mnt/c/Users/ehrli/ngn/ngn.exe
2c29f19652ce0bfa2d068af093e83c4eeca4f5f1907bc5dba7475162439a3c33  /mnt/c/Users/ehrli/ngn/repin/ngn.exe
```

All three exactly match the recorded classical `53e4d1b` release, whose
historical absolute test was 2884 [2834, 2934] under its own conditions. They do
not contain the much later optimized Rodent configuration used by the accepted
external Counter checkpoint. This is a current artifact-identity observation,
not a newly measured rating or evidence of which executable the user selected
in a GUI. Alternate launchers and GUI configuration were not inspected.

The source default and generic build-policy issues are separate:
[configuration addendum](evaluator-quality-screen-contract.md). No promotion or
default change was made. Existing release conditions remain in force; using
this receipt to bypass them would misinterpret a diagnostic as authorization.
