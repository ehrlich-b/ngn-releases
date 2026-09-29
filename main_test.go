package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunDefaultHCEAdvertisesActualDefaults(t *testing.T) {
	var output, errorOutput bytes.Buffer
	if exit := run(nil, strings.NewReader("uci\nquit\n"), &output, &errorOutput); exit != 0 {
		t.Fatalf("run exit=%d stderr=%s", exit, errorOutput.String())
	}
	if errorOutput.Len() != 0 {
		t.Fatalf("default startup stderr=%q", errorOutput.String())
	}
	for _, want := range []string{
		"option name EvalBackend type combo default hce var hce var ngn-v1 var ngn-k4-768-v1 var sf18-big var counter-5.5 var rodent-v1.1-anand var rodent-v1.2-default\n",
		"option name EvalFile type string default <empty>\n",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("default UCI output missing %q:\n%s", want, output.String())
		}
	}
}

func setOwnedReleaseForTest(t *testing.T) {
	t.Helper()
	profile, version, network := releaseProfile, releaseVersion, releaseNetwork
	parallelism := runtime.GOMAXPROCS(0)
	t.Cleanup(func() {
		releaseProfile, releaseVersion, releaseNetwork = profile, version, network
		runtime.GOMAXPROCS(parallelism)
	})
	releaseProfile, releaseVersion, releaseNetwork = "owned", "0.2.0-rc.1", "missing-owned-release-test.nnue"
}

func TestOwnedReleaseFindsAdjacentModelAndFailsClosed(t *testing.T) {
	setOwnedReleaseForTest(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	backend, model, scale, book, err := releaseDefaults()
	if err != nil || backend != "ngn-k4-768-v1" || model != filepath.Join(filepath.Dir(executable), releaseNetwork) || scale != 60 || book {
		t.Fatalf("release defaults: backend=%q model=%q scale=%d book=%v err=%v", backend, model, scale, book, err)
	}
	var output, errorOutput bytes.Buffer
	if exit := run(nil, strings.NewReader("uci\nquit\n"), &output, &errorOutput); exit != 2 || output.Len() != 0 || !strings.Contains(errorOutput.String(), "evaluator startup configuration failed") {
		t.Fatalf("missing owned model: exit=%d stdout=%q stderr=%q", exit, output.String(), errorOutput.String())
	}
	output.Reset()
	errorOutput.Reset()
	if exit := run([]string{"-version"}, strings.NewReader(""), &output, &errorOutput); exit != 0 || output.String() != "0.2.0-rc.1\n" || errorOutput.Len() != 0 {
		t.Fatalf("version without model: exit=%d stdout=%q stderr=%q", exit, output.String(), errorOutput.String())
	}
}

func TestOwnedReleaseThreadsControlSchedulerWithExplicitOverride(t *testing.T) {
	setOwnedReleaseForTest(t)
	t.Setenv("GOMAXPROCS", "")
	input := "uci\nsetoption name Threads value 8\nsetoption name Threads value 0\nquit\n"
	args := []string{"-eval-backend", "hce", "-eval-file", ""}
	var output, errorOutput bytes.Buffer
	if exit := run(args, strings.NewReader(input), &output, &errorOutput); exit != 0 || errorOutput.Len() != 0 {
		t.Fatalf("owned-profile session: exit=%d stderr=%q", exit, errorOutput.String())
	}
	if runtime.GOMAXPROCS(0) != 8 {
		t.Fatalf("accepted width followed by invalid width changed parallelism to %d", runtime.GOMAXPROCS(0))
	}
	if !strings.Contains(output.String(), "id name ngn 0.2.0-rc.1\n") {
		t.Fatalf("release identity missing: %s", output.String())
	}
	t.Setenv("GOMAXPROCS", "3")
	runtime.GOMAXPROCS(3)
	output.Reset()
	errorOutput.Reset()
	if exit := run(args, strings.NewReader(input), &output, &errorOutput); exit != 0 || runtime.GOMAXPROCS(0) != 3 {
		t.Fatalf("explicit scheduler constraint lost: exit=%d parallelism=%d stderr=%q", exit, runtime.GOMAXPROCS(0), errorOutput.String())
	}
}

func TestRunInvalidExplicitEvaluatorFailsBeforeUCI(t *testing.T) {
	for _, test := range []struct {
		name, wantPrefix string
		args             []string
	}{
		{"missing model", "ngn: evaluator startup configuration failed: ", []string{"-eval-backend", "counter-5.5"}},
		{"unsupported backend", "ngn: evaluator startup configuration failed: ", []string{"-eval-backend", "stockfish"}},
		{"file with hce", "ngn: evaluator startup configuration failed: ", []string{"-eval-backend", "hce", "-eval-file", "model.nn"}},
		{"positional before explicit request", "ngn: unexpected positional arguments: ", []string{"stray", "-eval-backend", "counter-5.5"}},
		{"scale below range", "ngn: K4 score scale configuration failed: ", []string{"-k4-eval-scale", "9"}},
		{"scale above range", "ngn: K4 score scale configuration failed: ", []string{"-k4-eval-scale", "401"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output, errorOutput bytes.Buffer
			exit := run(test.args, strings.NewReader("uci\nquit\n"), &output, &errorOutput)
			if exit != 2 {
				t.Fatalf("run exit=%d, want 2", exit)
			}
			if output.Len() != 0 {
				t.Fatalf("failed startup reached UCI: %q", output.String())
			}
			if !strings.HasPrefix(errorOutput.String(), test.wantPrefix) {
				t.Fatalf("startup stderr=%q", errorOutput.String())
			}
		})
	}
}

func TestRunExplicitStartupScaleAndBookDefaults(t *testing.T) {
	var output, errorOutput bytes.Buffer
	input := "uci\nsetoption name K4EvalScale value 100\nsetoption name OwnBook value true\nuci\nquit\n"
	args := []string{"-k4-eval-scale", "60", "-own-book=false"}
	if exit := run(args, strings.NewReader(input), &output, &errorOutput); exit != 0 {
		t.Fatalf("run exit=%d stderr=%s", exit, errorOutput.String())
	}
	for _, want := range []string{
		"option name K4EvalScale type spin default 60 min 10 max 400\n",
		"option name OwnBook type check default false\n",
	} {
		if strings.Count(output.String(), want) != 2 {
			t.Fatalf("startup default %q changed after a runtime option:\n%s", want, output.String())
		}
	}
}
