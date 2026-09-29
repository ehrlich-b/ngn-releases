//go:build counteroracle

package engine

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestUCIShortLegalSearchWithAcceptedCounter55File(t *testing.T) {
	path := requireCounterOracleEnv(t, "COUNTER_MODEL")
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(encoded)); got != acceptedCounter55SHA256 {
		t.Fatalf("Counter model SHA256=%s", got)
	}
	uci := NewUCIEngine()
	t.Cleanup(func() { uci.joinSearch(true) })
	var output bytes.Buffer
	for _, command := range []string{
		"setoption name OwnBook value false",
		"setoption name EvalFile value " + path,
		"setoption name EvalBackend value counter-5.5",
	} {
		uci.handleCommand(command, &output)
	}
	if strings.Contains(output.String(), "info string error") {
		t.Fatalf("Counter UCI selection failed:\n%s", output.String())
	}
	const fen = "r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 0 1"
	output.Reset()
	uci.handleCommand("position fen "+fen, &output)
	output.Reset()
	uci.handleCommand("go depth 2", &output)
	deadline := time.Now().Add(5 * time.Second)
	for uci.isSearching() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if uci.isSearching() {
		t.Fatal("Counter UCI search did not finish")
	}
	response := output.String()
	if strings.Contains(response, "info string error") || !strings.Contains(response, "info depth 2") || strings.Count(response, "bestmove ") != 1 {
		t.Fatalf("Counter UCI response:\n%s", response)
	}
	fields := strings.Fields(response)
	best := ""
	for index, field := range fields {
		if field == "bestmove" && index+1 < len(fields) {
			best = fields[index+1]
		}
	}
	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseUCIMove(pos, best); err != nil {
		t.Fatalf("Counter UCI bestmove %q is not legal: %v", best, err)
	}
}
