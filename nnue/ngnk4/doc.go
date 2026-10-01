// Source lineage: Rodent V by Naman Thanki and Pawel Koziol.
// NGN adapted the feature design and internally reused its Rodent-backend
// kernels; the explicit 2026-09-27 port is recorded in NOTICE.

// Package ngnk4 implements the versioned NGN-owned king-bucketed NNUE
// contract. It is deliberately independent of the strict Rodent V1.2 loader:
// the two evaluators share an architecture shape, but not model identity,
// score scaling, accepted files, or overflow semantics.
package ngnk4
