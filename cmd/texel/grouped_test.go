package main

import (
	"reflect"
	"testing"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/internal/texeldata"
)

func TestFinalTestLabelsCannotChangeFittedModelOrCalibration(t *testing.T) {
	baseline := engine.ExportTexelModel()
	restore := func() {
		t.Helper()
		if err := engine.ApplyTexelModel(baseline); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(restore)
	sample := func(fen string, label float64) engine.TexelSample {
		t.Helper()
		p, err := engine.ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		return engine.TexelSample{Board: p.Board, Result: label}
	}
	train := []engine.TexelSample{sample("7k/8/8/3n4/4P3/8/8/K7 w - - 0 1", 1), sample("7k/8/8/8/3r4/1N6/8/K7 w - - 0 1", 0)}
	validation := []engine.TexelSample{sample("7k/8/8/3r4/8/8/8/K2Q4 w - - 0 1", .75)}
	cfg := fitConfig{Epochs: 3, Patience: 0, LR: .5, L2: 0}
	var reports [2]groupedFitReport
	var models [2]engine.TexelModelExport
	for i, target := range []float64{0, 1} {
		restore()
		parts := texeldata.Partitions{Train: train, Validation: validation, Test: []engine.TexelSample{sample("7k/8/q7/8/8/8/R7/7K w - - 0 1", target)}}
		if err := fitPartitions(parts, cfg, &reports[i]); err != nil {
			t.Fatal(err)
		}
		models[i] = engine.ExportTexelModel()
	}
	if !reflect.DeepEqual(models[0], models[1]) || reports[0].K != reports[1].K || reports[0].FinalValidation != reports[1].FinalValidation {
		t.Fatal("final-test labels influenced training, calibration or checkpoint selection")
	}
	if reports[0].FinalTest == reports[1].FinalTest {
		t.Fatal("final-test measurement did not observe changed labels")
	}
}
