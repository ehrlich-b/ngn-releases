package k4label

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHelperTeacher(t *testing.T) {
	if os.Getenv("NGN_K4_LABEL_HELPER") != "1" {
		return
	}
	mode := os.Getenv("NGN_K4_LABEL_HELPER_MODE")
	configured := map[string]bool{}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		command := strings.TrimSpace(scanner.Text())
		switch {
		case command == "uci":
			fmt.Println("id name Stockfish 18 NGN fixture")
			fmt.Println("option name Threads type spin default 1 min 1 max 128")
			fmt.Println("option name Hash type spin default 16 min 1 max 33554432")
			fmt.Println("option name MultiPV type spin default 1 min 1 max 500")
			fmt.Println("option name SyzygyPath type string default <empty>")
			fmt.Println("uciok")
		case strings.HasPrefix(command, "setoption name Threads value 1"):
			configured["threads"] = true
		case strings.HasPrefix(command, "setoption name Hash value 16"):
			configured["hash"] = true
		case strings.HasPrefix(command, "setoption name MultiPV value 1"):
			configured["multipv"] = true
		case strings.HasPrefix(command, "setoption name SyzygyPath value <empty>"):
			configured["syzygy"] = true
		case command == "isready":
			fmt.Println("readyok")
		case strings.HasPrefix(command, "go nodes 5000"):
			if !configured["threads"] || !configured["hash"] || !configured["multipv"] || !configured["syzygy"] {
				fmt.Println("info string error: missing frozen options")
				continue
			}
			switch mode {
			case "accepted":
				fmt.Println("info depth 3 seldepth 5 multipv 1 score cp 11 nodes 3000 pv d2d4")
				fmt.Println("info depth 7 seldepth 11 multipv 1 score cp 100 nodes 5000 nps 100000 pv e2e4 e7e5")
				fmt.Println("bestmove e2e4 ponder e7e5")
			case "bound":
				fmt.Println("info depth 7 seldepth 11 score cp 100 lowerbound nodes 5000 pv e2e4")
				fmt.Println("bestmove e2e4")
			case "mate":
				fmt.Println("info depth 7 seldepth 11 score mate 3 nodes 5000 pv e2e4")
				fmt.Println("bestmove e2e4")
			case "short":
				fmt.Println("info depth 7 seldepth 11 score cp 100 nodes 4999 pv e2e4")
				fmt.Println("bestmove e2e4")
			case "malformed":
				fmt.Println("info depth nope score cp 100 nodes 5000 pv e2e4")
				fmt.Println("bestmove e2e4")
			case "timeout":
				time.Sleep(5 * time.Second)
			default:
				fmt.Println("bestmove e2e4")
			}
		case command == "quit":
			return
		}
	}
}

func startFixtureSession(t *testing.T, mode string) *Session {
	t.Helper()
	t.Setenv("NGN_K4_LABEL_HELPER", "1")
	t.Setenv("NGN_K4_LABEL_HELPER_MODE", mode)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	session, err := StartSession(ctx, []string{os.Args[0], "-test.run=TestHelperTeacher"}, TeacherSourceCommit, TeacherBigNetworkSHA, TeacherSmallNetSHA)
	if err != nil {
		t.Fatal(err)
	}
	if session.Provenance().Name != "Stockfish 18 NGN fixture" || !validLowerHexSHA(session.Provenance().ExecutableSHA256) ||
		!validLowerHexSHA(session.Provenance().HandshakeOptionDigest) {
		t.Fatalf("fixture provenance = %+v", session.Provenance())
	}
	return session
}

func TestSessionExactScoreAndRejectionContracts(t *testing.T) {
	const fen = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	tests := []struct {
		mode       string
		wantReason string
		wantError  bool
	}{
		{"accepted", "", false},
		{"bound", "bound-only-score", false},
		{"mate", "mate-score", false},
		{"short", "", false},
		{"malformed", "", true},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			session := startFixtureSession(t, test.mode)
			result, reason, err := session.Analyze(context.Background(), fen)
			closeErr := session.Close()
			if test.wantError {
				if err == nil {
					t.Fatal("malformed teacher response succeeded")
				}
				return
			}
			if err != nil || closeErr != nil || reason != test.wantReason {
				t.Fatalf("Analyze = %+v reason=%q err=%v close=%v", result, reason, err, closeErr)
			}
			if test.mode == "accepted" && (result.BestMove != "e2e4" || result.PVMove != "e2e4" ||
				result.CP != 100 || result.Depth != 7 || result.SelDepth != 11 || result.Nodes != 5000 || result.InfoLine == "") {
				t.Fatalf("accepted result = %+v", result)
			}
		})
	}
}

func TestSessionTimeoutIsFatalAndKillsTeacher(t *testing.T) {
	session := startFixtureSession(t, "timeout")
	session.config.TimeoutMillis = 50
	_, _, err := session.Analyze(context.Background(), "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("timeout error = %v", err)
	}
	_ = session.Close()
}

func TestParseInfoRejectsMalformedAndIgnoresOtherMultiPV(t *testing.T) {
	if _, err := parseInfo("info depth nope score cp 3"); err == nil {
		t.Fatal("malformed depth accepted")
	}
	parsed, err := parseInfo("info depth 9 multipv 2 score cp 44 nodes 5000 pv d2d4")
	if err != nil || parsed.hasScore {
		t.Fatalf("multipv 2 parse = %+v, %v", parsed, err)
	}
}
