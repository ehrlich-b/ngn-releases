package engine

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func newThreadsTestUCI(t *testing.T) *UCIEngine {
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
	t.Cleanup(func() { uci.joinSearch(true) })
	return uci
}

func TestUCIThreadsOptionHasSynchronousPreflightReceipt(t *testing.T) {
	uci := newThreadsTestUCI(t)
	var output bytes.Buffer
	uci.handleSetOption([]string{"name", "Threads", "value", "2"}, &output)
	uci.handleIsReady(&output)
	want := "info string threads configured 2 effective 2\nreadyok\n"
	if output.String() != want || uci.searcher.ThreadCount() != 2 {
		t.Fatalf("Threads preflight output/count=%q/%d, want %q/2", output.String(), uci.searcher.ThreadCount(), want)
	}

	output.Reset()
	uci.handleSetOption([]string{"name", "Threads", "value", "0"}, &output)
	if output.Len() != 0 || uci.searcher.ThreadCount() != 2 {
		t.Fatalf("invalid Threads changed output/count=%q/%d", output.String(), uci.searcher.ThreadCount())
	}
}

func TestUCIAcceptedGoReportsConfiguredAndEffectiveThreads(t *testing.T) {
	for _, test := range []struct {
		name      string
		args      []string
		effective int
	}{
		{"depth", []string{"depth", "1"}, 2},
		{"nodes", []string{"nodes", "64"}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			uci := newThreadsTestUCI(t)
			if err := uci.searcher.ConfigureThreads(2); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			uci.handleGo(test.args, &output)
			uci.lifecycleMu.Lock()
			session := uci.activeSearch
			uci.lifecycleMu.Unlock()
			if session == nil {
				t.Fatal("go did not publish a session")
			}
			waitUCIChannel(t, session.done, "Threads go completion")
			line := "info string threads configured 2 effective " + string(rune('0'+test.effective))
			if strings.Count(output.String(), line+"\n") != 1 {
				t.Fatalf("effective diagnostic count/output:\n%s", output.String())
			}
			if !strings.Contains(output.String(), "bestmove ") {
				t.Fatalf("Threads go omitted bestmove:\n%s", output.String())
			}
		})
	}
}

func TestUCIDuplicateGoDoesNotEmitSecondThreadsReceipt(t *testing.T) {
	uci := newThreadsTestUCI(t)
	if err := uci.searcher.ConfigureThreads(2); err != nil {
		t.Fatal(err)
	}
	entered, blockSetup, releaseSetup := newUCISetupGate(t)
	uci.copyPosition = func(pos *Position) *Position {
		blockSetup()
		return pos.Copy()
	}
	var output bytes.Buffer
	uci.handleGo([]string{"depth", "4"}, &output)
	waitUCIChannel(t, entered, "Threads duplicate-go setup")
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		t.Fatal("missing active search")
	}
	uci.handleGo([]string{"depth", "1"}, &output)
	stopDone := make(chan struct{})
	go func() {
		uci.handleStop(io.Discard)
		close(stopDone)
	}()
	waitUCIChannel(t, session.cancelled, "duplicate-go cancellation")
	releaseSetup()
	waitUCIChannel(t, stopDone, "duplicate-go join")
	line := "info string threads configured 2 effective 1\n"
	if strings.Count(output.String(), line) != 1 {
		t.Fatalf("duplicate go emitted unexpected Threads receipts:\n%s", output.String())
	}
}

func TestUCIThreadsReconfigurationCancelsAndJoinsPreparedSession(t *testing.T) {
	uci := newThreadsTestUCI(t)
	if err := uci.searcher.ConfigureThreads(2); err != nil {
		t.Fatal(err)
	}
	entered, blockSetup, releaseSetup := newUCISetupGate(t)
	uci.copyPosition = func(pos *Position) *Position {
		blockSetup()
		return pos.Copy()
	}
	var searchOutput bytes.Buffer
	uci.handleGo([]string{"depth", "8"}, &searchOutput)
	waitUCIChannel(t, entered, "Threads reconfigure setup")
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		t.Fatal("missing prepared session")
	}
	var optionOutput bytes.Buffer
	done := make(chan struct{})
	go func() {
		uci.handleSetOption([]string{"name", "Threads", "value", "3"}, &optionOutput)
		close(done)
	}()
	waitUCIChannel(t, session.cancelled, "Threads reconfigure cancellation")
	releaseSetup()
	waitUCIChannel(t, done, "Threads reconfigure join")
	if uci.searcher.ThreadCount() != 3 || optionOutput.String() != "info string threads configured 3 effective 3\n" {
		t.Fatalf("reconfigure count/output=%d/%q", uci.searcher.ThreadCount(), optionOutput.String())
	}
	if strings.Contains(searchOutput.String(), "bestmove ") {
		t.Fatalf("replacement configuration leaked suppressed bestmove:\n%s", searchOutput.String())
	}
}

func TestUCIThreadsOneKeepsOrdinaryGoDiagnosticFree(t *testing.T) {
	uci := newThreadsTestUCI(t)
	var output bytes.Buffer
	uci.handleGo([]string{"depth", "1"}, &output)
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		t.Fatal("missing session")
	}
	waitUCIChannel(t, session.done, "Threads=1 completion")
	if strings.Contains(output.String(), "info string threads configured") {
		t.Fatalf("default Threads=1 changed ordinary go output:\n%s", output.String())
	}
}

func TestUCIThreadsQuitJoinsActiveHelpers(t *testing.T) {
	uci := newThreadsTestUCI(t)
	if err := uci.searcher.ConfigureThreads(2); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	uci.handleGo([]string{"infinite"}, &output)
	deadline := time.After(5 * time.Second)
	for {
		uci.lifecycleMu.Lock()
		session := uci.activeSearch
		state := uci.searchState
		uci.lifecycleMu.Unlock()
		if session != nil && state == uciSearchRunning {
			break
		}
		select {
		case <-deadline:
			t.Fatal("Threads search did not reach running state")
		default:
		}
	}
	uci.handleQuit(io.Discard)
	if uci.isSearching() {
		t.Fatal("quit returned before active helper session joined")
	}
}

func TestUCIThreadsStopEmitsExactlyOneBestMove(t *testing.T) {
	uci := newThreadsTestUCI(t)
	if err := uci.searcher.ConfigureThreads(2); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	uci.handleGo([]string{"infinite"}, &output)
	deadline := time.After(5 * time.Second)
	for {
		uci.lifecycleMu.Lock()
		session := uci.activeSearch
		state := uci.searchState
		uci.lifecycleMu.Unlock()
		if session != nil && state == uciSearchRunning {
			break
		}
		select {
		case <-deadline:
			t.Fatal("Threads search did not reach running state")
		default:
		}
	}
	uci.handleStop(io.Discard)
	if got := strings.Count(output.String(), "bestmove "); got != 1 {
		t.Fatalf("explicit stop emitted %d bestmoves:\n%s", got, output.String())
	}
}

func TestUCIThreadsEOFJoinsSession(t *testing.T) {
	uci := newThreadsTestUCI(t)
	var output bytes.Buffer
	input := strings.NewReader("setoption name Threads value 2\nposition fen 7k/8/8/8/3Q4/8/8/K7 w - - 0 1\ngo infinite\n")
	uci.Run(input, &output)
	if uci.isSearching() {
		t.Fatal("Run returned on EOF before Threads session joined")
	}
	// EOF suppression may win before the coordinator reaches its per-go write;
	// the synchronous configuration receipt remains mandatory.
	if got := strings.Count(output.String(), "info string threads configured 2 effective 2\n"); got != 1 {
		t.Fatalf("EOF-visible Threads receipts=%d, want configuration-only 1:\n%s", got, output.String())
	}
}

func TestUCIThreadsDiagnosticPanicCompletesWithLegalFallback(t *testing.T) {
	uci := newThreadsTestUCI(t)
	if err := uci.searcher.ConfigureThreads(2); err != nil {
		t.Fatal(err)
	}
	output := &panicFirstSetupWriter{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(output.unblock)
	uci.handleGo([]string{"depth", "2"}, output)
	waitUCIChannel(t, output.entered, "Threads diagnostic panic")
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil || session.fallback == EmptyMove {
		t.Fatal("Threads diagnostic ran before session publication and legal fallback")
	}
	want := session.fallback.ToString()
	output.unblock()
	waitUCIChannel(t, session.done, "Threads diagnostic panic completion")
	got := output.buf.String()
	if !strings.Contains(got, "info string error search session failed: injected pre-search output failure") ||
		strings.Count(got, "bestmove ") != 1 || !strings.Contains(got, "bestmove "+want) {
		t.Fatalf("Threads diagnostic panic lost error/fallback:\n%s", got)
	}
}

func TestUCIThreadsReceiptUsesCancelledSetupWidth(t *testing.T) {
	uci := newThreadsTestUCI(t)
	if err := uci.searcher.ConfigureThreads(3); err != nil {
		t.Fatal(err)
	}
	uci.searcher.smpHooks = &smpSearchHooks{afterRootCopy: func(worker int) {
		if worker == 1 {
			uci.searcher.RequestStop()
		}
	}}
	var output bytes.Buffer
	uci.handleGo([]string{"depth", "4"}, &output)
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		t.Fatal("missing cancelled-setup session")
	}
	waitUCIChannel(t, session.done, "cancelled setup Threads receipt")
	if strings.Count(output.String(), "info string threads configured 3 effective 1\n") != 1 ||
		strings.Contains(output.String(), "info string threads configured 3 effective 3") {
		t.Fatalf("cancelled setup reported dishonest width:\n%s", output.String())
	}
}
