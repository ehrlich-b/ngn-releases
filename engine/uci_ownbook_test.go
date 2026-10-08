package engine

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

func waitOwnBookSearchDone(t *testing.T, uci *UCIEngine) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for uci.isSearching() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if uci.isSearching() {
		t.Fatal("timed out waiting for OwnBook search")
	}
}

func requireRealDepthTwoSearch(t *testing.T, output string) {
	t.Helper()
	if !strings.Contains(output, "info depth 2 ") {
		t.Fatalf("OwnBook=false did not complete a real depth-2 search:\n%s", output)
	}
	if strings.Contains(output, "info depth 1 score cp 50 nodes 1 time 0 nps 0 pv ") {
		t.Fatalf("OwnBook=false emitted the fixed embedded-book signature:\n%s", output)
	}
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "info depth 2 ") {
			continue
		}
		fields := strings.Fields(line)
		for i := range fields {
			if fields[i] == "nodes" && i+1 < len(fields) {
				nodes, err := strconv.Atoi(fields[i+1])
				if err != nil {
					t.Fatalf("invalid depth-2 nodes field %q: %v", fields[i+1], err)
				}
				if nodes <= 1 {
					t.Fatalf("real depth-2 search reported %d nodes, want >1:\n%s", nodes, output)
				}
				return
			}
		}
		t.Fatalf("depth-2 info omitted nodes:\n%s", output)
	}
	t.Fatalf("missing depth-2 info:\n%s", output)
}

func TestUCIOwnBookOptionAdvertisedDefaultFalse(t *testing.T) {
	uci := NewUCIEngine()
	var output bytes.Buffer
	uci.handleCommand("uci", &output)
	if !strings.Contains(output.String(), "option name OwnBook type check default false\n") {
		t.Fatalf("uci response omitted OwnBook default:\n%s", output.String())
	}
	uci.lifecycleMu.Lock()
	got := uci.ownBook
	uci.lifecycleMu.Unlock()
	if got {
		t.Fatal("new UCI engine did not default OwnBook to false")
	}
}

func TestUCIOwnBookFalseBypassesEmbeddedBookAndPersistsNewGame(t *testing.T) {
	withBook := NewUCIEngine()
	withBook.ConfigureStartupOwnBook(true)
	var bookOutput bytes.Buffer
	withBook.handleCommand("position startpos", io.Discard)
	withBook.handleCommand("go depth 2", &bookOutput)
	waitOwnBookSearchDone(t, withBook)
	if !strings.Contains(bookOutput.String(), "info depth 1 score cp 50 nodes 1 time 0 nps 0 pv e2e4\n") {
		t.Fatalf("explicit OwnBook=true missed embedded startpos witness:\n%s", bookOutput.String())
	}

	withoutBook := NewUCIEngine()
	withoutBook.handleCommand("setoption name OwnBook value false", io.Discard)
	withoutBook.handleCommand("ucinewgame", io.Discard)
	withoutBook.lifecycleMu.Lock()
	if withoutBook.ownBook {
		withoutBook.lifecycleMu.Unlock()
		t.Fatal("ucinewgame reset OwnBook=false")
	}
	withoutBook.lifecycleMu.Unlock()

	var searchOutput bytes.Buffer
	withoutBook.handleCommand("position startpos", io.Discard)
	withoutBook.handleCommand("go depth 2", &searchOutput)
	waitOwnBookSearchDone(t, withoutBook)
	requireRealDepthTwoSearch(t, searchOutput.String())
	if !strings.Contains(searchOutput.String(), "bestmove ") {
		t.Fatalf("OwnBook=false search omitted bestmove:\n%s", searchOutput.String())
	}
}

func TestUCIOwnBookInvalidValuePreservesState(t *testing.T) {
	uci := NewUCIEngine()
	uci.handleCommand("setoption name OwnBook value false", io.Discard)
	uci.handleCommand("setoption name OwnBook value 1", io.Discard)
	uci.lifecycleMu.Lock()
	got := uci.ownBook
	uci.lifecycleMu.Unlock()
	if got {
		t.Fatal("invalid OwnBook value changed false state")
	}

	uci.handleCommand("setoption name OwnBook value true", io.Discard)
	uci.handleCommand("setoption name OwnBook value maybe", io.Discard)
	uci.lifecycleMu.Lock()
	got = uci.ownBook
	uci.lifecycleMu.Unlock()
	if !got {
		t.Fatal("invalid OwnBook value changed true state")
	}
}

func TestUCIOwnBookReplacementJoinsPreparedSearchAndSuppressesOutput(t *testing.T) {
	clock := &uciFakeClock{now: time.Date(2026, 9, 6, 15, 0, 0, 0, time.UTC)}
	uci := newLifecycleTestUCI(t, clock)
	uci.ConfigureStartupOwnBook(true)
	entered, blockSetup, releaseSetup := newUCISetupGate(t)
	uci.copyPosition = func(pos *Position) *Position {
		blockSetup()
		return pos.Copy()
	}

	var staleOutput bytes.Buffer
	uci.handleGo([]string{"depth", "30"}, &staleOutput)
	waitUCIChannel(t, entered, "OwnBook prepared setup")
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil || !session.ownBook {
		t.Fatal("prepared session did not snapshot explicit OwnBook=true")
	}

	setDone := make(chan struct{})
	go func() {
		uci.handleCommand("setoption name OwnBook value false", io.Discard)
		close(setDone)
	}()
	waitUCIChannel(t, session.cancelled, "OwnBook prepared-search cancellation")
	select {
	case <-setDone:
		releaseSetup()
		t.Fatal("OwnBook replacement returned before prepared search joined")
	default:
	}
	releaseSetup()
	waitUCIChannel(t, setDone, "OwnBook prepared-search replacement")
	if strings.Contains(staleOutput.String(), "bestmove ") {
		t.Fatalf("OwnBook replacement published stale prepared bestmove:\n%s", staleOutput.String())
	}
	uci.lifecycleMu.Lock()
	got := uci.ownBook
	uci.lifecycleMu.Unlock()
	if got {
		t.Fatal("OwnBook=false was not applied after prepared search joined")
	}
}

func TestUCIOwnBookReplacementJoinsRunningSearchAndSuppressesOutput(t *testing.T) {
	uci := NewUCIEngine()
	uci.ConfigureStartupOwnBook(true)
	t.Cleanup(func() { uci.joinSearch(true) })
	uci.handleCommand("position fen r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 0 1", io.Discard)

	var staleOutput bytes.Buffer
	uci.handleGo([]string{"depth", "40"}, &staleOutput)
	session := waitUCIRunning(t, uci)
	uci.handleCommand("setoption name OwnBook value false", io.Discard)
	select {
	case <-session.done:
	default:
		t.Fatal("OwnBook replacement returned before running search joined")
	}
	if strings.Contains(staleOutput.String(), "bestmove ") {
		t.Fatalf("OwnBook replacement published stale running bestmove:\n%s", staleOutput.String())
	}
	sizeAtReturn := staleOutput.Len()
	time.Sleep(10 * time.Millisecond)
	if staleOutput.Len() != sizeAtReturn {
		t.Fatal("joined running search emitted output after OwnBook replacement returned")
	}
	uci.lifecycleMu.Lock()
	got := uci.ownBook
	uci.lifecycleMu.Unlock()
	if got {
		t.Fatal("OwnBook=false was not applied after running search joined")
	}
}
