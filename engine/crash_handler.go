package engine

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// Enable expensive debugging context in hot paths
// Set to false for performance in production/benchmarks
const ENABLE_HOT_PATH_DEBUG = false

// CrashHandler provides comprehensive panic recovery and crash reporting
type CrashHandler struct {
	logger  *log.Logger
	logFile *os.File
	mutex   sync.Mutex
	crashID int
}

var (
	globalCrashHandler *CrashHandler
	crashHandlerOnce   sync.Once
)

// GetGlobalCrashHandler returns the singleton crash handler
func GetGlobalCrashHandler() *CrashHandler {
	crashHandlerOnce.Do(func() {
		globalCrashHandler = NewCrashHandler()
	})
	return globalCrashHandler
}

// NewCrashHandler creates a new crash handler with logging
func NewCrashHandler() *CrashHandler {
	// Create crash log file
	execPath, err := os.Executable()
	if err != nil {
		execPath = "./ngn"
	}
	execDir := filepath.Dir(execPath)

	logPath := filepath.Join(execDir, "ngn_crashes.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// Fallback to stderr
		logFile = os.Stderr
	}

	logger := log.New(logFile, "[NGN CRASH] ", log.LstdFlags|log.Lmicroseconds|log.Lshortfile)

	handler := &CrashHandler{
		logger:  logger,
		logFile: logFile,
		crashID: 0,
	}

	// Log startup
	handler.logger.Println("=== NGN Crash Handler Initialized ===")
	handler.logger.Printf("Crash log: %s", logPath)

	return handler
}

// SafeRecover provides comprehensive panic recovery with stack traces and context
func (ch *CrashHandler) SafeRecover(functionName string, context map[string]interface{}) {
	if r := recover(); r != nil {
		ch.mutex.Lock()
		ch.crashID++
		crashID := ch.crashID
		ch.mutex.Unlock()

		// Get stack trace
		stackTrace := debug.Stack()

		// Log comprehensive crash information
		ch.logger.Printf("\n"+
			"████████████████████████████████████████████████████████████████\n"+
			"🚨 PANIC RECOVERED - CRASH ID #%d\n"+
			"████████████████████████████████████████████████████████████████\n"+
			"Function: %s\n"+
			"Time: %s\n"+
			"Panic: %v\n"+
			"Type: %T\n"+
			"Go Version: %s\n"+
			"Go Routines: %d\n"+
			"────────────────────────────────────────────────────────────────\n"+
			"CONTEXT:\n",
			crashID, functionName, time.Now().Format(time.RFC3339), r, r, runtime.Version(), runtime.NumGoroutine())

		// Log context information
		for key, value := range context {
			ch.logger.Printf("  %s: %+v", key, value)
		}

		ch.logger.Printf("────────────────────────────────────────────────────────────────\n"+
			"STACK TRACE:\n%s\n"+
			"████████████████████████████████████████████████████████████████\n", string(stackTrace))

		// Force flush
		if ch.logFile != os.Stderr {
			ch.logFile.Sync()
		}

		// Also log to stderr for immediate visibility
		fmt.Fprintf(os.Stderr, "🚨 NGN ENGINE PANIC #%d in %s: %v\n", crashID, functionName, r)
		fmt.Fprintf(os.Stderr, "See ngn_crashes.log for full details\n")
	}
}

// WrapFunction wraps a function with panic recovery
func (ch *CrashHandler) WrapFunction(functionName string, fn func(), context map[string]interface{}) {
	defer ch.SafeRecover(functionName, context)
	fn()
}

// WrapFunctionWithReturn wraps a function that returns a value
func WrapFunctionWithReturn[T any](functionName string, fn func() T, defaultReturn T, context map[string]interface{}) T {
	ch := GetGlobalCrashHandler()
	defer ch.SafeRecover(functionName, context)
	return fn()
}

// WrapSearch wraps search functions with detailed context
func WrapSearch(functionName string, fn func() *SearchInfo, pos *Position, depth int) *SearchInfo {
	context := map[string]interface{}{
		"position_fen": GenerateFEN(pos),
		"depth":        depth,
		"turn":         pos.Turn(),
		"move_count":   len(GenerateLegalMoves(pos)),
	}

	ch := GetGlobalCrashHandler()
	defer ch.SafeRecover(functionName, context)

	return fn()
}

// WrapMoveGeneration wraps move generation with context
func WrapMoveGeneration(functionName string, fn func() []Move, pos *Position) []Move {
	context := map[string]interface{}{
		"position_fen": GenerateFEN(pos),
		"turn":         pos.Turn(),
		"castling":     pos.Tag,
		"en_passant":   pos.EnPassant,
	}

	ch := GetGlobalCrashHandler()
	defer ch.SafeRecover(functionName, context)

	return fn()
}

// WrapEvaluation wraps evaluation functions with context
func WrapEvaluation(functionName string, fn func() int, board *Bitboard) int {
	if !ENABLE_HOT_PATH_DEBUG {
		// Fast path: absolutely zero overhead in hot path
		// Do NOT set up a defer when debug is disabled.
		return fn()
	}

	// Slow path - full debugging context (disabled by default)
	context := map[string]interface{}{
		"white_king":   board.GetBitboardOf(WhiteKing),
		"black_king":   board.GetBitboardOf(BlackKing),
		"white_pieces": board.GetWhitePieces(),
		"black_pieces": board.GetBlackPieces(),
		"all_pieces":   board.AllPieces(),
	}

	ch := GetGlobalCrashHandler()
	defer ch.SafeRecover(functionName, context)

	return fn()
}

// WrapUCI wraps UCI command handlers with context
func WrapUCI(functionName string, fn func(), command string, args []string) {
	context := map[string]interface{}{
		"command": command,
		"args":    args,
		"time":    time.Now().Format(time.RFC3339),
	}

	ch := GetGlobalCrashHandler()
	defer ch.SafeRecover(functionName, context)

	fn()
}

// Close closes the crash handler
func (ch *CrashHandler) Close() error {
	if ch.logFile != nil && ch.logFile != os.Stderr {
		return ch.logFile.Close()
	}
	return nil
}
