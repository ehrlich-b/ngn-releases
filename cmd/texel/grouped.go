package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/internal/texeldata"
)

type groupedFitReport struct {
	Stage             string
	ModelVersion      string
	SplitSeed         string
	Corpus            texeldata.Report
	Coverage          engine.TexelCoverage
	Config            fitConfig
	K                 float64
	InitialTrain      float64
	InitialValidation float64
	FinalTrain        float64
	FinalValidation   float64
	BaselineTest      float64
	FinalTest         float64
}

type fitConfig struct {
	Epochs, Patience int
	LR, L2           float64
}

// groupedGradient never sends the final test partition to K calibration or
// checkpoint selection. Test loss is computed once, after fitting is finished.
func groupedGradient(paths, splitSeed, out, reportPath string, cfg fitConfig, auditOnly bool) error {
	if paths == "" || reportPath == "" || (!auditOnly && out == "") {
		return fmt.Errorf("grouped data requires -corpus, -report, and (for fitting) -out")
	}
	if !auditOnly && (cfg.Epochs <= 0 || cfg.Patience < 0 || cfg.LR <= 0 || cfg.L2 < 0 || math.IsNaN(cfg.LR) || math.IsInf(cfg.LR, 0) || math.IsNaN(cfg.L2) || math.IsInf(cfg.L2, 0)) {
		return fmt.Errorf("invalid grouped fit hyperparameters")
	}
	var files []string
	for _, path := range strings.Split(paths, ",") {
		if path = strings.TrimSpace(path); path != "" {
			files = append(files, path)
		}
	}
	// Never let a result path overwrite another result or any input corpus.
	used := make(map[string]bool)
	for _, path := range files {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		used[absolute] = true
	}
	for _, path := range []string{out, reportPath} {
		if path == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if used[absolute] {
			return fmt.Errorf("output path overlaps an input or other output: %s", path)
		}
		used[absolute] = true
	}
	parts, corpus, err := texeldata.Load(files, texeldata.LoadOptions{Seed: splitSeed})
	if err != nil {
		return err
	}
	report := groupedFitReport{Stage: "audit_only", ModelVersion: engine.TexelModelVersion, SplitSeed: splitSeed, Corpus: corpus, Config: cfg}
	report.Coverage = engine.TexelTrainingCoverage(parts.Train)
	if auditOnly {
		return writeJSON(reportPath, report)
	}
	if len(parts.Train) == 0 || len(parts.Validation) == 0 || len(parts.Test) == 0 {
		return fmt.Errorf("all three partitions must be nonempty: train=%d validation=%d test=%d", len(parts.Train), len(parts.Validation), len(parts.Test))
	}
	if err := fitPartitions(parts, cfg, &report); err != nil {
		return err
	}
	report.Stage = "fit_complete"
	if err := writeJSON(out, engine.ExportTexelModel()); err != nil {
		return err
	}
	return writeJSON(reportPath, report)
}

func fitPartitions(parts texeldata.Partitions, cfg fitConfig, report *groupedFitReport) error {
	baseline := engine.ExportTexelModel()
	report.K = engine.TexelFindK(parts.Train)
	report.InitialTrain = engine.TexelModelMSE(parts.Train, report.K)
	report.InitialValidation = engine.TexelModelMSE(parts.Validation, report.K)
	start := time.Now()
	var err error
	report.FinalTrain, report.FinalValidation, err = engine.TryTexelGradientTune(parts.Train, parts.Validation, engine.TexelGradientConfig{
		K: report.K, LR: cfg.LR, Epochs: cfg.Epochs, Patience: cfg.Patience, L2: cfg.L2,
		Progress: func(epoch int, train, validation float64) {
			if epoch == 1 || epoch%10 == 0 {
				fmt.Fprintf(os.Stderr, "epoch=%d trainMSE=%.8f validationMSE=%.8f elapsed=%s\n", epoch, train, validation, time.Since(start).Round(time.Second))
			}
		},
	})
	if err != nil {
		return err
	}
	// Compare candidate and baseline on the final test set only after checkpoint
	// selection has ended. Neither test loss feeds back into fitting.
	model := engine.ExportTexelModel()
	report.FinalTest = engine.TexelModelMSE(parts.Test, report.K)
	if err := engine.ApplyTexelModel(baseline); err != nil {
		return err
	}
	report.BaselineTest = engine.TexelModelMSE(parts.Test, report.K)
	if err := engine.ApplyTexelModel(model); err != nil {
		return err
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".texel-result-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
