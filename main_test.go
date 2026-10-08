package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ehrlich-b/ngn/engine"
)

func useProfile(t *testing.T, name string) {
	profile := releaseProfile
	t.Cleanup(func() { releaseProfile = profile })
	releaseProfile = name
}

func TestRunDefaultEmbedsNNUEAndKeepsItForEchoedDefaults(t *testing.T) {
	var output, errorOutput bytes.Buffer
	input := "uci\neval\nsetoption name EvalFile value <embedded>\nsetoption name EvalFile value <empty>\nsetoption name EvalFile value\nsetoption name UseNNUE value true\nucinewgame\neval\n" +
		"setoption name EvalFile value missing.nnue\neval\nsetoption name UseNNUE value false\neval\nquit\n"
	if exit := run(nil, strings.NewReader(input), &output, &errorOutput); exit != 0 || errorOutput.Len() != 0 {
		t.Fatalf("NNUE startup exit=%d stderr=%q", exit, errorOutput.String())
	}
	for _, option := range []string{"option name EvalFile type string default <embedded>\n", "option name UseNNUE type check default true\n",
		"option name OwnBook type check default false\n", "id author Bryan Ehrlich\n"} {
		if !strings.Contains(output.String(), option) {
			t.Fatalf("missing %q in %s", option, output.String())
		}
	}
	if got := strings.Count(output.String(), "info string eval backend ngnn2 "); got != 3 {
		t.Fatalf("embedded network evaluations=%d, want 3: %s", got, output.String())
	}
	if !strings.Contains(output.String(), "using embedded network") || strings.Count(output.String(), "info string eval backend hce ") != 1 {
		t.Fatalf("missing-file fallback or UseNNUE=false wrong: %s", output.String())
	}
}

func TestEmbeddedNetDigest(t *testing.T) {
	sum := sha256.Sum256(engine.DefaultNetBytes())
	if hex.EncodeToString(sum[:]) != engine.DefaultNetSHA256 {
		t.Fatalf("embedded network digest %x, want %s", sum, engine.DefaultNetSHA256)
	}
}

func TestRunDefaultHCEIgnoresAdjacentModelsAndOffersNGNN1(t *testing.T) {
	useProfile(t, "hce")
	var output, errorOutput bytes.Buffer
	input := "uci\neval\nsetoption name EvalFile value ngn.nnue\nsetoption name EvalBackend value ngn-v1\nucinewgame\neval\nquit\n"
	if exit := run(nil, strings.NewReader(input), &output, &errorOutput); exit != 0 || errorOutput.Len() != 0 {
		t.Fatalf("HCE startup exit=%d stderr=%q", exit, errorOutput.String())
	}
	for _, option := range []string{"option name EvalBackend", "option name K4EvalScale"} {
		if strings.Contains(output.String(), option) {
			t.Fatalf("HCE startup advertised evaluator switching: %s", output.String())
		}
	}
	for _, option := range []string{"option name EvalFile type string default <empty>\n", "option name UseNNUE type check default false\n"} {
		if !strings.Contains(output.String(), option) {
			t.Fatalf("missing NGNN1 option %q", option)
		}
	}
	if strings.Count(output.String(), "info string eval backend hce ") != 2 {
		t.Fatalf("GUI options or new game changed HCE: %s", output.String())
	}
	if !strings.Contains(output.String(), "option name OwnBook type check default false\n") {
		t.Fatalf("default book was enabled: %s", output.String())
	}
}

func TestRunHCERejectsExplicitExternalEvaluatorBeforeUCI(t *testing.T) {
	useProfile(t, "hce")
	for _, args := range [][]string{
		{"-eval-backend", "ngn-v1"},
		{"-eval-backend", "external", "-eval-file", "missing.nn"},
		{"-eval-file", "ngn.nnue"},
	} {
		var output, errorOutput bytes.Buffer
		if exit := run(args, strings.NewReader("uci\nquit\n"), &output, &errorOutput); exit != 2 || output.Len() != 0 ||
			!strings.Contains(errorOutput.String(), "HCE build requires") {
			t.Fatalf("args=%q exit=%d stdout=%q stderr=%q", args, exit, output.String(), errorOutput.String())
		}
	}
}

func TestRunHCEVersionAndExplicitBook(t *testing.T) {
	var output, errorOutput bytes.Buffer
	if exit := run([]string{"-version"}, strings.NewReader(""), &output, &errorOutput); exit != 0 ||
		output.String() != releaseVersion+"\n" || errorOutput.Len() != 0 {
		t.Fatalf("version exit=%d stdout=%q stderr=%q", exit, output.String(), errorOutput.String())
	}
	output.Reset()
	if exit := run([]string{"-own-book=true", "-eval-backend", "HCE", "-eval-file", "<empty>"},
		strings.NewReader("uci\nquit\n"), &output, &errorOutput); exit != 0 || errorOutput.Len() != 0 ||
		!strings.Contains(output.String(), "option name OwnBook type check default true\n") {
		t.Fatalf("explicit HCE exit=%d stdout=%q stderr=%q", exit, output.String(), errorOutput.String())
	}
}

func TestRunRetiredOwnedProfileFailsClosed(t *testing.T) {
	profile := releaseProfile
	t.Cleanup(func() { releaseProfile = profile })
	releaseProfile = "owned"
	var output, errorOutput bytes.Buffer
	if exit := run(nil, strings.NewReader("uci\nquit\n"), &output, &errorOutput); exit != 2 || output.Len() != 0 ||
		!strings.Contains(errorOutput.String(), "retired build profile") {
		t.Fatalf("retired profile exit=%d stdout=%q stderr=%q", exit, output.String(), errorOutput.String())
	}
}

func TestRunRetiredResearchProfileFailsClosed(t *testing.T) {
	profile := releaseProfile
	t.Cleanup(func() { releaseProfile = profile })
	releaseProfile = "research"
	var output, errorOutput bytes.Buffer
	if exit := run(nil, strings.NewReader("uci\nquit\n"), &output, &errorOutput); exit != 2 || output.Len() != 0 ||
		!strings.Contains(errorOutput.String(), "retired build profile") {
		t.Fatalf("retired research exit=%d stdout=%q stderr=%q", exit, output.String(), errorOutput.String())
	}
}
