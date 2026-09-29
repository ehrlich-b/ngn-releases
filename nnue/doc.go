// Package nnue implements the frozen NGN v1 neural-network format and its
// portable scalar reference evaluator.
//
// NGN v1 is deliberately small: a shared Chess768-to-128 feature transformer
// is evaluated from both colours' perspectives, the side-to-move accumulator
// is ordered first, and a squared clipped ReLU feeds one linear output. This
// package owns only raw side-to-move scores. Search-side clock attenuation and
// score clamping belong to the later engine adapter.
package nnue
