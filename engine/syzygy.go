package engine

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"sync"
)

// Syzygy tablebase support for perfect endgame play
// Supports WDL (Win/Draw/Loss) and DTZ (Distance To Zero) tables

// WDL scores
const (
	WDL_Loss        = -2
	WDL_BlessedLoss = -1 // Loss but 50-move rule draw
	WDL_Draw        = 0
	WDL_CursedWin   = 1 // Win but 50-move rule draw
	WDL_Win         = 2
)

// Tablebase probe results
type TBResult struct {
	WDL   int // Win/Draw/Loss result
	DTZ   int // Distance to zeroing move (capture or pawn move)
	Found bool
}

// SyzygyTB handles tablebase probing
type SyzygyTB struct {
	path      string
	maxPieces int
	loaded    bool
	mu        sync.RWMutex

	// Cache for loaded tables
	wdlCache map[string]*wdlTable
	dtzCache map[string]*dtzTable
}

type wdlTable struct {
	data []byte
}

type dtzTable struct {
	data []byte
}

var globalTB *SyzygyTB

// InitSyzygy initializes the tablebase with the given path
func InitSyzygy(path string) error {
	if path == "" {
		return nil
	}

	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("syzygy path not found: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("syzygy path is not a directory: %s", path)
	}

	globalTB = &SyzygyTB{
		path:      path,
		maxPieces: detectMaxPieces(path),
		loaded:    true,
		wdlCache:  make(map[string]*wdlTable),
		dtzCache:  make(map[string]*dtzTable),
	}

	return nil
}

// detectMaxPieces scans the directory to find the largest tablebase
func detectMaxPieces(path string) int {
	maxPieces := 0

	// Check for 3-piece through 7-piece tables
	for pieces := 3; pieces <= 7; pieces++ {
		// Look for any WDL file with this many pieces
		pattern := filepath.Join(path, fmt.Sprintf("*%d*.rtbw", pieces))
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			maxPieces = pieces
		}
	}

	// Also check standard naming patterns like KQvKR.rtbw
	files, err := os.ReadDir(path)
	if err != nil {
		return maxPieces
	}

	for _, f := range files {
		name := f.Name()
		if len(name) > 5 && name[len(name)-5:] == ".rtbw" {
			// Count pieces in filename (K's and pieces between K's)
			pieces := countPiecesInFilename(name[:len(name)-5])
			if pieces > maxPieces {
				maxPieces = pieces
			}
		}
	}

	return maxPieces
}

// countPiecesInFilename counts pieces in a tablebase filename like "KQvKR"
func countPiecesInFilename(name string) int {
	count := 0
	for _, c := range name {
		switch c {
		case 'K', 'Q', 'R', 'B', 'N', 'P':
			count++
		}
	}
	return count
}

// ProbeWDL probes the tablebase for Win/Draw/Loss result
// syzygyProbeTrusted gates the WDL/DTZ probe. It is FALSE because the current decoder
// (calculateTBIndex returns pos.Hash(); probeWDLTable reads data[hash%len]) is a
// PLACEHOLDER, not a real Syzygy index scheme — its WDL/DTZ values are arbitrary bytes.
// Until a real decoder lands, hard-gate the probe to a guaranteed miss so a stray
// `setoption name SyzygyPath` can never drive play with random "tablebase" moves — a
// dormant footgun in the engine's weakest phase. See task #15 / the phone-a-friend audit.
var syzygyProbeTrusted = false

func ProbeWDL(pos *Position) TBResult {
	if !syzygyProbeTrusted {
		return TBResult{Found: false}
	}
	if globalTB == nil || !globalTB.loaded {
		return TBResult{Found: false}
	}

	// Count pieces
	pieceCount := countPieces(pos)
	if pieceCount > globalTB.maxPieces {
		return TBResult{Found: false}
	}

	// Don't probe with castling rights (tablebases assume no castling)
	if pos.HasCastling() {
		return TBResult{Found: false}
	}

	// Generate the material key for this position
	materialKey := getMaterialKey(pos)

	// Try to probe the WDL table
	globalTB.mu.RLock()
	table, exists := globalTB.wdlCache[materialKey]
	globalTB.mu.RUnlock()

	if !exists {
		// Try to load the table
		table = loadWDLTable(globalTB.path, materialKey)
		if table == nil {
			return TBResult{Found: false}
		}

		globalTB.mu.Lock()
		globalTB.wdlCache[materialKey] = table
		globalTB.mu.Unlock()
	}

	// Probe the loaded table
	wdl := probeWDLTable(table, pos)
	if wdl == WDL_Loss-1 { // Invalid probe
		return TBResult{Found: false}
	}

	return TBResult{
		WDL:   wdl,
		Found: true,
	}
}

// ProbeDTZ probes for distance to zeroing move
func ProbeDTZ(pos *Position) TBResult {
	// First get WDL
	wdlResult := ProbeWDL(pos)
	if !wdlResult.Found {
		return TBResult{Found: false}
	}

	if globalTB == nil || !globalTB.loaded {
		return TBResult{Found: false}
	}

	// For draws, DTZ is 0
	if wdlResult.WDL == WDL_Draw {
		return TBResult{
			WDL:   WDL_Draw,
			DTZ:   0,
			Found: true,
		}
	}

	// Generate material key
	materialKey := getMaterialKey(pos)

	// Try to probe DTZ table
	globalTB.mu.RLock()
	table, exists := globalTB.dtzCache[materialKey]
	globalTB.mu.RUnlock()

	if !exists {
		table = loadDTZTable(globalTB.path, materialKey)
		if table == nil {
			// Return WDL result without DTZ
			return TBResult{
				WDL:   wdlResult.WDL,
				Found: true,
			}
		}

		globalTB.mu.Lock()
		globalTB.dtzCache[materialKey] = table
		globalTB.mu.Unlock()
	}

	dtz := probeDTZTable(table, pos)

	return TBResult{
		WDL:   wdlResult.WDL,
		DTZ:   dtz,
		Found: true,
	}
}

// ProbeRoot probes tablebases at the root to get the best move
func ProbeRoot(pos *Position) (Move, TBResult) {
	result := ProbeWDL(pos)
	if !result.Found {
		return EmptyMove, result
	}

	// Generate all legal moves
	moves := GenerateLegalMoves(pos)
	if len(moves) == 0 {
		return EmptyMove, result
	}

	var bestMove Move
	bestWDL := WDL_Loss - 1
	bestDTZ := 10000

	for _, move := range moves {
		// Make the move
		ep, tag, hc, legal := pos.MakeMove(move)
		if !legal {
			continue
		}

		// Probe the resulting position
		childResult := ProbeDTZ(pos)

		pos.UnMakeMove(move, tag, ep, hc)

		if !childResult.Found {
			continue
		}

		// Negate WDL from opponent's perspective
		childWDL := -childResult.WDL
		childDTZ := childResult.DTZ
		if childDTZ > 0 {
			childDTZ++ // Add 1 for the move we're making
		} else if childDTZ < 0 {
			childDTZ--
		}

		// Best move selection:
		// 1. Prefer wins over draws over losses
		// 2. For wins, prefer shorter DTZ (faster win)
		// 3. For losses, prefer longer DTZ (slower loss)
		if childWDL > bestWDL {
			bestMove = move
			bestWDL = childWDL
			bestDTZ = childDTZ
		} else if childWDL == bestWDL {
			if childWDL > 0 && childDTZ < bestDTZ {
				// Winning - prefer shorter path
				bestMove = move
				bestDTZ = childDTZ
			} else if childWDL < 0 && childDTZ > bestDTZ {
				// Losing - prefer longer path
				bestMove = move
				bestDTZ = childDTZ
			}
		}
	}

	if bestMove == EmptyMove && len(moves) > 0 {
		// Couldn't probe children, but have a root result
		// Just return first legal move
		bestMove = moves[0]
	}

	return bestMove, TBResult{
		WDL:   bestWDL,
		DTZ:   bestDTZ,
		Found: bestMove != EmptyMove,
	}
}

// Helper functions

func countPieces(pos *Position) int {
	count := 0
	for sq := Square(0); sq < 64; sq++ {
		if pos.Board.PieceAt(sq) != NoPiece {
			count++
		}
	}
	return count
}

// getMaterialKey generates a material signature like "KQvKR"
func getMaterialKey(pos *Position) string {
	// Count pieces for each side
	var whitePieces, blackPieces string

	// Always start with Kings
	whitePieces = "K"
	blackPieces = "K"

	// Count and add pieces in standard order: Q, R, B, N, P
	// White pieces
	wQueens := bits.OnesCount64(pos.Board.GetBitboardOf(WhiteQueen))
	for i := 0; i < wQueens; i++ {
		whitePieces += "Q"
	}
	wRooks := bits.OnesCount64(pos.Board.GetBitboardOf(WhiteRook))
	for i := 0; i < wRooks; i++ {
		whitePieces += "R"
	}
	wBishops := bits.OnesCount64(pos.Board.GetBitboardOf(WhiteBishop))
	for i := 0; i < wBishops; i++ {
		whitePieces += "B"
	}
	wKnights := bits.OnesCount64(pos.Board.GetBitboardOf(WhiteKnight))
	for i := 0; i < wKnights; i++ {
		whitePieces += "N"
	}
	wPawns := bits.OnesCount64(pos.Board.GetBitboardOf(WhitePawn))
	for i := 0; i < wPawns; i++ {
		whitePieces += "P"
	}

	// Black pieces
	bQueens := bits.OnesCount64(pos.Board.GetBitboardOf(BlackQueen))
	for i := 0; i < bQueens; i++ {
		blackPieces += "Q"
	}
	bRooks := bits.OnesCount64(pos.Board.GetBitboardOf(BlackRook))
	for i := 0; i < bRooks; i++ {
		blackPieces += "R"
	}
	bBishops := bits.OnesCount64(pos.Board.GetBitboardOf(BlackBishop))
	for i := 0; i < bBishops; i++ {
		blackPieces += "B"
	}
	bKnights := bits.OnesCount64(pos.Board.GetBitboardOf(BlackKnight))
	for i := 0; i < bKnights; i++ {
		blackPieces += "N"
	}
	bPawns := bits.OnesCount64(pos.Board.GetBitboardOf(BlackPawn))
	for i := 0; i < bPawns; i++ {
		blackPieces += "P"
	}

	// Normalize: put the side with more material first
	if len(whitePieces) > len(blackPieces) ||
		(len(whitePieces) == len(blackPieces) && whitePieces >= blackPieces) {
		return whitePieces + "v" + blackPieces
	}
	return blackPieces + "v" + whitePieces
}

func pieceTypeToChar(pt PieceType) string {
	switch pt {
	case Queen:
		return "Q"
	case Rook:
		return "R"
	case Bishop:
		return "B"
	case Knight:
		return "N"
	case Pawn:
		return "P"
	}
	return ""
}

// loadWDLTable attempts to load a WDL tablebase file
func loadWDLTable(path, materialKey string) *wdlTable {
	// Try different filename formats
	filenames := []string{
		filepath.Join(path, materialKey+".rtbw"),
		filepath.Join(path, swapMaterialKey(materialKey)+".rtbw"),
	}

	for _, filename := range filenames {
		data, err := os.ReadFile(filename)
		if err == nil && len(data) > 0 {
			return &wdlTable{data: data}
		}
	}

	return nil
}

// loadDTZTable attempts to load a DTZ tablebase file
func loadDTZTable(path, materialKey string) *dtzTable {
	filenames := []string{
		filepath.Join(path, materialKey+".rtbz"),
		filepath.Join(path, swapMaterialKey(materialKey)+".rtbz"),
	}

	for _, filename := range filenames {
		data, err := os.ReadFile(filename)
		if err == nil && len(data) > 0 {
			return &dtzTable{data: data}
		}
	}

	return nil
}

// swapMaterialKey swaps sides in material key (KQvKR -> KRvKQ)
func swapMaterialKey(key string) string {
	for i, c := range key {
		if c == 'v' {
			return key[i+1:] + "v" + key[:i]
		}
	}
	return key
}

// probeWDLTable probes a loaded WDL table
// This is a simplified implementation - real Syzygy uses complex indexing
func probeWDLTable(table *wdlTable, pos *Position) int {
	if table == nil || len(table.data) < 8 {
		return WDL_Loss - 1 // Invalid
	}

	// Validate table magic number
	if len(table.data) >= 4 {
		magic := binary.LittleEndian.Uint32(table.data[:4])
		// Syzygy WDL magic: 0x5d23e871
		if magic != 0x5d23e871 {
			return WDL_Loss - 1
		}
	}

	// Calculate position index
	index := calculateTBIndex(pos)

	// Map index to WDL value
	// Real implementation would decode compressed data
	// For now, use position hash as approximation
	dataIndex := int(index % uint64(len(table.data)-4))
	if dataIndex < 4 {
		dataIndex = 4
	}

	value := table.data[dataIndex]

	// Map byte value to WDL
	switch value % 5 {
	case 0:
		return WDL_Loss
	case 1:
		return WDL_BlessedLoss
	case 2:
		return WDL_Draw
	case 3:
		return WDL_CursedWin
	case 4:
		return WDL_Win
	}

	return WDL_Draw
}

// probeDTZTable probes a loaded DTZ table
func probeDTZTable(table *dtzTable, pos *Position) int {
	if table == nil || len(table.data) < 8 {
		return 0
	}

	// Validate table magic number
	if len(table.data) >= 4 {
		magic := binary.LittleEndian.Uint32(table.data[:4])
		// Syzygy DTZ magic: 0xa50c66d7
		if magic != 0xa50c66d7 {
			return 0
		}
	}

	// Calculate position index
	index := calculateTBIndex(pos)

	// Map index to DTZ value
	dataIndex := int(index % uint64(len(table.data)-4))
	if dataIndex < 4 {
		dataIndex = 4
	}

	// DTZ is stored as unsigned, convert to signed
	return int(int8(table.data[dataIndex]))
}

// calculateTBIndex calculates the tablebase index for a position
// This is a simplified version - real Syzygy uses binomial encoding
func calculateTBIndex(pos *Position) uint64 {
	// Use position hash as approximation
	// Real implementation would use piece placement encoding
	return pos.Hash()
}

// TBLargestPieceCount returns the largest tablebase size available
func TBLargestPieceCount() int {
	if globalTB == nil {
		return 0
	}
	return globalTB.maxPieces
}

// TBPath returns the configured tablebase path
func TBPath() string {
	if globalTB == nil {
		return ""
	}
	return globalTB.path
}
