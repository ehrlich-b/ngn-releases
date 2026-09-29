//go:build counteroracle

package countereval

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"
)

var counterOutputDotBenchmarkSink float32

const counterOutputDotCapturedCorpusRecords = 12

func BenchmarkCounterOutputDotCapturedCorpus(b *testing.B) {
	modelPath := os.Getenv("COUNTER_MODEL")
	if modelPath == "" {
		b.Fatal("COUNTER_MODEL is required")
	}
	oraclePath := os.Getenv("COUNTER_ORACLE_JSON")
	if oraclePath == "" {
		b.Fatal("COUNTER_ORACLE_JSON is required")
	}

	modelBytes, err := os.ReadFile(modelPath)
	if err != nil {
		b.Fatal(err)
	}
	model, metadata, err := LoadCounter55Legacy(bytes.NewReader(modelBytes))
	if err != nil {
		b.Fatal(err)
	}
	if metadata.SHA256 != pinnedCounter55SHA256 {
		b.Fatalf("model SHA256=%s want=%s", metadata.SHA256, pinnedCounter55SHA256)
	}

	encoded, err := os.ReadFile(oraclePath)
	if err != nil {
		b.Fatal(err)
	}
	var oracle parityOracle
	if err := json.Unmarshal(encoded, &oracle); err != nil {
		b.Fatal(err)
	}
	if oracle.Schema != "counter55-portable-oracle-v1" || oracle.ModelSHA256 != pinnedCounter55SHA256 {
		b.Fatalf("oracle identity mismatch: schema=%q model=%q", oracle.Schema, oracle.ModelSHA256)
	}
	accumulators := make([][HiddenSize]float32, len(oracle.Records))
	for recordIndex, record := range oracle.Records {
		if len(record.AccumulatorBits) != HiddenSize {
			b.Fatalf("fixture %s accumulator lanes=%d want=%d", record.ID, len(record.AccumulatorBits), HiddenSize)
		}
		for lane, bits := range record.AccumulatorBits {
			accumulators[recordIndex][lane] = math.Float32frombits(bits)
		}
	}
	if len(accumulators) != counterOutputDotCapturedCorpusRecords {
		b.Fatalf("oracle corpus records=%d want=%d", len(accumulators), counterOutputDotCapturedCorpusRecords)
	}

	order := []string{"portable", "selected"}
	switch value := os.Getenv("NGN_COUNTER_OUTPUT_BENCH_ORDER"); value {
	case "", "portable-first":
	case "selected-first":
		order[0], order[1] = order[1], order[0]
	default:
		b.Fatalf("NGN_COUNTER_OUTPUT_BENCH_ORDER=%q; want portable-first or selected-first", value)
	}
	for _, implementation := range order {
		switch implementation {
		case "portable":
			b.Run(implementation, func(b *testing.B) {
				benchmarkCounterOutputDotPortable(b, accumulators, &model.outputWeights)
			})
		case "selected":
			b.Run(implementation, func(b *testing.B) {
				benchmarkCounterOutputDotSelected(b, accumulators, &model.outputWeights)
			})
		}
	}
}

func benchmarkCounterOutputDotPortable(b *testing.B, accumulators [][HiddenSize]float32, weights *[HiddenSize]float32) {
	b.ReportAllocs()
	b.ResetTimer()
	position := 0
	var output float32
	for iteration := 0; iteration < b.N; iteration++ {
		output = counterOutputDotPortable(&accumulators[position], weights)
		position++
		if position == len(accumulators) {
			position = 0
		}
	}
	counterOutputDotBenchmarkSink = output
}

func benchmarkCounterOutputDotSelected(b *testing.B, accumulators [][HiddenSize]float32, weights *[HiddenSize]float32) {
	b.ReportAllocs()
	b.ResetTimer()
	position := 0
	var output float32
	for iteration := 0; iteration < b.N; iteration++ {
		output = counterOutputDot(&accumulators[position], weights)
		position++
		if position == len(accumulators) {
			position = 0
		}
	}
	counterOutputDotBenchmarkSink = output
}
