package engine

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// runUCI drives the full UCI loop (the production Run entry point) with an
// in-memory command stream and returns everything the engine wrote. It reads the
// buffer only AFTER Run returns, which — because Run joins the search goroutine on
// exit — is race-clean (verify with `go test -race`). A hung loop (quit failing to
// terminate) trips the timeout.
func runUCI(t *testing.T, commands string) string {
	t.Helper()
	engine := NewUCIEngine()
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		engine.Run(strings.NewReader(commands), &buf)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not return within 15s — quit/EOF failed to terminate the loop")
	}
	return buf.String()
}

// TestUCIRunQuitTerminatesLoop proves `quit` breaks the scanner loop rather than
// relying on input EOF. The command AFTER quit must NOT be processed: if the loop
// kept scanning it would handle the trailing "uci" and emit an id banner.
func TestUCIRunQuitTerminatesLoop(t *testing.T) {
	out := runUCI(t, "isready\nquit\nuci\n")
	if !strings.Contains(out, "readyok") {
		t.Errorf("expected readyok from the pre-quit isready, got:\n%s", out)
	}
	if strings.Contains(out, "id name") {
		t.Errorf("a command after quit was processed — quit did not terminate the loop:\n%s", out)
	}
}

// TestUCIRunImmediateStop: stop right after go must produce a bestmove and be
// race-clean (the join in handleStop guarantees the searcher is done).
func TestUCIRunImmediateStop(t *testing.T) {
	out := runUCI(t, "uci\nposition startpos\ngo depth 30\nstop\nquit\n")
	if !strings.Contains(out, "uciok") {
		t.Errorf("missing uciok:\n%s", out)
	}
	if n := strings.Count(out, "bestmove "); n != 1 {
		t.Errorf("stop after go produced %d bestmoves, want exactly one:\n%s", n, out)
	}
}

// TestUCIRunRepeatedGo: a second go while the first search is still running must
// not spawn a concurrent search (the active session rejects it) and must not race.
func TestUCIRunRepeatedGo(t *testing.T) {
	out := runUCI(t, "position startpos\ngo depth 30\ngo depth 30\nstop\nquit\n")
	if n := strings.Count(out, "bestmove"); n < 1 {
		t.Errorf("expected at least one bestmove, got %d:\n%s", n, out)
	}
}

// TestUCIRunPositionAfterStop: changing the position after stop must be race-clean
// — handleStop joins the search goroutine before the loop mutates uci.position, so
// the searcher can never read a position pointer being swapped underneath it.
func TestUCIRunPositionAfterStop(t *testing.T) {
	out := runUCI(t, "position startpos\ngo depth 30\nstop\nposition startpos moves e2e4 e7e5\ngo depth 4\nstop\nquit\n")
	if n := strings.Count(out, "bestmove "); n != 2 {
		t.Errorf("expected exactly two bestmoves (one per search), got %d:\n%s", n, out)
	}
}

func TestUCIPreparedSessionStopSurvivesUntilSearchEntry(t *testing.T) {
	uci := NewUCIEngine()
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	uci.searcher = searcher
	uci.position, err = ParseFEN("7k/8/8/8/3Q4/8/8/K7 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	params := SearchParams{Depth: 1}
	session, accepted := uci.beginSearchSession(time.Now(), params)
	if !accepted {
		t.Fatal("first search session was rejected")
	}
	t.Cleanup(func() {
		session.cancel(uci.searcher)
		uci.finishSearchSession(session)
	})
	stopDone := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		go func() {
			uci.handleStop(io.Discard)
			stopDone <- struct{}{}
		}()
	}
	select {
	case <-session.cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not cancel the published prepared session")
	}
	if !uci.searcher.StopRequested() {
		t.Fatal("prepared session cancellation did not reach search control")
	}
	uci.lifecycleMu.Lock()
	stoppingState, stoppingActive := uci.searchState, uci.activeSearch
	uci.lifecycleMu.Unlock()
	if stoppingState != uciSearchStopping || stoppingActive != session {
		t.Fatalf("prepared stop state=%d active=%p, want stopping/%p", stoppingState, stoppingActive, session)
	}

	// A duplicate go must be rejected without clearing the stop already attached
	// to the published session.
	uci.handleGo([]string{"depth", "1"}, io.Discard)
	if !uci.searcher.StopRequested() {
		t.Fatal("rejected duplicate go cleared the active session stop")
	}

	var output bytes.Buffer
	go uci.runSearchSession(session, &output)
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for i := 0; i < 2; i++ {
		select {
		case <-stopDone:
		case <-deadline.C:
			t.Fatal("idempotent stop callers did not join the prepared search after launch")
		}
	}
	if n := strings.Count(output.String(), "bestmove "); n != 1 {
		t.Fatalf("prepared-stop search produced %d bestmoves, want exactly one:\n%s", n, output.String())
	}
	if uci.isSearching() {
		t.Fatal("joined prepared-stop search remained active")
	}
	uci.lifecycleMu.Lock()
	state, active := uci.searchState, uci.activeSearch
	uci.lifecycleMu.Unlock()
	if state != uciSearchIdle || active != nil {
		t.Fatalf("completed lifecycle state=%d active=%p, want idle/nil", state, active)
	}
}

type blockingBestmoveWriter struct {
	entered     chan struct{}
	release     chan struct{}
	enterOnce   sync.Once
	releaseOnce sync.Once
	buf         bytes.Buffer
}

func (w *blockingBestmoveWriter) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("bestmove ")) {
		w.enterOnce.Do(func() {
			close(w.entered)
			<-w.release
		})
	}
	return w.buf.Write(p)
}

func (w *blockingBestmoveWriter) unblock() {
	w.releaseOnce.Do(func() { close(w.release) })
}

func TestUCICompletionFollowsFinalOutputWithoutHoldingLifecycleLock(t *testing.T) {
	uci := NewUCIEngine()
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	uci.searcher = searcher
	uci.position, err = ParseFEN("7k/8/8/8/3Q4/8/8/K7 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	output := &blockingBestmoveWriter{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() {
		output.unblock()
		uci.searcher.RequestStop()
	})
	uci.handleGo([]string{"depth", "1"}, output)

	select {
	case <-output.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("search did not reach final bestmove write")
	}
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		t.Fatal("session completed while its final output was blocked")
	}
	stopDone := make(chan struct{})
	go func() {
		uci.handleStop(io.Discard)
		close(stopDone)
	}()
	select {
	case <-session.cancelled:
	case <-time.After(5 * time.Second):
		output.unblock()
		t.Fatal("stop did not reach the output-blocked session")
	}
	select {
	case <-stopDone:
		output.unblock()
		t.Fatal("stop returned before the final output write completed")
	default:
	}
	if !uci.lifecycleMu.TryLock() {
		output.unblock()
		t.Fatal("final output or join held lifecycleMu while blocking")
	}
	uci.lifecycleMu.Unlock()

	output.unblock()
	select {
	case <-stopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not receive completion after final output")
	}
	if n := strings.Count(output.buf.String(), "bestmove "); n != 1 {
		t.Fatalf("output-blocked search produced %d bestmoves, want one:\n%s", n, output.buf.String())
	}
}

type uciFakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *uciFakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *uciFakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func waitUCIChannel(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// newUCISetupGate makes negative-path failures release a blocked coordinator.
// newLifecycleTestUCI's later cleanup then joins it before the test exits.
func newUCISetupGate(t *testing.T) (<-chan struct{}, func(), func()) {
	t.Helper()
	entered := make(chan struct{})
	release := make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	block := func() {
		enterOnce.Do(func() { close(entered) })
		<-release
	}
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	return entered, block, unblock
}

func newLifecycleTestUCI(t *testing.T, clock *uciFakeClock) *UCIEngine {
	t.Helper()
	uci := NewUCIEngine()
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	uci.searcher = searcher
	uci.position, err = ParseFEN("7k/8/8/8/3Q4/8/8/K7 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	uci.now = clock.Now
	uci.timeManager = newTimeManager(clock.Now)
	t.Cleanup(func() { uci.joinSearch(true) })
	return uci
}

func TestUCIGoReceiptClockIncludesCancellableSetup(t *testing.T) {
	clock := &uciFakeClock{now: time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)}
	uci := newLifecycleTestUCI(t, clock)
	entered, blockSetup, releaseSetup := newUCISetupGate(t)
	uci.copyPosition = func(pos *Position) *Position {
		blockSetup()
		return pos.Copy()
	}

	var output bytes.Buffer
	uci.handleGo([]string{"movetime", "100"}, &output)
	waitUCIChannel(t, entered, "position-copy setup")

	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		t.Fatal("go did not publish a prepared session before position copy")
	}
	if !session.timeManager.startTime.Equal(clock.Now()) {
		t.Fatalf("time origin=%v, want go receipt %v", session.timeManager.startTime, clock.Now())
	}
	if got := session.timeManager.GetAllocatedTime(); got != 50*time.Millisecond {
		t.Fatalf("allocated movetime=%v, want existing 100ms minus 50ms overhead", got)
	}

	// Setup consumes the budget. Advancing the fake clock beyond the existing
	// hard limit must select the already-established legal fallback without
	// entering search or allocating the lazy TT.
	clock.Advance(60 * time.Millisecond)
	releaseSetup()
	waitUCIChannel(t, session.done, "setup-expired completion")
	if uci.searcher.tt != nil {
		t.Fatal("setup-expired search entered receiver setup and allocated a TT")
	}
	if n := strings.Count(output.String(), "bestmove "); n != 1 {
		t.Fatalf("setup expiry produced %d bestmoves, want one legal fallback:\n%s", n, output.String())
	}
	if want := "bestmove " + session.fallback.ToString(); !strings.Contains(output.String(), want) {
		t.Fatalf("setup expiry omitted legal fallback %q:\n%s", want, output.String())
	}
}

func TestUCIExplicitStopDuringSetupPublishesOneFallback(t *testing.T) {
	clock := &uciFakeClock{now: time.Date(2026, 9, 6, 12, 1, 0, 0, time.UTC)}
	uci := newLifecycleTestUCI(t, clock)
	entered, blockSetup, releaseSetup := newUCISetupGate(t)
	uci.copyPosition = func(pos *Position) *Position {
		blockSetup()
		return pos.Copy()
	}

	var output bytes.Buffer
	uci.handleGo([]string{"depth", "30"}, &output)
	waitUCIChannel(t, entered, "blocked setup")
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		t.Fatal("missing prepared session")
	}
	stopDone := make(chan struct{})
	go func() {
		uci.handleStop(io.Discard)
		close(stopDone)
	}()
	waitUCIChannel(t, session.cancelled, "setup cancellation")
	select {
	case <-stopDone:
		t.Fatal("stop returned before blocked setup released and completion closed")
	default:
	}
	releaseSetup()
	waitUCIChannel(t, stopDone, "joined setup cancellation")
	if n := strings.Count(output.String(), "bestmove "); n != 1 {
		t.Fatalf("explicit setup stop produced %d bestmoves, want one:\n%s", n, output.String())
	}
	if want := "bestmove " + session.fallback.ToString(); !strings.Contains(output.String(), want) {
		t.Fatalf("explicit setup stop omitted fallback %q:\n%s", want, output.String())
	}
}

func TestUCIReplacementJoinsBeforeRootMutationAndSuppressesStaleOutput(t *testing.T) {
	clock := &uciFakeClock{now: time.Date(2026, 9, 6, 12, 2, 0, 0, time.UTC)}
	uci := newLifecycleTestUCI(t, clock)
	oldFEN := GenerateFEN(uci.position)
	entered, blockSetup, releaseSetup := newUCISetupGate(t)
	uci.copyPosition = func(pos *Position) *Position {
		blockSetup()
		return pos.Copy()
	}

	var staleOutput bytes.Buffer
	uci.handleGo([]string{"depth", "30"}, &staleOutput)
	waitUCIChannel(t, entered, "replacement-blocked setup")
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		t.Fatal("missing prepared session")
	}
	replaced := make(chan struct{})
	go func() {
		uci.handlePosition([]string{"startpos", "moves", "e2e4"}, io.Discard)
		close(replaced)
	}()
	waitUCIChannel(t, session.cancelled, "replacement cancellation")
	select {
	case <-replaced:
		t.Fatal("position replacement mutated root before setup joined")
	default:
	}
	if got := GenerateFEN(session.rootSource); got != oldFEN {
		t.Fatalf("session root changed before join:\n got %s\nwant %s", got, oldFEN)
	}
	releaseSetup()
	waitUCIChannel(t, replaced, "position replacement join")
	if strings.Contains(staleOutput.String(), "bestmove ") {
		t.Fatalf("replacement published a stale bestmove:\n%s", staleOutput.String())
	}
	if got := GenerateFEN(uci.position); got == oldFEN {
		t.Fatal("position command did not apply after joining the old session")
	}
}

func TestUCISearchSetupPanicReportsErrorAndLegalFallback(t *testing.T) {
	clock := &uciFakeClock{now: time.Date(2026, 9, 6, 12, 3, 0, 0, time.UTC)}
	uci := newLifecycleTestUCI(t, clock)
	entered, blockSetup, releaseSetup := newUCISetupGate(t)
	uci.copyPosition = func(*Position) *Position {
		blockSetup()
		panic("injected setup failure")
	}

	var output bytes.Buffer
	uci.handleGo([]string{"depth", "4"}, &output)
	waitUCIChannel(t, entered, "panic injection")
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		t.Fatal("missing prepared session")
	}
	releaseSetup()
	waitUCIChannel(t, session.done, "panic recovery completion")
	got := output.String()
	if !strings.Contains(got, "info string error search session failed: injected setup failure") {
		t.Fatalf("panic recovery omitted error evidence:\n%s", got)
	}
	if n := strings.Count(got, "bestmove "); n != 1 {
		t.Fatalf("panic recovery produced %d bestmoves, want one:\n%s", n, got)
	}
	if want := "bestmove " + session.fallback.ToString(); !strings.Contains(got, want) {
		t.Fatalf("panic recovery omitted legal fallback %q:\n%s", want, got)
	}
	hceModelLifecycle.mu.Lock()
	activeUses := hceModelLifecycle.activeUses
	hceModelLifecycle.mu.Unlock()
	if activeUses != 0 {
		t.Fatalf("setup panic leaked %d HCE model leases", activeUses)
	}
}

func TestUCIEOFSuppressesPreparedSearchOutput(t *testing.T) {
	clock := &uciFakeClock{now: time.Date(2026, 9, 6, 12, 4, 0, 0, time.UTC)}
	uci := newLifecycleTestUCI(t, clock)
	entered, blockSetup, releaseSetup := newUCISetupGate(t)
	uci.copyPosition = func(pos *Position) *Position {
		blockSetup()
		return pos.Copy()
	}

	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = writer.Close()
		_ = reader.Close()
	})
	var output bytes.Buffer
	runDone := make(chan struct{})
	go func() {
		uci.Run(reader, &output)
		close(runDone)
	}()
	if _, err := io.WriteString(writer, "go depth 30\n"); err != nil {
		t.Fatal(err)
	}
	waitUCIChannel(t, entered, "EOF-blocked setup")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		t.Fatal("missing active search at EOF")
	}
	waitUCIChannel(t, session.cancelled, "EOF cancellation")
	releaseSetup()
	waitUCIChannel(t, runDone, "EOF join")
	if strings.Contains(output.String(), "bestmove ") {
		t.Fatalf("EOF published stale search output:\n%s", output.String())
	}
}

func TestUCITimePolicyStateCarriesAcrossPrivateSessionManagers(t *testing.T) {
	clock := &uciFakeClock{now: time.Date(2026, 9, 6, 12, 5, 0, 0, time.UTC)}
	uci := newLifecycleTestUCI(t, clock)
	uci.movesPlayed = 4
	params := SearchParams{WhiteTime: 60_000, BlackTime: 60_000}
	expected := newTimeManager(clock.Now)

	runSetupOnly := func(position *Position, movesPlayed int) *uciSearchSession {
		t.Helper()
		uci.position = position
		uci.movesPlayed = movesPlayed
		receipt := clock.Now()
		expected.SetTimeControlAt(params, position.Turn() == White, receipt)
		wantSoft, wantHard := expected.softTime, expected.hardTime
		expected.UpdateGameState(movesPlayed, uci.lastScore, uci.lastScore, position)

		entered, blockSetup, releaseSetup := newUCISetupGate(t)
		uci.copyPosition = func(pos *Position) *Position {
			blockSetup()
			return pos.Copy()
		}
		var output bytes.Buffer
		uci.handleGo([]string{"wtime", "60000", "btime", "60000"}, &output)
		waitUCIChannel(t, entered, "policy-state setup")
		uci.lifecycleMu.Lock()
		session := uci.activeSearch
		uci.lifecycleMu.Unlock()
		if session == nil {
			t.Fatal("missing policy-state session")
		}
		if session.timeManager.softTime != wantSoft || session.timeManager.hardTime != wantHard {
			t.Fatalf("private policy budgets soft/hard=%v/%v, want legacy sequence %v/%v",
				session.timeManager.softTime, session.timeManager.hardTime, wantSoft, wantHard)
		}
		if !session.timeManager.startTime.Equal(receipt) {
			t.Fatalf("private manager start=%v, want receipt %v", session.timeManager.startTime, receipt)
		}
		clock.Advance(time.Minute)
		releaseSetup()
		waitUCIChannel(t, session.done, "policy-state setup completion")
		return session
	}

	endgame, err := ParseFEN("7k/8/8/8/3Q4/8/8/K7 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	first := runSetupOnly(endgame, 4)
	opening := newUCIStartingPosition()
	second := runSetupOnly(opening, 8)
	if first.timeManager.softTime == second.timeManager.softTime && first.timeManager.hardTime == second.timeManager.hardTime {
		t.Fatal("consecutive sessions lost the prior endgame/move-count policy state")
	}
}

func TestUCIQuitSuppressesPreparedSearchOutput(t *testing.T) {
	clock := &uciFakeClock{now: time.Date(2026, 9, 6, 12, 6, 0, 0, time.UTC)}
	uci := newLifecycleTestUCI(t, clock)
	entered, blockSetup, releaseSetup := newUCISetupGate(t)
	uci.copyPosition = func(pos *Position) *Position {
		blockSetup()
		return pos.Copy()
	}

	var output bytes.Buffer
	uci.handleGo([]string{"depth", "30"}, &output)
	waitUCIChannel(t, entered, "quit-blocked setup")
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		t.Fatal("missing active search before quit")
	}
	quitDone := make(chan struct{})
	go func() {
		uci.handleQuit(io.Discard)
		close(quitDone)
	}()
	waitUCIChannel(t, session.cancelled, "quit cancellation")
	releaseSetup()
	waitUCIChannel(t, quitDone, "quit join")
	if strings.Contains(output.String(), "bestmove ") {
		t.Fatalf("quit published stale search output:\n%s", output.String())
	}
}

func TestUCIIsReadyDuringSetupDoesNotJoinOrMutateSession(t *testing.T) {
	clock := &uciFakeClock{now: time.Date(2026, 9, 6, 12, 7, 0, 0, time.UTC)}
	uci := newLifecycleTestUCI(t, clock)
	entered, blockSetup, releaseSetup := newUCISetupGate(t)
	uci.copyPosition = func(pos *Position) *Position {
		blockSetup()
		return pos.Copy()
	}

	var searchOutput bytes.Buffer
	uci.handleGo([]string{"depth", "30"}, &searchOutput)
	waitUCIChannel(t, entered, "isready-blocked setup")
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	state := uci.searchState
	uci.lifecycleMu.Unlock()
	if session == nil || state != uciSearchPrepared {
		t.Fatalf("setup state=%d session=%p, want prepared/non-nil", state, session)
	}

	var readyOutput bytes.Buffer
	uci.handleIsReady(&readyOutput)
	if readyOutput.String() != "readyok\n" {
		t.Fatalf("isready during setup output=%q, want readyok", readyOutput.String())
	}
	uci.lifecycleMu.Lock()
	stillActive := uci.activeSearch
	stillState := uci.searchState
	uci.lifecycleMu.Unlock()
	if stillActive != session || stillState != uciSearchPrepared || session.isCancelled() {
		t.Fatalf("isready changed session: active=%p state=%d cancelled=%v", stillActive, stillState, session.isCancelled())
	}

	stopDone := make(chan struct{})
	go func() {
		uci.handleStop(io.Discard)
		close(stopDone)
	}()
	waitUCIChannel(t, session.cancelled, "isready-test cleanup cancellation")
	releaseSetup()
	waitUCIChannel(t, stopDone, "isready-test cleanup join")
}

func TestUCIStaleSessionStopCannotCancelNewSearch(t *testing.T) {
	uci := NewUCIEngine()
	first, accepted := uci.beginSearchSession(time.Now(), SearchParams{Depth: 1})
	if !accepted {
		t.Fatal("first session rejected")
	}
	uci.finishSearchSession(first)

	second, accepted := uci.beginSearchSession(time.Now(), SearchParams{Depth: 1})
	if !accepted {
		t.Fatal("second session rejected")
	}
	t.Cleanup(func() { uci.finishSearchSession(second) })
	if uci.stopSearchSession(first, true) {
		t.Fatal("stale first-session cancellation was accepted after second admission")
	}
	if uci.searcher.StopRequested() {
		t.Fatal("stale first-session cancellation stopped the new search control")
	}
	uci.lifecycleMu.Lock()
	active, state := uci.activeSearch, uci.searchState
	uci.lifecycleMu.Unlock()
	if active != second || state != uciSearchPrepared {
		t.Fatalf("stale cancellation changed new session: active=%p state=%d", active, state)
	}
}

type panicFirstSetupWriter struct {
	entered     chan struct{}
	release     chan struct{}
	panicOnce   sync.Once
	releaseOnce sync.Once
	buf         bytes.Buffer
}

func (w *panicFirstSetupWriter) Write(p []byte) (int, error) {
	w.panicOnce.Do(func() {
		close(w.entered)
		<-w.release
		panic("injected pre-search output failure")
	})
	return w.buf.Write(p)
}

func (w *panicFirstSetupWriter) unblock() {
	w.releaseOnce.Do(func() { close(w.release) })
}

func TestUCIPostPublicationDebugPanicCompletesWithFallback(t *testing.T) {
	clock := &uciFakeClock{now: time.Date(2026, 9, 6, 12, 8, 0, 0, time.UTC)}
	uci := newLifecycleTestUCI(t, clock)
	uci.debugMode = true
	output := &panicFirstSetupWriter{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(output.unblock)

	uci.handleGo([]string{"depth", "4"}, output)
	waitUCIChannel(t, output.entered, "post-publication debug output")
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		t.Fatal("debug output ran before session publication")
	}
	if session.fallback == EmptyMove {
		t.Fatal("debug output ran before legal fallback establishment")
	}
	output.unblock()
	waitUCIChannel(t, session.done, "post-publication panic completion")
	got := output.buf.String()
	if !strings.Contains(got, "info string error search session failed: injected pre-search output failure") {
		t.Fatalf("post-publication panic omitted error evidence:\n%s", got)
	}
	if n := strings.Count(got, "bestmove "); n != 1 {
		t.Fatalf("post-publication panic produced %d bestmoves, want one:\n%s", n, got)
	}
	if want := "bestmove " + session.fallback.ToString(); !strings.Contains(got, want) {
		t.Fatalf("post-publication panic omitted legal fallback %q:\n%s", want, got)
	}
}

func TestUCIMoveOverheadUpdateJoinsPreparedSession(t *testing.T) {
	clock := &uciFakeClock{now: time.Date(2026, 9, 6, 15, 0, 0, 0, time.UTC)}
	uci := newLifecycleTestUCI(t, clock)
	entered, blockSetup, releaseSetup := newUCISetupGate(t)
	uci.copyPosition = func(pos *Position) *Position {
		blockSetup()
		return pos.Copy()
	}

	var searchOutput bytes.Buffer
	uci.handleGo([]string{"depth", "1"}, &searchOutput)
	waitUCIChannel(t, entered, "prepared search setup")

	setDone := make(chan struct{})
	go func() {
		uci.handleSetOption([]string{"name", "Move", "Overhead", "value", "325"}, io.Discard)
		close(setDone)
	}()
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		releaseSetup()
		t.Fatal("prepared session disappeared before option update joined it")
	}
	waitUCIChannel(t, session.cancelled, "option-triggered prepared cancellation")
	select {
	case <-setDone:
		releaseSetup()
		t.Fatal("Move Overhead update returned before prepared setup joined")
	default:
	}

	releaseSetup()
	waitUCIChannel(t, setDone, "Move Overhead option update")
	if got := uci.timeManager.moveOverhead; got != 325*time.Millisecond {
		t.Fatalf("configured move overhead=%v, want 325ms", got)
	}
	if n := strings.Count(searchOutput.String(), "bestmove "); n != 0 {
		t.Fatalf("reconfiguration emitted %d stale bestmoves:\n%s", n, searchOutput.String())
	}
}
