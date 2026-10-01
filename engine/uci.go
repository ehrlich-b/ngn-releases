package engine

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Global opening book instance
var globalOpeningBook *PolyglotBook

// InitOpeningBook loads an opening book from the given path
func InitOpeningBook(path string) error {
	book, err := LoadPolyglotBook(path)
	if err != nil {
		return err
	}
	globalOpeningBook = book
	return nil
}

// GetOpeningBook returns the global opening book
func GetOpeningBook() *PolyglotBook {
	return globalOpeningBook
}

// TryLoadDefaultBook attempts to load a book from common locations
func TryLoadDefaultBook() {
	// Try common book locations
	bookPaths := []string{
		"book.bin",
		"opening.bin",
		"openings.bin",
		"./book/book.bin",
		"./book/opening.bin",
	}

	for _, path := range bookPaths {
		if err := InitOpeningBook(path); err == nil {
			return
		}
	}
}

type uciSearchState uint8

const (
	uciSearchIdle uciSearchState = iota
	uciSearchPrepared
	uciSearchRunning
	uciSearchStopping
)

type uciSearchSession struct {
	id                uint64
	done              chan struct{}
	cancelOnce        sync.Once
	completeOnce      sync.Once
	cancelled         chan struct{}
	resultOnce        sync.Once
	threadReceiptOnce sync.Once

	receivedAt        time.Time
	params            SearchParams
	rootSource        *Position
	root              *Position
	legalMoves        []Move
	fallback          Move
	timeManager       *TimeManager
	movesPlayed       int
	lastScore         int
	debugMode         bool
	allowResignation  bool
	copyPosition      func(*Position) *Position
	openingBook       *PolyglotBook
	ownBook           bool
	configuredThreads int

	outputMu sync.Mutex
	suppress atomic.Bool
}

func (s *uciSearchSession) cancel(searcher *SearchEngine) {
	s.cancelOnce.Do(func() {
		searcher.RequestStop()
		close(s.cancelled)
	})
}

func (s *uciSearchSession) isCancelled() bool {
	select {
	case <-s.cancelled:
		return true
	default:
		return false
	}
}

func (s *uciSearchSession) suppressOutput() {
	s.suppress.Store(true)
}

type uciSessionWriter struct {
	session *uciSearchSession
	w       io.Writer
}

func (w *uciSessionWriter) Write(p []byte) (int, error) {
	w.session.outputMu.Lock()
	defer w.session.outputMu.Unlock()
	if w.session.suppress.Load() {
		return len(p), nil
	}
	return w.w.Write(p)
}

func (w *uciSessionWriter) sendMessage(uci *UCIEngine, message string) {
	w.session.outputMu.Lock()
	defer w.session.outputMu.Unlock()
	if w.session.suppress.Load() {
		return
	}
	uci.rawUCILogger.Printf("<<< %s", message)
	fmt.Fprintln(w.w, message)
}

// UCI Engine represents the UCI interface for the chess engine
type UCIEngine struct {
	position                *Position
	searcher                *SearchEngine
	lifecycleMu             sync.Mutex
	searchState             uciSearchState
	activeSearch            *uciSearchSession
	nextSearchID            uint64
	debugMode               bool
	engineName              string
	engineAuthor            string
	version                 string
	threadScheduler         func(int)
	timeManager             *TimeManager
	movesPlayed             int
	lastScore               int
	debugLogger             *log.Logger
	rawUCILogger            *log.Logger
	allowResignation        bool
	now                     func() time.Time
	copyPosition            func(*Position) *Position
	evaluatorConfig         uciEvaluatorConfig
	startupDefaults         uciEvaluatorDefaults
	startupOwnBook          bool
	researchEvaluatorLocked bool
	ownBook                 bool
}

const defaultUCIMoveOverheadMilliseconds = 100

func newUCITimeManager() *TimeManager {
	tm := NewTimeManager()
	if err := tm.SetMoveOverhead(defaultUCIMoveOverheadMilliseconds); err != nil {
		panic(err)
	}
	return tm
}

// NewUCIEngine creates a new UCI engine instance
func NewUCIEngine() *UCIEngine {
	// Set up debug logging to file in executable directory
	execPath, err := os.Executable()
	if err != nil {
		execPath = "./ngn" // fallback
	}
	execDir := filepath.Dir(execPath)
	// Debug logs are opt-in via NGN_DEBUG_LOG: they're append-only and unbounded
	// and reached multi-GB during long gauntlet/SPRT runs. Default to io.Discard so
	// a background run can't fill the disk (and never leak debug text onto stdout,
	// which is the UCI protocol channel). The loggers stay valid either way.
	var logW, rawW io.Writer = io.Discard, io.Discard
	if os.Getenv("NGN_DEBUG_LOG") != "" {
		if f, e := os.OpenFile(filepath.Join(execDir, "ngn_debug.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); e == nil {
			logW = f
		}
		if f, e := os.OpenFile(filepath.Join(execDir, "ngn_raw_uci.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); e == nil {
			rawW = f
		}
	}
	logger := log.New(logW, "[NGN DEBUG] ", log.LstdFlags|log.Lmicroseconds)
	rawUCILogger := log.New(rawW, "", log.LstdFlags|log.Lmicroseconds)

	logger.Println("=== NGN Chess Engine Starting ===")
	logger.Printf("Executable path: %s", execPath)
	logger.Printf("Log file directory: %s", execDir)
	rawUCILogger.Println("=== RAW UCI MESSAGE LOG ===")
	rawUCILogger.Println("Format: >>> = sent to engine, <<< = sent from engine")

	// Try to load opening book
	TryLoadDefaultBook()
	if book := GetOpeningBook(); book != nil {
		logger.Printf("Opening book loaded: %d entries", book.Size())
	} else {
		logger.Println("No opening book found")
	}

	return &UCIEngine{
		position:                newUCIStartingPosition(),
		searcher:                NewSearchEngine(),
		debugMode:               false,
		engineName:              "ngn",
		engineAuthor:            "ngn team",
		version:                 "0.1.0",
		timeManager:             newUCITimeManager(),
		movesPlayed:             0,
		lastScore:               0,
		debugLogger:             logger,
		rawUCILogger:            rawUCILogger,
		allowResignation:        false, // Default to no resignation
		now:                     time.Now,
		copyPosition:            func(pos *Position) *Position { return pos.Copy() },
		evaluatorConfig:         newUCIEvaluatorConfig(),
		startupDefaults:         uciEvaluatorDefaults{backend: uciEvaluatorHCE},
		researchEvaluatorLocked: true,
		ownBook:                 true,
		startupOwnBook:          true,
	}
}

// sendUCIMessage sends a UCI message and logs it
func (uci *UCIEngine) sendUCIMessage(output io.Writer, message string) {
	if sessionOutput, ok := output.(*uciSessionWriter); ok {
		sessionOutput.sendMessage(uci, message)
		return
	}
	uci.rawUCILogger.Printf("<<< %s", message)
	fmt.Fprintln(output, message)
}

// SetResignationPolicy sets whether the engine should resign in hopeless positions
func (uci *UCIEngine) SetResignationPolicy(allowResignation bool) {
	uci.lifecycleMu.Lock()
	uci.allowResignation = allowResignation
	uci.lifecycleMu.Unlock()
}

// ConfigureStartupVersion sets the release identity before Run. Metadata is
// read under the same lifecycle lock, independently of search/evaluation state.
func (uci *UCIEngine) ConfigureStartupVersion(version string) error {
	if version == "" || strings.ContainsAny(version, "\r\n\x00") {
		return fmt.Errorf("invalid UCI version")
	}
	uci.lifecycleMu.Lock()
	uci.version = version
	uci.lifecycleMu.Unlock()
	return nil
}

// ConfigureStartupThreadScheduler lets the one-engine executable own its Go
// scheduler. Library engines retain no process-global scheduling side effects.
func (uci *UCIEngine) ConfigureStartupThreadScheduler(update func(int)) {
	uci.joinSearch(true)
	uci.lifecycleMu.Lock()
	uci.threadScheduler = update
	uci.lifecycleMu.Unlock()
	if update != nil {
		update(uci.searcher.ThreadCount())
	}
}

// ConfigureStartupOwnBook lets an explicit launcher retain the audited
// book-free configuration while ordinary startup keeps its historical default.
func (uci *UCIEngine) ConfigureStartupOwnBook(enabled bool) {
	uci.joinSearch(true)
	uci.lifecycleMu.Lock()
	uci.ownBook = enabled
	uci.startupOwnBook = enabled
	uci.lifecycleMu.Unlock()
}

func (uci *UCIEngine) nowTime() time.Time {
	if uci.now == nil {
		return time.Now()
	}
	return uci.now()
}

// Helper function to create starting position
func newUCIStartingPosition() *Position {
	pos := &Position{
		Board:         StartingBoard(),
		Tag:           WhiteCanCastleKingSide | WhiteCanCastleQueenSide | BlackCanCastleKingSide | BlackCanCastleQueenSide | WhiteToMove,
		EnPassant:     NoSquare,
		HalfMoveClock: 0,
		Positions:     make(map[uint64]int),
	}
	// Add initial position to repetition tracking
	pos.positionsMutex.Lock()
	pos.Positions[pos.Hash()] = 1
	pos.positionsMutex.Unlock()
	return pos
}

// Run starts the UCI engine main loop
// syncWriter serializes concurrent writes to the underlying writer. The search
// goroutine and the main Run loop both emit UCI lines (info / bestmove vs
// readyok / id ...); without serialization those writes race on the writer (a
// bytes.Buffer in tests, os.Stdout in production). One lock per whole message
// keeps lines from interleaving.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

func (uci *UCIEngine) Run(input io.Reader, output io.Writer) {
	defer GetGlobalCrashHandler().SafeRecover("UCIEngine.Run", map[string]interface{}{
		"engine_name": uci.engineName,
		"debug_mode":  uci.debugMode,
	})

	// Serialize every write so the search goroutine and this loop cannot corrupt
	// the output channel; hand the wrapped writer to all command handlers.
	out := &syncWriter{w: output}
	scanner := bufio.NewScanner(input)

	for scanner.Scan() {
		command := strings.TrimSpace(scanner.Text())
		if command == "" {
			continue
		}

		// Log raw UCI input
		uci.rawUCILogger.Printf(">>> %s", command)

		if uci.debugMode {
			fmt.Fprintf(out, "info string received: %s\n", command)
		}

		// Wrap each command in panic recovery; handleCommand reports whether the
		// loop should terminate (the quit command).
		quit := false
		func() {
			defer GetGlobalCrashHandler().SafeRecover("UCIEngine.handleCommand", map[string]interface{}{
				"command": command,
				"time":    time.Now().Format(time.RFC3339),
			})
			quit = uci.handleCommand(command, out)
		}()
		if quit {
			break
		}
	}

	// The loop has ended (quit command or input EOF): stop and JOIN any in-flight
	// search so the search goroutine never outlives Run. This is what makes
	// process-level tests race-clean — no output write or shared-state mutation
	// happens after Run returns.
	uci.joinSearch(true)
}

// handleCommand processes UCI commands
func (uci *UCIEngine) handleCommand(command string, output io.Writer) (quit bool) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "uci":
		uci.handleUCI(output)
	case "debug":
		uci.handleDebug(args, output)
	case "isready":
		uci.handleIsReady(output)
	case "setoption":
		uci.handleSetOption(args, output)
	case "register":
		uci.handleRegister(args, output)
	case "ucinewgame":
		uci.handleNewGame(output)
	case "position":
		uci.handlePosition(args, output)
	case "go":
		uci.handleGo(args, output)
	case "stop":
		uci.handleStop(output)
	case "ponderhit":
		uci.handlePonderHit(output)
	case "eval":
		uci.handleEvalDiagnostic(output)
	case "quit":
		uci.handleQuit(output)
		return true
	default:
		if uci.debugMode {
			fmt.Fprintf(output, "info string unknown command: %s\n", cmd)
		}
	}
	return false
}

// handleUCI responds to the uci command
func (uci *UCIEngine) handleUCI(output io.Writer) {
	uci.lifecycleMu.Lock()
	name, version, author := uci.engineName, uci.version, uci.engineAuthor
	evalBackendDefault := uci.startupDefaults.backend
	evalFileDefault := uci.startupDefaults.file
	ownBookDefault := uci.startupOwnBook
	researchEvaluatorLocked := uci.researchEvaluatorLocked
	uci.lifecycleMu.Unlock()
	uci.sendUCIMessage(output, fmt.Sprintf("id name %s %s", name, version))
	uci.sendUCIMessage(output, fmt.Sprintf("id author %s", author))
	if evalBackendDefault == "" {
		evalBackendDefault = uciEvaluatorHCE
	}
	if evalFileDefault == "" {
		evalFileDefault = "<empty>"
	}

	// Engine options (we can add more later)
	uci.sendUCIMessage(output, "option name Hash type spin default 128 min 1 max 1024")
	uci.sendUCIMessage(output, "option name Threads type spin default 1 min 1 max 64")
	uci.sendUCIMessage(output, "option name Move Overhead type spin default 100 min 0 max 5000")
	uci.sendUCIMessage(output, fmt.Sprintf("option name OwnBook type check default %t", ownBookDefault))
	uci.sendUCIMessage(output, "option name SyzygyPath type string default <empty>")
	if !researchEvaluatorLocked {
		uci.sendUCIMessage(output, fmt.Sprintf("option name EvalBackend type combo default %s var hce var ngn-v1 var sf18-big var counter-5.5", evalBackendDefault))
		uci.sendUCIMessage(output, fmt.Sprintf("option name EvalFile type string default %s", evalFileDefault))
	}

	// Tunable search parameters (for SPSA / manual A/B; defaults == shipped values).
	for _, p := range TunableSearchParams {
		uci.sendUCIMessage(output, fmt.Sprintf("option name %s type spin default %d min %d max %d", p.Name, p.Def, p.Min, p.Max))
	}

	uci.sendUCIMessage(output, "uciok")
}

// handleDebug handles debug mode toggle
func (uci *UCIEngine) handleDebug(args []string, output io.Writer) {
	if len(args) > 0 {
		switch args[0] {
		case "on":
			uci.debugMode = true
		case "off":
			uci.debugMode = false
		}
	}
}

// handleIsReady responds to isready command
func (uci *UCIEngine) handleIsReady(output io.Writer) {
	uci.sendUCIMessage(output, "readyok")
}

// handleSetOption handles option setting
func (uci *UCIEngine) handleSetOption(args []string, output io.Writer) {
	// Parse "name <name> value <value>" format
	var optionName, optionValue string
	for i := 0; i < len(args); i++ {
		if args[i] == "name" && i+1 < len(args) {
			// Collect name until "value" keyword
			nameStart := i + 1
			nameEnd := nameStart
			for j := nameStart; j < len(args); j++ {
				if args[j] == "value" {
					break
				}
				nameEnd = j + 1
			}
			optionName = strings.Join(args[nameStart:nameEnd], " ")
		}
		if args[i] == "value" && i+1 < len(args) {
			optionValue = strings.Join(args[i+1:], " ")
			break
		}
	}

	// Handle specific options
	switch strings.ToLower(optionName) {
	case "syzygypath":
		uci.joinSearch(true)
		if optionValue != "" && optionValue != "<empty>" {
			err := InitSyzygy(optionValue)
			if err != nil {
				uci.debugLogger.Printf("Failed to initialize Syzygy tablebases: %v", err)
				if uci.debugMode {
					fmt.Fprintf(output, "info string syzygy init failed: %v\n", err)
				}
			} else {
				uci.debugLogger.Printf("Syzygy tablebases loaded from: %s (max %d pieces)", optionValue, TBLargestPieceCount())
				if uci.debugMode {
					fmt.Fprintf(output, "info string syzygy loaded: %s (%d pieces)\n", optionValue, TBLargestPieceCount())
				}
			}
		}
	case "move overhead":
		milliseconds, err := strconv.Atoi(optionValue)
		if err != nil || milliseconds < minMoveOverheadMilliseconds || milliseconds > maxMoveOverheadMilliseconds {
			break
		}
		uci.joinSearch(true)
		uci.lifecycleMu.Lock()
		err = uci.timeManager.SetMoveOverhead(milliseconds)
		uci.lifecycleMu.Unlock()
		if err != nil {
			panic(err)
		}
	case "hash":
		mb, err := strconv.Atoi(optionValue)
		if err != nil {
			break
		}
		if mb < 1 {
			mb = 1
		} else if mb > 1024 {
			mb = 1024
		}
		uci.joinSearch(true)
		if err := uci.searcher.ResizeHash(mb); err != nil {
			panic(err)
		}
	case "threads":
		count, err := strconv.Atoi(optionValue)
		if err != nil || validateThreadCount(count) != nil {
			break
		}
		uci.joinSearch(true)
		if err := uci.searcher.ConfigureThreads(count); err != nil {
			panic(err)
		}
		uci.lifecycleMu.Lock()
		update := uci.threadScheduler
		uci.lifecycleMu.Unlock()
		if update != nil {
			update(count)
		}
		uci.sendUCIMessage(output, fmt.Sprintf("info string threads configured %d effective %d", count, count))
	case "ownbook":
		normalized := strings.ToLower(strings.TrimSpace(optionValue))
		if normalized != "true" && normalized != "false" {
			return
		}
		uci.joinSearch(true)
		uci.lifecycleMu.Lock()
		uci.ownBook = normalized == "true"
		uci.lifecycleMu.Unlock()
	case "evalbackend":
		if uci.researchEvaluatorIsLocked() {
			uci.sendEvalOptionError(output, "research startup evaluator is locked")
			return
		}
		uci.handleEvalBackendOption(optionValue, output)
		return
	case "evalfile":
		if uci.researchEvaluatorIsLocked() {
			uci.sendEvalOptionError(output, "research startup evaluator is locked")
			return
		}
		uci.handleEvalFileOption(optionValue, output)
		return
	default:
		if isTunableParam(optionName) {
			uci.joinSearch(true)
			setTunableParam(optionName, optionValue)
		}
	}

	if uci.debugMode {
		fmt.Fprintf(output, "info string option set: %s = %s\n", optionName, optionValue)
	}
}

func isTunableParam(name string) bool {
	for i := range TunableSearchParams {
		if strings.EqualFold(TunableSearchParams[i].Name, name) {
			return true
		}
	}
	return false
}

// setTunableParam applies a search-parameter override registered in
// TunableSearchParams (matched case-insensitively), clamping to [Min,Max].
// Returns false if the name is not a known tunable.
func setTunableParam(name, value string) bool {
	lname := strings.ToLower(name)
	for i := range TunableSearchParams {
		p := &TunableSearchParams[i]
		if strings.ToLower(p.Name) == lname {
			v, err := strconv.Atoi(value)
			if err != nil {
				return true
			}
			if v < p.Min {
				v = p.Min
			}
			if v > p.Max {
				v = p.Max
			}
			*p.Ptr = v
			return true
		}
	}
	return false
}

// handleRegister handles registration (not needed for open source engine)
func (uci *UCIEngine) handleRegister(args []string, output io.Writer) {
	// Open source engine, no registration needed
}

// handleNewGame handles ucinewgame command
func (uci *UCIEngine) handleNewGame(output io.Writer) {
	uci.joinSearch(true)
	uci.position = newUCIStartingPosition()
	// Clear game-scoped TT and histories. Same-model HCE full and pawn caches stay warm.
	uci.searcher.NewGame()

	// Reset game state
	uci.movesPlayed = 0
	uci.lastScore = 0
	moveOverhead := uci.timeManager.moveOverhead
	uci.timeManager = newUCITimeManager()
	uci.timeManager.moveOverhead = moveOverhead

	if uci.debugMode {
		fmt.Fprintf(output, "info string new game started\n")
	}
}

// handlePosition handles position setup
func (uci *UCIEngine) handlePosition(args []string, output io.Writer) {
	if len(args) == 0 {
		return
	}
	uci.joinSearch(true)

	switch args[0] {
	case "startpos":
		uci.position = newUCIStartingPosition()
		args = args[1:] // Remove "startpos"
	case "fen":
		if len(args) < 7 { // Need at least "fen" + 6 FEN parts
			if uci.debugMode {
				fmt.Fprintf(output, "info string invalid FEN command\n")
			}
			return
		}

		// Join FEN parts (positions 1-6 after "fen")
		fenString := strings.Join(args[1:7], " ")
		pos, err := ParseFEN(fenString)
		if err != nil {
			if uci.debugMode {
				fmt.Fprintf(output, "info string FEN parse error: %v\n", err)
			}
			return
		}
		uci.position = pos
		args = args[7:] // Remove "fen" and FEN string
	}

	// Handle moves if present
	if len(args) >= 2 && args[0] == "moves" {
		moves := args[1:]
		uci.movesPlayed = 0                       // Reset move counter
		uci.searcher.SetLastMovePlayed(EmptyMove) // Reset last move for new position
		for _, moveStr := range moves {
			move, err := ParseAlgebraicMove(moveStr, uci.position)
			if err != nil {
				if uci.debugMode {
					fmt.Fprintf(output, "info string invalid move: %s (%v)\n", moveStr, err)
				}
				return
			}

			// Make the move using GameMakeMove to track repetition history
			_, _, _, _ = uci.position.GameMakeMove(move)
			uci.movesPlayed++
			// Track the last move played for counter move heuristic
			uci.searcher.SetLastMovePlayed(move)
		}
	}
}

// SearchParams holds parameters for the search
type SearchParams struct {
	Depth     int
	Nodes     uint64
	MoveTime  int // milliseconds
	WhiteTime int // milliseconds
	BlackTime int // milliseconds
	WhiteInc  int // milliseconds increment
	BlackInc  int // milliseconds increment
	MovesToGo int
	Infinite  bool
	Ponder    bool
}

func parseSearchParams(args []string) SearchParams {
	params := SearchParams{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "depth":
			if i+1 < len(args) {
				if value, err := strconv.Atoi(args[i+1]); err == nil {
					params.Depth = value
				}
				i++
			}
		case "nodes":
			if i+1 < len(args) {
				if value, err := strconv.ParseUint(args[i+1], 10, 64); err == nil {
					params.Nodes = value
				}
				i++
			}
		case "movetime":
			if i+1 < len(args) {
				if value, err := strconv.Atoi(args[i+1]); err == nil {
					params.MoveTime = value
				}
				i++
			}
		case "wtime":
			if i+1 < len(args) {
				if value, err := strconv.Atoi(args[i+1]); err == nil {
					params.WhiteTime = value
				}
				i++
			}
		case "btime":
			if i+1 < len(args) {
				if value, err := strconv.Atoi(args[i+1]); err == nil {
					params.BlackTime = value
				}
				i++
			}
		case "winc":
			if i+1 < len(args) {
				if value, err := strconv.Atoi(args[i+1]); err == nil {
					params.WhiteInc = value
				}
				i++
			}
		case "binc":
			if i+1 < len(args) {
				if value, err := strconv.Atoi(args[i+1]); err == nil {
					params.BlackInc = value
				}
				i++
			}
		case "movestogo":
			if i+1 < len(args) {
				if value, err := strconv.Atoi(args[i+1]); err == nil {
					params.MovesToGo = value
				}
				i++
			}
		case "infinite":
			params.Infinite = true
		case "ponder":
			params.Ponder = true
		}
	}

	// Fixed-node searches retain the one-thread policy: without another limit,
	// only the explicit node budget ends the search.
	if params.Nodes > 0 && params.Depth == 0 && params.MoveTime == 0 &&
		params.WhiteTime == 0 && params.BlackTime == 0 {
		params.Infinite = true
	}
	return params
}

func (uci *UCIEngine) beginSearchSession(receivedAt time.Time, params SearchParams) (*uciSearchSession, bool) {
	uci.lifecycleMu.Lock()
	defer uci.lifecycleMu.Unlock()
	if uci.activeSearch != nil {
		return nil, false
	}

	// Clear and configure receiver control before publication. A stop may arrive
	// at any point after activeSearch becomes visible and must remain observable.
	uci.searcher.ClearStop()
	uci.searcher.SetMaxNodes(params.Nodes)
	tm := *uci.timeManager
	tm.now = uci.now
	uci.nextSearchID++
	session := &uciSearchSession{
		id:                uci.nextSearchID,
		done:              make(chan struct{}),
		cancelled:         make(chan struct{}),
		receivedAt:        receivedAt,
		params:            params,
		rootSource:        uci.position,
		timeManager:       &tm,
		movesPlayed:       uci.movesPlayed,
		lastScore:         uci.lastScore,
		debugMode:         uci.debugMode,
		allowResignation:  uci.allowResignation,
		copyPosition:      uci.copyPosition,
		openingBook:       GetOpeningBook(),
		ownBook:           uci.ownBook,
		configuredThreads: uci.searcher.ThreadCount(),
	}
	uci.activeSearch = session
	uci.searchState = uciSearchPrepared
	return session, true
}

func (uci *UCIEngine) markSearchRunning(session *uciSearchSession) bool {
	uci.lifecycleMu.Lock()
	defer uci.lifecycleMu.Unlock()
	if uci.activeSearch != session || uci.searchState != uciSearchPrepared || session.isCancelled() {
		return false
	}
	uci.searchState = uciSearchRunning
	return true
}

func (uci *UCIEngine) finishSearchSession(session *uciSearchSession) {
	session.completeOnce.Do(func() {
		uci.lifecycleMu.Lock()
		if uci.activeSearch == session {
			uci.activeSearch = nil
			uci.searchState = uciSearchIdle
		}
		close(session.done)
		uci.lifecycleMu.Unlock()
	})
}

func (uci *UCIEngine) isSearching() bool {
	uci.lifecycleMu.Lock()
	defer uci.lifecycleMu.Unlock()
	return uci.activeSearch != nil
}

// handleGo admits a search. The receipt clock is sampled before logging,
// parsing, allocation, position cloning, or any other setup work.
func (uci *UCIEngine) handleGo(args []string, output io.Writer) {
	receivedAt := uci.nowTime()
	uci.debugLogger.Printf("handleGo called with args: %v", args)
	params := parseSearchParams(args)
	session, accepted := uci.beginSearchSession(receivedAt, params)
	if !accepted {
		uci.debugLogger.Println("Already searching, returning")
		if uci.debugMode {
			fmt.Fprintln(output, "info string already searching, returning")
		}
		return
	}
	// The session is already published in prepared state. The coordinator owns
	// all remaining setup and observes cancellation before entering search.
	go uci.runSearchSession(session, output)
	uci.debugLogger.Printf("Search coordinator launched for session %d", session.id)
}

// emitThreadReceipt publishes one per-go admission width after setup selects it. Completion is registered first and therefore runs last.
func (uci *UCIEngine) emitThreadReceipt(session *uciSearchSession, output io.Writer, effective int) {
	if session.configuredThreads <= 1 {
		return
	}
	session.threadReceiptOnce.Do(func() {
		uci.sendUCIMessage(output, fmt.Sprintf("info string threads configured %d effective %d", session.configuredThreads, effective))
	})
}

func (uci *UCIEngine) runSearchSession(session *uciSearchSession, output io.Writer) {
	defer uci.finishSearchSession(session)
	sessionOutput := &uciSessionWriter{session: session, w: output}
	defer func() {
		if recovered := recover(); recovered != nil {
			originalStack := string(debug.Stack())
			if helperPanic, ok := recovered.(*searchWorkerPanic); ok {
				originalStack = string(helperPanic.stack)
			}
			uci.debugLogger.Printf("search session %d failed: %v", session.id, recovered)
			// Publish the active session's explicit error and legal fallback first.
			// Re-panic only inside the existing crash reporter afterward so its
			// recovery cannot bypass protocol completion.
			uci.publishSearchFailure(session, output, recovered)
			func() {
				defer GetGlobalCrashHandler().SafeRecover("UCIEngine.searchSession", map[string]interface{}{
					"session_id":     session.id,
					"params":         session.params,
					"moves_played":   session.movesPlayed,
					"original_stack": originalStack,
				})
				panic(recovered)
			}()
		}
	}()

	if !uci.prepareSearchSession(session, sessionOutput) {
		uci.emitThreadReceipt(session, sessionOutput, 1)
		uci.publishSearchResult(session, output, session.fallback, 0)
		return
	}
	if !uci.markSearchRunning(session) {
		uci.emitThreadReceipt(session, sessionOutput, 1)
		uci.publishSearchResult(session, output, session.fallback, 0)
		return
	}
	uci.searchSession(session, sessionOutput, output)
}

// prepareSearchSession holds an HCE/PST read lease while board fallback
// generation and copying use the process tables. It releases that lease before
// searchIterativeDeepeningPrepared takes the established sessionMu -> HCE lease
// order; the prepared search recomputes the copied root accumulator under the
// generation it admits.
func (uci *UCIEngine) prepareSearchSession(session *uciSearchSession, output io.Writer) bool {
	mustAcquireHCEModelUse()
	defer releaseHCEModelUse()

	// Establish a legal fallback before cloning or evaluator/search setup. The
	// command loop cannot mutate rootSource until it has cancelled and joined us.
	session.legalMoves = GenerateLegalMoves(session.rootSource)
	if len(session.legalMoves) > 0 {
		session.fallback = session.legalMoves[0]
	}
	if session.debugMode {
		fmt.Fprintln(output, "info string handleGo called")
	}
	uci.debugLogger.Println("About to set up time management")
	if session.debugMode {
		fmt.Fprintln(output, "info string setting up time management")
	}
	session.timeManager.SetTimeControlAt(session.params, session.rootSource.Turn() == White, session.receivedAt)
	uci.debugLogger.Println("Time management set, about to update game state")
	if session.debugMode {
		fmt.Fprintln(output, "info string updating game state")
	}
	session.timeManager.UpdateGameState(session.movesPlayed, session.lastScore, session.lastScore, session.rootSource)
	uci.debugLogger.Println("About to start search goroutine")
	if session.debugMode {
		fmt.Fprintln(output, "info string starting search goroutine")
	}
	if session.isCancelled() || session.timeManager.SetupDeadlineExceeded() {
		return false
	}

	copyPosition := session.copyPosition
	if copyPosition == nil {
		copyPosition = func(pos *Position) *Position { return pos.Copy() }
	}
	session.root = copyPosition(session.rootSource)
	if session.root == nil {
		panic("position copy returned nil")
	}
	return !session.isCancelled() && !session.timeManager.SetupDeadlineExceeded()
}

// searchSession performs the actual search on session-private state.
func (uci *UCIEngine) searchSession(session *uciSearchSession, output io.Writer, rawOutput io.Writer) {
	params := session.params
	uci.debugLogger.Printf("search() function started with params: %+v", params)
	if session.debugMode {
		fmt.Fprintf(output, "info string search() function started\n")
	}

	uci.debugLogger.Println("search initialized, setting flags")
	if session.debugMode {
		fmt.Fprintf(output, "info string search initialized\n")
	}

	// Determine maximum search depth
	maxDepth := params.Depth
	if maxDepth <= 0 {
		// No explicit depth specified - use time-based search with high max depth
		maxDepth = MaximumDepth
	}

	// Initialize search variables
	var bestMove Move
	var bestScore int
	lastIterationScore := session.lastScore

	if session.debugMode {
		fmt.Fprintf(output, "info string time control: %s, allocated: %dms\n",
			session.timeManager.GetTimeControlString(),
			session.timeManager.GetAllocatedTime().Milliseconds())
	}

	// Debug: Always output what we're doing
	if session.debugMode {
		fmt.Fprintf(output, "info string starting search, maxDepth=%d\n", maxDepth)
	}

	// Check opening books first when enabled for this immutable search session.
	// OwnBook=false bypasses both external Polyglot and embedded fallback books.
	bookMove, bookFound := EmptyMove, false
	if session.ownBook {
		if book := session.openingBook; book != nil {
			bookMove, bookFound = book.ProbeBook(session.root)
		}
		// Fallback to embedded book if no external book or no hit.
		if !bookFound {
			bookMove, bookFound = ProbeEmbeddedBook(session.root)
		}
	}

	if bookFound {
		uci.emitThreadReceipt(session, output, 1)
		uci.debugLogger.Printf("Opening book hit! Move: %v", bookMove.ToString())
		if session.debugMode {
			fmt.Fprintf(output, "info string opening book move: %s\n", bookMove.ToString())
		}

		bestMove = bookMove
		bestScore = 50
		uci.debugLogger.Printf("Using book move: %v with score %d", bestMove.ToString(), bestScore)

		// Output the book move result
		fmt.Fprintf(output, "info depth 1 score cp %d nodes 1 time 0 nps 0 pv %s\n",
			bestScore, bestMove.ToString())
	} else if tbMove, tbResult := ProbeRoot(session.root); tbResult.Found && tbMove != EmptyMove {
		uci.emitThreadReceipt(session, output, 1)
		// Syzygy tablebase hit - perfect endgame play
		uci.debugLogger.Printf("Syzygy tablebase hit! WDL=%d DTZ=%d Move=%s", tbResult.WDL, tbResult.DTZ, tbMove.ToString())
		if session.debugMode {
			fmt.Fprintf(output, "info string tablebase move: %s (WDL=%d)\n", tbMove.ToString(), tbResult.WDL)
		}

		bestMove = tbMove
		// Convert WDL to centipawn score
		switch tbResult.WDL {
		case WDL_Win:
			bestScore = MATE_VALUE - 100 - tbResult.DTZ // Winning
		case WDL_CursedWin:
			bestScore = 500 // Theoretically winning but 50-move draw
		case WDL_Draw:
			bestScore = 0
		case WDL_BlessedLoss:
			bestScore = -500 // Theoretically losing but 50-move draw
		case WDL_Loss:
			bestScore = -MATE_VALUE + 100 + tbResult.DTZ // Losing
		}

		uci.debugLogger.Printf("Using tablebase move: %v with score %d", bestMove.ToString(), bestScore)

		// Output the tablebase result
		var scoreStr string
		if bestScore >= MATE_IN_MAX {
			scoreStr = fmt.Sprintf("mate %d", (MATE_VALUE-bestScore+1)/2)
		} else if bestScore <= -MATE_IN_MAX {
			scoreStr = fmt.Sprintf("mate %d", -(MATE_VALUE+bestScore+1)/2)
		} else {
			scoreStr = fmt.Sprintf("cp %d", bestScore)
		}
		fmt.Fprintf(output, "info depth 1 score %s nodes 1 time 0 nps 0 tbhits 1 pv %s\n",
			scoreStr, bestMove.ToString())
	} else {
		uci.debugLogger.Printf("No opening book move found for current position")
		if session.debugMode {
			fmt.Fprintf(output, "info string no book move found, starting search\n")
		}

		// Perform iterative deepening search with callback for UCI info output
		uci.debugLogger.Printf("Starting SearchIterativeDeepeningWithCallback, maxDepth=%d", maxDepth)

		// Define callback to send UCI info after each depth completes
		depthCallback := func(searchInfo *SearchInfo) {
			depth := searchInfo.Depth
			score := searchInfo.BestScore
			move := searchInfo.BestMove
			nodes := searchInfo.Nodes

			// Suppress progress from an iteration that raced an explicit stop.
			if session.isCancelled() {
				uci.debugLogger.Printf("STOP COMMAND received during search")
				return
			}

			// Update best move and score
			bestMove = move
			bestScore = score
			lastIterationScore = score

			// Update time manager with score change
			session.timeManager.UpdateGameState(session.movesPlayed, lastIterationScore, score, session.root)

			// Send search info
			elapsed := session.timeManager.GetElapsedTime()

			// UCI: report mate scores as "score mate N" when applicable
			var scoreStr string
			if score == INFINITY || score == -INFINITY {
				scoreStr = "cp 0"
			} else if score >= MATE_IN_MAX {
				mate := (MATE_VALUE - score + 1) / 2
				scoreStr = fmt.Sprintf("mate %d", mate)
			} else if score <= -MATE_IN_MAX {
				// Mirror the winning branch (and the tablebase path at line ~610):
				// plies-to-mate is MATE_VALUE+score, reported as a negative "mate -N".
				// The old `-score - MATE_VALUE` underflowed for real losing mates and
				// clamped to the garbage "mate -501" seen 25k+ times in the logs.
				scoreStr = fmt.Sprintf("mate %d", -((MATE_VALUE + score + 1) / 2))
			} else {
				scoreStr = fmt.Sprintf("cp %d", score)
			}

			// Calculate NPS
			var nps int
			if elapsed.Seconds() > 0 {
				nps = int(float64(nodes) / elapsed.Seconds())
			}

			// Search copied reporting state before entering this non-reentrant callback.
			pvLength := searchInfo.PVLength
			if pvLength > 32 {
				pvLength = 32
			}
			var pvStr string
			if pvLength == 0 {
				pvStr = move.ToString()
			} else {
				pvStr = searchInfo.PV[0].ToString()
				for i := 1; i < pvLength; i++ {
					pvStr += " " + searchInfo.PV[i].ToString()
				}
			}

			// seldepth tracks the max ply reached including quiescence; floor at depth.
			seldepth := searchInfo.SelDepth
			if seldepth < depth {
				seldepth = depth
			}
			hashfull := searchInfo.Hashfull

			infoMsg := fmt.Sprintf("info depth %d seldepth %d score %s nodes %d time %d nps %d hashfull %d pv %s",
				depth, seldepth, scoreStr, nodes, elapsed.Milliseconds(), nps, hashfull, pvStr)
			uci.sendUCIMessage(output, infoMsg)
			uci.debugLogger.Printf("Sent info line for depth %d: score=%s bestMove=%s", depth, scoreStr, move.ToString())
		}

		// Run the search - this handles progressive TT building and aspiration windows internally
		info := uci.searcher.searchIterativeDeepeningPreparedWithWidth(session.root, maxDepth, session.timeManager, depthCallback, func(effective int) {
			uci.emitThreadReceipt(session, output, effective)
		})

		uci.debugLogger.Printf("SearchIterativeDeepeningWithCallback completed: depth=%d, bestMove=%v, bestScore=%d, nodes=%d",
			info.Depth, info.BestMove, info.BestScore, info.Nodes)

		// RT1: end-of-search counter dump (behind `debug on`). Surfaces the search
		// statistics already tracked in SearchInfo so pruning behaviour is visible in
		// real searches — e.g. watch nmcut to confirm null-move pruning actually fires.
		if session.debugMode {
			uci.sendUCIMessage(output, fmt.Sprintf(
				"info string stats depth %d seldepth %d nodes %d qnodes %d nmcut %d lmr %d lmp %d fut %d seeq %d ttcut %d tthits %d betacut %d checkext %d singext %d",
				info.Depth, info.SelDepth, info.Nodes, info.QNodes,
				info.NullMoveCutoffs, info.LMRReductions, info.LMPPrunes, info.FutilityPrunes, info.SEEQuietPrunes,
				info.TTCutoffs, info.TTHits, info.BetaCutoffs, info.CheckExtensions, info.SingularExtensions))
			// EBF-attribution dump (docs/11 §3). The `stats` line above is a frozen
			// contract (nodecheck.sh seds it); these sd_* lines are additive and never
			// contain a bare token named "nodes".
			uci.sendUCIMessage(output, fmt.Sprintf(
				"info string sd_order mln %d ttlist %d bcut %d fmc %d cidx %d/%d/%d/%d/%d ctried %d cuttt %d cutcap %d cutpromo %d cutkill %d cutcntr %d cuthist %d",
				info.MoveLoopNodes, info.TTMoveListed, info.BetaCutoffs, info.FirstMoveCutoffs,
				info.CutIdxHist[0], info.CutIdxHist[1], info.CutIdxHist[2], info.CutIdxHist[3], info.CutIdxHist[4],
				info.CutTriedSum, info.CutByTT, info.CutByCapture, info.CutByPromo, info.CutByKiller, info.CutByCounter, info.CutByQuietHist))
			// Class of the cutting move among NON-first cutoffs (ordering misses),
			// split move2/move3/move4+ — says which class deserved to be ordered first.
			uci.sendUCIMessage(output, fmt.Sprintf(
				"info string sd_miss m2/m3/m4+ tt %d/%d/%d cap %d/%d/%d promo %d/%d/%d kill %d/%d/%d cntr %d/%d/%d qh %d/%d/%d",
				info.CutMissByClass[0][0], info.CutMissByClass[0][1], info.CutMissByClass[0][2],
				info.CutMissByClass[1][0], info.CutMissByClass[1][1], info.CutMissByClass[1][2],
				info.CutMissByClass[2][0], info.CutMissByClass[2][1], info.CutMissByClass[2][2],
				info.CutMissByClass[3][0], info.CutMissByClass[3][1], info.CutMissByClass[3][2],
				info.CutMissByClass[4][0], info.CutMissByClass[4][1], info.CutMissByClass[4][2],
				info.CutMissByClass[5][0], info.CutMissByClass[5][1], info.CutMissByClass[5][2]))
			uci.sendUCIMessage(output, fmt.Sprintf(
				"info string sd_band fmc/bcut d12 %d/%d d35 %d/%d d69 %d/%d d10 %d/%d",
				info.FmcBand[0], info.BcutBand[0], info.FmcBand[1], info.BcutBand[1],
				info.FmcBand[2], info.BcutBand[2], info.FmcBand[3], info.BcutBand[3]))
			uci.sendUCIMessage(output, fmt.Sprintf(
				"info string sd_prune rfp %d nmtry %d nmcut %d pcut %d fut %d lmp %d seecap %d seeq %d histp %d alln %d alltried %d pvn %d",
				info.RFPPrunes, info.NullMoveTries, info.NullMoveCutoffs, info.ProbcutPrunes,
				info.FutilityPrunes, info.LMPPrunes, info.SEECapPrunes, info.SEEQuietPrunes, info.HistPrunes,
				info.AllNodes, info.AllTriedSum, info.PVNodesExact))
			uci.sendUCIMessage(output, fmt.Sprintf(
				"info string sd_lmr scout %d red %d redplies %d lmrre %d pvre %d",
				info.NullWindowScouts, info.LMRReductions, info.LMRPliesSum, info.LMRReSearches, info.PVReSearches))
			uci.sendUCIMessage(output, fmt.Sprintf(
				"info string sd_q qn %d qcap %d qttp %d qtthit %d qttcut %d qstand %d qdelta %d qsee %d qbcut %d qstore %d",
				info.QNodes, info.QDepthCapHits, info.QTTProbes, info.QTTHits, info.QTTCutoffs,
				info.QStandPatCuts, info.QDeltaPrunes, info.QSEEPrunes, info.QBetaCutoffs, info.QSearchTTStores))
			uci.sendUCIMessage(output, fmt.Sprintf(
				"info string sd_root aspfl %d aspfh %d aspab %d bmchg %d iid %d singtry %d singok %d extclamp %d chk %d recap %d pp %d",
				info.AspFailLows, info.AspFailHighs, info.AspAbandoned, info.RootBestMoveChanges,
				info.IIDSearches, info.SingularTries, info.SingularExtensions, info.ExtBudgetClamps,
				info.CheckExtensions, info.RecaptureExtensions, info.PassedPawnExtensions))
		}

		// Use final result from search
		if info.BestMove != EmptyMove {
			bestMove = info.BestMove
			bestScore = info.BestScore
		}

		uci.debugLogger.Printf("=== SEARCH COMPLETE ===")
	}

	uci.publishSearchResult(session, rawOutput, bestMove, bestScore)
	uci.debugLogger.Printf("=== EXITING search() FUNCTION ===")
}

func (uci *UCIEngine) publishSearchFailure(session *uciSearchSession, output io.Writer, recovered any) {
	session.resultOnce.Do(func() {
		session.outputMu.Lock()
		defer session.outputMu.Unlock()
		if session.suppress.Load() {
			return
		}
		uci.sendUCIMessage(output, fmt.Sprintf("info string error search session failed: %v", recovered))
		uci.commitAndSendBestMoveLocked(session, output, session.fallback, 0)
	})
}

func (uci *UCIEngine) publishSearchResult(session *uciSearchSession, output io.Writer, bestMove Move, bestScore int) {
	session.resultOnce.Do(func() {
		session.outputMu.Lock()
		defer session.outputMu.Unlock()
		if session.suppress.Load() {
			return
		}
		uci.commitAndSendBestMoveLocked(session, output, bestMove, bestScore)
	})
}

// commitAndSendBestMoveLocked validates against the legal root snapshot, commits
// session-local game state, and writes exactly one final result. outputMu is held.
func (uci *UCIEngine) commitAndSendBestMoveLocked(session *uciSearchSession, output io.Writer, bestMove Move, bestScore int) {
	isLegal := false
	for _, move := range session.legalMoves {
		if bestMove == move {
			isLegal = true
			break
		}
	}
	if bestMove != EmptyMove && !isLegal {
		uci.debugLogger.Printf("search returned illegal move %s; using legal fallback", bestMove.ToString())
		bestMove = session.fallback
	}
	if bestMove == EmptyMove && !session.allowResignation {
		bestMove = session.fallback
	}

	uci.lifecycleMu.Lock()
	if uci.activeSearch == session {
		uci.lastScore = bestScore
		uci.movesPlayed = session.movesPlayed + 1
		uci.timeManager = session.timeManager
	}
	uci.lifecycleMu.Unlock()

	if bestMove == EmptyMove {
		uci.sendUCIMessage(output, "bestmove (none)")
		return
	}
	uci.sendUCIMessage(output, fmt.Sprintf("bestmove %s", bestMove.ToString()))
}

// handleStop is the explicit UCI stop: it joins and retains the session's
// obligation to emit exactly one bestmove.
func (uci *UCIEngine) handleStop(_ io.Writer) {
	uci.joinSearch(false)
}

// joinSearch cancels and joins without holding lifecycleMu. Replacement, EOF,
// and quit suppress future output before cancellation. A write already in
// progress may finish; join guarantees no output after the replacement mutates
// state or returns.
func (uci *UCIEngine) joinSearch(suppress bool) {
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		return
	}
	uci.stopSearchSession(session, suppress)
	<-session.done
}

// stopSearchSession applies cancellation only while session is still the active
// identity. Suppression and RequestStop are published under lifecycleMu, so a
// delayed join for an old session can never stop a newly admitted search.
func (uci *UCIEngine) stopSearchSession(session *uciSearchSession, suppress bool) bool {
	uci.lifecycleMu.Lock()
	defer uci.lifecycleMu.Unlock()
	if uci.activeSearch != session {
		return false
	}
	uci.searchState = uciSearchStopping
	if suppress {
		session.suppressOutput()
	}
	session.cancel(uci.searcher)
	return true
}

// handlePonderHit handles ponderhit command
func (uci *UCIEngine) handlePonderHit(output io.Writer) {
	// TODO: Implement pondering support
}

// handleQuit handles the quit command
func (uci *UCIEngine) handleQuit(output io.Writer) {
	uci.joinSearch(true)
	// The main loop should exit after this
}
