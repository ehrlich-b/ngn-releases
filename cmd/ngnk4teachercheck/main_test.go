package main

import (
	"math"
	"testing"
)

func TestTeacherQualityBoundKeepsProtocolExceptionsVisible(t *testing.T) {
	results := make([]result, sampleSize)
	for i := range results {
		results[i].sample.Target5K = 0.5
		results[i].sample.Score5K = 100
		target, score := 0.51, 105
		results[i].Target20K, results[i].Score20K = &target, &score
	}
	for i := 945; i < 990; i++ {
		results[i].Target20K, results[i].Score20K = nil, nil
		results[i].Reason = "pv-bestmove-mismatch"
	}
	for i := 990; i < sampleSize; i++ {
		results[i].Target20K, results[i].Score20K = nil, nil
		results[i].Target5K = 0.99
		results[i].Reason = "mate-score"
		mate := 3
		results[i].MatePly20K = &mate
	}
	s := summarize(results)
	if s.Status != "PASS_BOUND" || s.Comparable != 945 || s.PVMismatches != 45 || s.MateScores != 10 {
		t.Fatalf("summary = %+v", s)
	}
	want := (945*0.01 + 45*0.5 + 10*0.01) / 1000.0
	if math.Abs(s.WorstCaseMeanBound-want) > 1e-12 {
		t.Fatalf("worst-case bound = %.15f, want %.15f", s.WorstCaseMeanBound, want)
	}
	// A missing mate sign uses the full [0,1] bound; this fixture still passes.
	results[990].MatePly20K = nil
	s = summarize(results)
	if s.Status != "PASS_BOUND" { // still below the cutoff even at [0,1]
		t.Fatalf("conservative missing-sign bound should remain below cutoff: %+v", s)
	}
	for i := 0; i < 945; i++ {
		target := 0.53
		results[i].Target20K = &target
	}
	s = summarize(results)
	if s.Status != "HOLD" || s.WorstCaseMeanBound <= s.MeanGateMaximum {
		t.Fatalf("high worst-case error must hold: %+v", s)
	}
}

func TestSpearmanUsesAverageRanksForTies(t *testing.T) {
	if got := spearman([]int{1, 1, 3, 4}, []int{2, 2, 4, 5}); math.Abs(got-1) > 1e-12 {
		t.Fatalf("identical tied order correlation = %g", got)
	}
}
