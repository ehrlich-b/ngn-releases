package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ehrlich-b/ngn/countereval"
	"github.com/ehrlich-b/ngn/nnue"
)

func writeUCINNUEFixture(t *testing.T, name string, tensors *nnue.Tensors) string {
	t.Helper()
	encoded, err := nnue.Marshal(tensors)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeUCICounterFixture(t *testing.T, name string, outputBias float32) string {
	t.Helper()
	data := make([]byte, countereval.LegacyFileSize)
	copy(data[:countereval.LegacyHeaderSize], []byte{
		0x42, 0x5a, 0x02, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x00, 0x03, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00,
	})
	binary.LittleEndian.PutUint32(data[len(data)-4:], math.Float32bits(outputBias))
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func requireUCIEvaluatorConfig(t *testing.T, uci *UCIEngine, backend, path string) uciEvaluatorConfig {
	t.Helper()
	config := uci.evaluatorConfigSnapshot()
	if config.backendName() != backend || config.file != path {
		t.Fatalf("evaluator config backend=%q file=%q, want backend=%q file=%q", config.backendName(), config.file, backend, path)
	}
	if path == "" && config.staged.kind != stagedEvaluatorNone {
		t.Fatal("empty EvalFile retained a staged model")
	}
	if path != "" && config.staged.kind == stagedEvaluatorNone {
		t.Fatal("non-empty EvalFile has no staged model")
	}
	return config
}

func TestUCIEvaluatorOptionDefaultsAndSelectedDiagnostic(t *testing.T) {
	uci := NewUCIEngine()
	if got := uci.searcher.SelectedEvaluatorBackend(); got != EvaluatorBackendHCEName {
		t.Fatalf("default SearchEngine evaluator = %q, want hce", got)
	}
	requireUCIEvaluatorConfig(t, uci, uciEvaluatorHCE, "")

	var output bytes.Buffer
	uci.handleCommand("uci", &output)
	response := output.String()
	for _, want := range []string{
		"option name EvalBackend type combo default hce var hce var ngn-v1 var ngn-k4-768-v1 var sf18-big var counter-5.5 var rodent-v1.1-anand var rodent-v1.2-default",
		"option name EvalFile type string default <empty>",
	} {
		if !strings.Contains(response, want) {
			t.Fatalf("uci response missing %q:\n%s", want, response)
		}
	}

	score, backend, err := uci.searcher.EvaluateSelected(uci.position.Copy())
	if err != nil {
		t.Fatal(err)
	}
	output.Reset()
	uci.handleCommand("eval", &output)
	want := fmt.Sprintf("info string eval backend %s score_cp %d pov side-to-move policy base-SearchSTM rule50-and-backend-adapter correction-history excluded", backend, score)
	if got := strings.TrimSpace(output.String()); got != want {
		t.Fatalf("eval diagnostic = %q, want %q", got, want)
	}
}

func TestConfigureStartupEvaluatorCounterAndAdvertisedDefaults(t *testing.T) {
	path := writeUCICounterFixture(t, "counter startup.nn", 19)
	uci := NewUCIEngine()
	if err := uci.ConfigureStartupEvaluator(" COUNTER-5.5 ", " "+path+" "); err != nil {
		t.Fatal(err)
	}
	config := requireUCIEvaluatorConfig(t, uci, uciEvaluatorCounter55, path)
	if config.staged.counter55 == nil || uci.searcher.SelectedEvaluatorBackend() != EvaluatorBackendCounter55Name {
		t.Fatal("startup configuration did not activate Counter")
	}

	var output bytes.Buffer
	uci.handleCommand("uci", &output)
	for _, want := range []string{
		"option name EvalBackend type combo default counter-5.5 var hce var ngn-v1 var ngn-k4-768-v1 var sf18-big var counter-5.5 var rodent-v1.1-anand var rodent-v1.2-default",
		"option name EvalFile type string default " + path,
	} {
		if !strings.Contains(output.String(), want+"\n") {
			t.Fatalf("configured UCI response missing %q:\n%s", want, output.String())
		}
	}

	// Defaults describe how this process started, not its mutable live selection.
	uci.handleCommand("setoption name EvalBackend value hce", &output)
	uci.handleCommand("setoption name EvalFile value <empty>", &output)
	output.Reset()
	uci.handleCommand("uci", &output)
	for _, want := range []string{
		"option name EvalBackend type combo default counter-5.5 var hce var ngn-v1 var ngn-k4-768-v1 var sf18-big var counter-5.5 var rodent-v1.1-anand var rodent-v1.2-default",
		"option name EvalFile type string default " + path,
	} {
		if !strings.Contains(output.String(), want+"\n") {
			t.Fatalf("runtime switch changed startup default %q:\n%s", want, output.String())
		}
	}
}

func TestConfigureStartupEvaluatorNGNV1(t *testing.T) {
	path := writeUCINNUEFixture(t, "startup.ngn", new(nnue.Tensors))
	uci := NewUCIEngine()
	if err := uci.ConfigureStartupEvaluator(uciEvaluatorNGNV1, path); err != nil {
		t.Fatal(err)
	}
	requireUCIEvaluatorConfig(t, uci, uciEvaluatorNGNV1, path)
	if uci.searcher.SelectedEvaluatorBackend() != EvaluatorBackendNGNV1Name {
		t.Fatal("startup configuration did not activate NGN v1")
	}
}

func TestConfigureStartupEvaluatorRejectsInvalidConfigurationTransactionally(t *testing.T) {
	counterPath := writeUCICounterFixture(t, "counter.nn", 19)
	ngnPath := writeUCINNUEFixture(t, "ngn.ngn", new(nnue.Tensors))
	missing := filepath.Join(t.TempDir(), "missing.nn")

	for _, test := range []struct {
		name, backend, path string
	}{
		{"unknown backend", "stockfish", ""},
		{"counter without file", uciEvaluatorCounter55, ""},
		{"ngn without file", uciEvaluatorNGNV1, "<empty>"},
		{"hce with file", uciEvaluatorHCE, counterPath},
		{"missing file", uciEvaluatorCounter55, missing},
		{"wrong model format", uciEvaluatorCounter55, ngnPath},
		{"line break in file", uciEvaluatorCounter55, counterPath + "\nignored"},
	} {
		t.Run(test.name, func(t *testing.T) {
			uci := NewUCIEngine()
			beforeModel := uci.searcher.evaluatorModel
			beforeWorker := uci.searcher.worker.evaluator
			if err := uci.ConfigureStartupEvaluator(test.backend, test.path); err == nil {
				t.Fatal("invalid startup evaluator configuration succeeded")
			}
			requireUCIEvaluatorConfig(t, uci, uciEvaluatorHCE, "")
			if uci.searcher.evaluatorModel != beforeModel || uci.searcher.worker.evaluator != beforeWorker ||
				uci.searcher.SelectedEvaluatorBackend() != EvaluatorBackendHCEName {
				t.Fatal("failed startup evaluator configuration changed active state")
			}
		})
	}
}

func TestUCIEvaluatorSetOptionOrdersAndTransactionalErrors(t *testing.T) {
	var tensors nnue.Tensors
	tensors.OutputBias = 7
	valid := writeUCINNUEFixture(t, "valid.ngn", &tensors)
	truncated := filepath.Join(t.TempDir(), "truncated.ngn")
	if err := os.WriteFile(truncated, []byte("NGNNUE"), 0o600); err != nil {
		t.Fatal(err)
	}
	unsupportedBytes, err := nnue.Marshal(&tensors)
	if err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint32(unsupportedBytes[16:], nnue.ArchitectureID+1)
	unsupported := filepath.Join(t.TempDir(), "unsupported.ngn")
	if err := os.WriteFile(unsupported, unsupportedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing.ngn")

	uci := NewUCIEngine()
	var output bytes.Buffer

	// Backend-first is rejected and remains HCE. Loading afterward stages the
	// exact model but never silently completes the rejected backend switch.
	uci.handleCommand("setoption name EvalBackend value ngn-v1", &output)
	if !strings.Contains(output.String(), "info string error eval option: EvalBackend ngn-v1 requires a matching valid staged EvalFile") {
		t.Fatalf("backend-first rejection missing canonical error:\n%s", output.String())
	}
	requireUCIEvaluatorConfig(t, uci, uciEvaluatorHCE, "")
	if got := uci.searcher.SelectedEvaluatorBackend(); got != EvaluatorBackendHCEName {
		t.Fatalf("failed backend-first selection changed backend to %q", got)
	}

	output.Reset()
	uci.handleCommand("setoption name EvalFile value "+valid, &output)
	if strings.Contains(output.String(), "info string error") {
		t.Fatalf("valid EvalFile failed:\n%s", output.String())
	}
	staged := requireUCIEvaluatorConfig(t, uci, uciEvaluatorHCE, valid)
	if got := uci.searcher.SelectedEvaluatorBackend(); got != EvaluatorBackendHCEName {
		t.Fatalf("staging EvalFile silently changed backend to %q", got)
	}

	output.Reset()
	uci.handleCommand("setoption name EvalBackend value ngn-v1", &output)
	if strings.Contains(output.String(), "info string error") {
		t.Fatalf("file-first backend selection failed:\n%s", output.String())
	}
	requireUCIEvaluatorConfig(t, uci, uciEvaluatorNGNV1, valid)
	if got := uci.searcher.SelectedEvaluatorBackend(); got != EvaluatorBackendNGNV1Name {
		t.Fatalf("selected backend = %q, want ngn-v1", got)
	}
	selectedIdentity := uci.searcher.evaluatorModel.identity

	for _, test := range []struct {
		name, command string
	}{
		{"invalid backend", "setoption name EvalBackend value stockfish"},
		{"empty active file", "setoption name EvalFile value <empty>"},
		{"missing file", "setoption name EvalFile value " + missing},
		{"truncated file", "setoption name EvalFile value " + truncated},
		{"unsupported file", "setoption name EvalFile value " + unsupported},
	} {
		t.Run(test.name, func(t *testing.T) {
			output.Reset()
			uci.handleCommand(test.command, &output)
			if !strings.HasPrefix(output.String(), "info string error eval option: ") {
				t.Fatalf("failure omitted canonical prefix:\n%s", output.String())
			}
			config := requireUCIEvaluatorConfig(t, uci, uciEvaluatorNGNV1, valid)
			if config.staged.ngnV1 != staged.staged.ngnV1 {
				t.Fatal("failed reconfiguration replaced the staged model")
			}
			if uci.searcher.evaluatorModel.identity != selectedIdentity {
				t.Fatal("failed reconfiguration changed selected model identity")
			}
		})
	}

	uci.handleCommand("setoption name EvalBackend value hce", &output)
	config := requireUCIEvaluatorConfig(t, uci, uciEvaluatorHCE, valid)
	if config.staged.ngnV1 != staged.staged.ngnV1 {
		t.Fatal("selecting HCE discarded the staged EvalFile model")
	}
	output.Reset()
	uci.handleCommand("setoption name EvalFile value <empty>", &output)
	if output.Len() != 0 {
		t.Fatalf("clearing staged HCE file produced output: %s", output.String())
	}
	requireUCIEvaluatorConfig(t, uci, uciEvaluatorHCE, "")
}

func TestUCIEvaluatorActiveModelReplacementAndNewGamePersistence(t *testing.T) {
	var tensorsA, tensorsB nnue.Tensors
	tensorsA.OutputBias = -30000
	tensorsB.OutputBias = 30000
	pathA := writeUCINNUEFixture(t, "a.ngn", &tensorsA)
	pathB := writeUCINNUEFixture(t, "b.ngn", &tensorsB)
	bad := filepath.Join(t.TempDir(), "bad.ngn")
	if err := os.WriteFile(bad, []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}

	uci := NewUCIEngine()
	var output bytes.Buffer
	uci.handleCommand("setoption name EvalFile value "+pathA, &output)
	uci.handleCommand("setoption name EvalBackend value ngn-v1", &output)
	configA := requireUCIEvaluatorConfig(t, uci, uciEvaluatorNGNV1, pathA)
	identityA := uci.searcher.evaluatorModel.identity
	scoreA, _, err := uci.searcher.EvaluateSelected(uci.position.Copy())
	if err != nil {
		t.Fatal(err)
	}

	output.Reset()
	uci.handleCommand("setoption name EvalFile value "+pathB, &output)
	if strings.Contains(output.String(), "info string error") {
		t.Fatalf("valid active replacement failed:\n%s", output.String())
	}
	configB := requireUCIEvaluatorConfig(t, uci, uciEvaluatorNGNV1, pathB)
	identityB := uci.searcher.evaluatorModel.identity
	if configB.staged.ngnV1 == configA.staged.ngnV1 || identityB == identityA {
		t.Fatal("A-to-B replacement retained model A")
	}
	scoreB, _, err := uci.searcher.EvaluateSelected(uci.position.Copy())
	if err != nil {
		t.Fatal(err)
	}
	if scoreB == scoreA {
		t.Fatalf("distinct output-bias fixtures produced equal selected scores %d", scoreA)
	}

	output.Reset()
	uci.handleCommand("setoption name EvalFile value "+bad, &output)
	if !strings.HasPrefix(output.String(), "info string error eval option: ") {
		t.Fatalf("failed replacement omitted canonical error:\n%s", output.String())
	}
	configAfterFailure := requireUCIEvaluatorConfig(t, uci, uciEvaluatorNGNV1, pathB)
	if configAfterFailure.staged.ngnV1 != configB.staged.ngnV1 || uci.searcher.evaluatorModel.identity != identityB {
		t.Fatal("failed replacement changed active model B")
	}
	info := uci.searcher.SearchFixed(newUCIStartingPosition(), 1, nil)
	if info.BestMove == EmptyMove || info.Stopped {
		t.Fatalf("preserved model B is not usable after failed replacement: move=%s stopped=%v", info.BestMove.ToString(), info.Stopped)
	}

	uci.handleCommand("ucinewgame", &output)
	configAfterNewGame := requireUCIEvaluatorConfig(t, uci, uciEvaluatorNGNV1, pathB)
	if configAfterNewGame.staged.ngnV1 != configB.staged.ngnV1 || uci.searcher.SelectedEvaluatorBackend() != EvaluatorBackendNGNV1Name {
		t.Fatal("ucinewgame changed evaluator selection or staged file")
	}
}

func TestUCIEvalFileReplacementJoinsPreparedSearchAndSuppressesOutput(t *testing.T) {
	var tensorsA, tensorsB nnue.Tensors
	tensorsA.OutputBias = 3
	tensorsB.OutputBias = 5
	pathA := writeUCINNUEFixture(t, "prepared-a.ngn", &tensorsA)
	pathB := writeUCINNUEFixture(t, "prepared-b.ngn", &tensorsB)

	clock := &uciFakeClock{now: time.Date(2026, 9, 6, 17, 0, 0, 0, time.UTC)}
	uci := newLifecycleTestUCI(t, clock)
	uci.handleCommand("setoption name EvalFile value "+pathA, ioDiscardBuffer{})
	uci.handleCommand("setoption name EvalBackend value ngn-v1", ioDiscardBuffer{})
	entered, blockSetup, releaseSetup := newUCISetupGate(t)
	uci.copyPosition = func(pos *Position) *Position {
		blockSetup()
		return pos.Copy()
	}

	var searchOutput bytes.Buffer
	uci.handleGo([]string{"depth", "8"}, &searchOutput)
	waitUCIChannel(t, entered, "NNUE prepared search setup")
	uci.lifecycleMu.Lock()
	session := uci.activeSearch
	uci.lifecycleMu.Unlock()
	if session == nil {
		releaseSetup()
		t.Fatal("prepared NNUE session disappeared")
	}

	setDone := make(chan struct{})
	var optionOutput bytes.Buffer
	go func() {
		uci.handleCommand("setoption name EvalFile value "+pathB, &optionOutput)
		close(setDone)
	}()
	waitUCIChannel(t, session.cancelled, "EvalFile prepared-search cancellation")
	select {
	case <-setDone:
		releaseSetup()
		t.Fatal("EvalFile replacement returned before prepared search joined")
	default:
	}
	releaseSetup()
	waitUCIChannel(t, setDone, "EvalFile prepared-search replacement")
	if optionOutput.Len() != 0 {
		t.Fatalf("valid replacement produced output:\n%s", optionOutput.String())
	}
	if strings.Contains(searchOutput.String(), "bestmove ") {
		t.Fatalf("suppressed prepared search emitted stale bestmove:\n%s", searchOutput.String())
	}
	requireUCIEvaluatorConfig(t, uci, uciEvaluatorNGNV1, pathB)
}

func TestUCICounter55StagingSelectionAndCrossFormatReplacement(t *testing.T) {
	counterPath := writeUCICounterFixture(t, "counter.nn", 19)
	replacementPath := writeUCICounterFixture(t, "counter-replacement.nn", 23)
	ngnPath := writeUCINNUEFixture(t, "ngn.ngn", new(nnue.Tensors))
	uci := NewUCIEngine()
	var output bytes.Buffer

	uci.handleCommand("setoption name EvalFile value "+counterPath, &output)
	config := requireUCIEvaluatorConfig(t, uci, uciEvaluatorHCE, counterPath)
	if config.staged.kind != stagedEvaluatorCounter55 || config.staged.counter55 == nil {
		t.Fatalf("Counter file staged as %+v", config.staged)
	}
	output.Reset()
	uci.handleCommand("setoption name EvalBackend value ngn-v1", &output)
	if !strings.Contains(output.String(), "requires a matching valid staged EvalFile") {
		t.Fatalf("mismatched backend was not rejected: %s", output.String())
	}
	requireUCIEvaluatorConfig(t, uci, uciEvaluatorHCE, counterPath)

	output.Reset()
	uci.handleCommand("setoption name EvalBackend value counter-5.5", &output)
	if strings.Contains(output.String(), "info string error") {
		t.Fatalf("Counter selection failed: %s", output.String())
	}
	requireUCIEvaluatorConfig(t, uci, uciEvaluatorCounter55, counterPath)
	if got := uci.searcher.SelectedEvaluatorBackend(); got != EvaluatorBackendCounter55Name {
		t.Fatalf("selected backend=%q", got)
	}
	identity := uci.searcher.evaluatorModel.identity

	output.Reset()
	uci.handleCommand("setoption name EvalFile value "+replacementPath, &output)
	if strings.Contains(output.String(), "info string error") {
		t.Fatalf("same-format Counter replacement failed: %s", output.String())
	}
	replacement := requireUCIEvaluatorConfig(t, uci, uciEvaluatorCounter55, replacementPath)
	if replacement.staged.counter55 == nil || uci.searcher.evaluatorModel.identity == identity {
		t.Fatal("same-format Counter replacement did not publish new immutable identity")
	}
	identity = uci.searcher.evaluatorModel.identity

	output.Reset()
	uci.handleCommand("setoption name EvalFile value "+ngnPath, &output)
	if !strings.Contains(output.String(), "select hce first") {
		t.Fatalf("active cross-format replacement was not rejected: %s", output.String())
	}
	config = requireUCIEvaluatorConfig(t, uci, uciEvaluatorCounter55, replacementPath)
	if config.staged.counter55 == nil || uci.searcher.evaluatorModel.identity != identity {
		t.Fatal("failed cross-format replacement changed Counter state")
	}

	uci.handleCommand("ucinewgame", &output)
	requireUCIEvaluatorConfig(t, uci, uciEvaluatorCounter55, replacementPath)
	if uci.searcher.SelectedEvaluatorBackend() != EvaluatorBackendCounter55Name {
		t.Fatal("new game changed Counter backend")
	}
}

func TestUCISF18BIGStagingSelectionAndNewGamePersistence(t *testing.T) {
	path := os.Getenv(sf18BigResearchModelEnvironment)
	if path == "" {
		t.Skip("official SF18 BIG model is required for the normal UCI gate")
	}
	bad := filepath.Join(t.TempDir(), "bad-sf18-big.nnue")
	if err := os.WriteFile(bad, []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}

	uci := NewUCIEngine()
	var output bytes.Buffer
	uci.handleCommand("setoption name EvalFile value "+path, &output)
	if strings.Contains(output.String(), "info string error") {
		t.Fatalf("SF18 BIG staging failed: %s", output.String())
	}
	config := requireUCIEvaluatorConfig(t, uci, uciEvaluatorHCE, path)
	if config.staged.kind != stagedEvaluatorSF18BIG || config.staged.sf18Big == nil {
		t.Fatalf("SF18 BIG file staged as %+v", config.staged)
	}
	if got := uci.searcher.SelectedEvaluatorBackend(); got != EvaluatorBackendHCEName {
		t.Fatalf("staging silently selected %q", got)
	}

	output.Reset()
	uci.handleCommand("setoption name EvalBackend value sf18-big", &output)
	if strings.Contains(output.String(), "info string error") {
		t.Fatalf("SF18 BIG selection failed: %s", output.String())
	}
	config = requireUCIEvaluatorConfig(t, uci, uciEvaluatorSF18BIG, path)
	if got := uci.searcher.SelectedEvaluatorBackend(); got != EvaluatorBackendSF18BIGName {
		t.Fatalf("selected backend=%q, want %q", got, EvaluatorBackendSF18BIGName)
	}
	identity := uci.searcher.evaluatorModel.identity
	if _, backend, err := uci.searcher.EvaluateSelected(uci.position.Copy()); err != nil {
		t.Fatal(err)
	} else if backend != EvaluatorBackendSF18BIGName {
		t.Fatalf("evaluation backend=%q, want %q", backend, EvaluatorBackendSF18BIGName)
	}

	output.Reset()
	uci.handleCommand("setoption name EvalFile value "+bad, &output)
	if !strings.HasPrefix(output.String(), "info string error eval option: ") {
		t.Fatalf("invalid replacement omitted canonical error: %s", output.String())
	}
	afterFailure := requireUCIEvaluatorConfig(t, uci, uciEvaluatorSF18BIG, path)
	if afterFailure.staged.sf18Big != config.staged.sf18Big || uci.searcher.evaluatorModel.identity != identity {
		t.Fatal("failed replacement changed active SF18 BIG state")
	}

	output.Reset()
	uci.handleCommand("ucinewgame", &output)
	afterNewGame := requireUCIEvaluatorConfig(t, uci, uciEvaluatorSF18BIG, path)
	if afterNewGame.staged.sf18Big != config.staged.sf18Big ||
		uci.searcher.SelectedEvaluatorBackend() != EvaluatorBackendSF18BIGName {
		t.Fatal("ucinewgame changed SF18 BIG selection or staged file")
	}
	info := uci.searcher.SearchFixed(newUCIStartingPosition(), 1, nil)
	if info.BestMove == EmptyMove || info.Stopped {
		t.Fatalf("selected SF18 BIG model is not searchable: move=%s stopped=%v", info.BestMove.ToString(), info.Stopped)
	}
}

type ioDiscardBuffer struct{}

func (ioDiscardBuffer) Write(p []byte) (int, error) { return len(p), nil }

func waitUCIRunning(t *testing.T, uci *UCIEngine) *uciSearchSession {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		uci.lifecycleMu.Lock()
		session, state := uci.activeSearch, uci.searchState
		uci.lifecycleMu.Unlock()
		if session != nil && state == uciSearchRunning {
			return session
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for running UCI search")
	return nil
}

func TestUCIEvalFileReplacementJoinsRunningSearchAndSuppressesOutput(t *testing.T) {
	var tensorsA, tensorsB nnue.Tensors
	tensorsA.OutputBias = 13
	tensorsB.OutputBias = 17
	pathA := writeUCINNUEFixture(t, "running-a.ngn", &tensorsA)
	pathB := writeUCINNUEFixture(t, "running-b.ngn", &tensorsB)

	uci := NewUCIEngine()
	t.Cleanup(func() { uci.joinSearch(true) })
	uci.handleCommand("setoption name EvalFile value "+pathA, ioDiscardBuffer{})
	uci.handleCommand("setoption name EvalBackend value ngn-v1", ioDiscardBuffer{})

	var searchOutput bytes.Buffer
	uci.handleCommand("position fen r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 0 1", &searchOutput)
	searchOutput.Reset()
	uci.handleGo([]string{"depth", "40"}, &searchOutput)
	session := waitUCIRunning(t, uci)
	uci.handleCommand("setoption name EvalFile value "+pathB, ioDiscardBuffer{})
	select {
	case <-session.done:
	default:
		t.Fatal("EvalFile replacement returned before running search joined")
	}
	if strings.Contains(searchOutput.String(), "bestmove ") {
		t.Fatalf("suppressed running search emitted stale bestmove:\n%s", searchOutput.String())
	}
	sizeAtReturn := searchOutput.Len()
	time.Sleep(10 * time.Millisecond)
	if searchOutput.Len() != sizeAtReturn {
		t.Fatal("joined running search emitted output after replacement returned")
	}
	requireUCIEvaluatorConfig(t, uci, uciEvaluatorNGNV1, pathB)
}

func TestUCIShortLegalSearchWithAcceptedNGNV1SmokeFile(t *testing.T) {
	path := os.Getenv("NGN_ACCEPTED_NNUE_SMOKE")
	if path == "" {
		t.Skip("NGN_ACCEPTED_NNUE_SMOKE is not set")
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const wantSHA = "c7581bfae2e43e267aae6b2cbd24d85596c35e89b1728c27f45d9ccd4c9300a6"
	if got := fmt.Sprintf("%x", sha256.Sum256(encoded)); got != wantSHA {
		t.Fatalf("accepted smoke model SHA256=%s, want %s", got, wantSHA)
	}

	uci := NewUCIEngine()
	t.Cleanup(func() { uci.joinSearch(true) })
	var optionOutput bytes.Buffer
	uci.handleCommand("setoption name EvalFile value "+path, &optionOutput)
	uci.handleCommand("setoption name EvalBackend value ngn-v1", &optionOutput)
	if strings.Contains(optionOutput.String(), "info string error") {
		t.Fatalf("accepted smoke model selection failed:\n%s", optionOutput.String())
	}

	var searchOutput bytes.Buffer
	uci.handleCommand("position fen r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 0 1", &searchOutput)
	searchOutput.Reset()
	uci.handleCommand("go depth 2", &searchOutput)
	deadline := time.Now().Add(5 * time.Second)
	for uci.isSearching() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if uci.isSearching() {
		t.Fatal("accepted smoke NNUE search did not finish")
	}
	response := searchOutput.String()
	if strings.Contains(response, "info string error") || !strings.Contains(response, "info depth 2") || !strings.Contains(response, "bestmove ") {
		t.Fatalf("accepted smoke NNUE search response:\n%s", response)
	}
}

func TestUCIEvaluatorFormatDispatchRejectsMutationsTransactionally(t *testing.T) {
	validCounter := writeUCICounterFixture(t, "valid-counter.nn", 7)
	corruptedCounter := writeUCICounterFixture(t, "corrupt-counter.nn", 7)
	encoded, err := os.ReadFile(corruptedCounter)
	if err != nil {
		t.Fatal(err)
	}
	encoded[5] ^= 1
	if err := os.WriteFile(corruptedCounter, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	unsupported := filepath.Join(t.TempDir(), "unsupported.nn")
	if err := os.WriteFile(unsupported, bytes.Repeat([]byte{0x5a}, countereval.LegacyFileSize), 0o600); err != nil {
		t.Fatal(err)
	}

	uci := NewUCIEngine()
	var output bytes.Buffer
	uci.handleCommand("setoption name EvalFile value "+validCounter, &output)
	before := requireUCIEvaluatorConfig(t, uci, uciEvaluatorHCE, validCounter)
	for _, path := range []string{corruptedCounter, unsupported} {
		output.Reset()
		uci.handleCommand("setoption name EvalFile value "+path, &output)
		if !strings.Contains(output.String(), "info string error eval option") {
			t.Fatalf("invalid format %q produced no error: %s", path, output.String())
		}
		after := requireUCIEvaluatorConfig(t, uci, uciEvaluatorHCE, validCounter)
		if after.staged.kind != before.staged.kind || after.staged.counter55 != before.staged.counter55 {
			t.Fatal("invalid format changed staged Counter model")
		}
	}
}
