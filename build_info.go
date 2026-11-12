package main

import (
	"crypto/sha256"
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"

	"github.com/ehrlich-b/ngn/engine"
)

// buildInfo uses null for metadata that the Go toolchain did not supply.
type buildInfo struct {
	Revision            *string `json:"revision"`
	Dirty               *bool   `json:"dirty"`
	GOOS                string  `json:"goos"`
	GOARCH              string  `json:"goarch"`
	GoVersion           string  `json:"go_version"`
	DefaultEvaluator    string  `json:"default_evaluator"`
	EmbeddedModelSHA256 *string `json:"embedded_model_sha256"`
}

func makeBuildInfo(metadata *debug.BuildInfo, model []byte, profile string) buildInfo {
	info := buildInfo{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoVersion: runtime.Version(), DefaultEvaluator: profile}
	if metadata != nil {
		for _, setting := range metadata.Settings {
			switch setting.Key {
			case "vcs.revision":
				if setting.Value != "" {
					value := setting.Value
					info.Revision = &value
				}
			case "vcs.modified":
				if value, err := strconv.ParseBool(setting.Value); err == nil {
					info.Dirty = &value
				}
			}
		}
	}
	if len(model) != 0 {
		digest := fmt.Sprintf("%x", sha256.Sum256(model))
		info.EmbeddedModelSHA256 = &digest
	}
	return info
}

func currentBuildInfo() buildInfo {
	metadata, _ := debug.ReadBuildInfo()
	evaluator := "ngnn3"
	if releaseProfile == "hce" {
		evaluator = engine.EvaluatorBackendHCEName
	}
	return makeBuildInfo(metadata, engine.DefaultNetBytes(), evaluator)
}
