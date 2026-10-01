// Stockfish, a UCI chess playing engine derived from Glaurung 2.1.
// Copyright (C) 2004-2026 The Stockfish developers (see upstream AUTHORS).
// Modified Go implementation introduced in NGN on 2026-09-06; see NOTICE.

// Package sf18big decodes the exact Stockfish 18 BIG NNUE file format.
//
// It implements immutable model loading only. Feature extraction, evaluation,
// incremental contexts, dual-network selection, engine adapters and optimized
// layouts belong to later, separately validated stages.
package sf18big
