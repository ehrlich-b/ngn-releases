// Stockfish, a UCI chess playing engine derived from Glaurung 2.1.
// Copyright (C) 2004-2026 The Stockfish developers (see upstream AUTHORS).
// Modified Go implementation introduced in NGN on 2026-09-06; see NOTICE.

// Package sf18small decodes the exact Stockfish 18 SMALL NNUE file format.
//
// This package intentionally stops at immutable model loading. Feature
// extraction, evaluation, incremental contexts, engine selection, and SIMD
// layouts belong to later, separately validated stages.
package sf18small
