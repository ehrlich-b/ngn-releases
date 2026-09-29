package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

type BenchmarkSummary struct {
	Timestamp      time.Time     `json:"timestamp"`
	TotalPositions int           `json:"total_positions"`
	Successful     int           `json:"successful"`
	Failed         int           `json:"failed"`
	TotalTime      time.Duration `json:"total_time"`
	AvgNPS         int           `json:"avg_nps"`
	MinNPS         int           `json:"min_nps"`
	MaxNPS         int           `json:"max_nps"`
}

type TacticalSummary struct {
	Timestamp    time.Time     `json:"timestamp"`
	TotalTests   int           `json:"total_tests"`
	Correct      int           `json:"correct"`
	Accuracy     float64       `json:"accuracy"`
	AvgSolveTime time.Duration `json:"avg_solve_time"`
}

type EngineAssessment struct {
	ReportTime       time.Time        `json:"report_time"`
	Benchmark        BenchmarkSummary `json:"benchmark"`
	Tactical         TacticalSummary  `json:"tactical"`
	OverallScore     float64          `json:"overall_score"`
	Performance      string           `json:"performance"`
	TacticalStrength string           `json:"tactical_strength"`
	Recommendations  []string         `json:"recommendations"`
}

func loadBenchmarkResults(filename string) (*BenchmarkSummary, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var summary BenchmarkSummary
	if err := json.NewDecoder(file).Decode(&summary); err != nil {
		return nil, err
	}

	return &summary, nil
}

func loadTacticalResults(filename string) (*TacticalSummary, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var summary TacticalSummary
	if err := json.NewDecoder(file).Decode(&summary); err != nil {
		return nil, err
	}

	return &summary, nil
}

func assessPerformance(nps int) string {
	switch {
	case nps >= 500000:
		return "Excellent"
	case nps >= 300000:
		return "Very Good"
	case nps >= 200000:
		return "Good"
	case nps >= 100000:
		return "Fair"
	default:
		return "Needs Improvement"
	}
}

func assessTactical(accuracy float64) string {
	switch {
	case accuracy >= 80:
		return "Excellent"
	case accuracy >= 60:
		return "Very Good"
	case accuracy >= 40:
		return "Good"
	case accuracy >= 20:
		return "Fair"
	default:
		return "Needs Improvement"
	}
}

func calculateOverallScore(benchmark *BenchmarkSummary, tactical *TacticalSummary) float64 {
	// Weight performance at 40%, tactical at 60%
	perfScore := float64(benchmark.AvgNPS) / 400000.0 * 100 // Target 400K NPS
	if perfScore > 100 {
		perfScore = 100
	}

	tacticalScore := tactical.Accuracy

	return perfScore*0.4 + tacticalScore*0.6
}

func generateRecommendations(benchmark *BenchmarkSummary, tactical *TacticalSummary) []string {
	var recs []string

	if benchmark.AvgNPS < 300000 {
		recs = append(recs, "Performance optimization needed - focus on search speed improvements")
	}

	if tactical.Accuracy < 20 {
		recs = append(recs, "Critical: Tactical strength is very poor - prioritize move ordering improvements")
	}

	if tactical.Accuracy < 50 {
		recs = append(recs, "Improve move ordering to find tactical moves earlier in search")
	}

	if benchmark.AvgNPS >= 400000 && tactical.Accuracy >= 60 {
		recs = append(recs, "Engine is well-balanced - consider advanced features")
	}

	return recs
}

func runCombinedReport() error {
	fmt.Println("=== NGN ENGINE COMBINED ASSESSMENT ===")

	benchmarkFile := "../benchmark/benchmark_results.json"
	tacticalFile := "../tactical-test/tactical_results.json"

	// Load benchmark results
	benchmark, err := loadBenchmarkResults(benchmarkFile)
	if err != nil {
		fmt.Printf("Warning: Could not load benchmark results: %v\n", err)
		fmt.Println("Run 'make benchmark' first")
		return err
	}

	// Load tactical results
	tactical, err := loadTacticalResults(tacticalFile)
	if err != nil {
		fmt.Printf("Warning: Could not load tactical results: %v\n", err)
		fmt.Println("Run 'make tactical-test' first")
		return err
	}

	// Calculate assessments
	perfRating := assessPerformance(benchmark.AvgNPS)
	tacticalRating := assessTactical(tactical.Accuracy)
	overallScore := calculateOverallScore(benchmark, tactical)
	recommendations := generateRecommendations(benchmark, tactical)

	// Print report
	fmt.Printf("\nPERFORMANCE METRICS:\n")
	fmt.Printf("  Average NPS: %d (%s)\n", benchmark.AvgNPS, perfRating)
	fmt.Printf("  Min NPS: %d\n", benchmark.MinNPS)
	fmt.Printf("  Max NPS: %d\n", benchmark.MaxNPS)
	fmt.Printf("  Positions tested: %d\n", benchmark.TotalPositions)
	fmt.Printf("  Success rate: %.1f%%\n", float64(benchmark.Successful)/float64(benchmark.TotalPositions)*100)

	fmt.Printf("\nTACTICAL METRICS:\n")
	fmt.Printf("  Accuracy: %.1f%% (%s)\n", tactical.Accuracy, tacticalRating)
	fmt.Printf("  Correct solutions: %d/%d\n", tactical.Correct, tactical.TotalTests)
	fmt.Printf("  Average solve time: %v\n", tactical.AvgSolveTime.Round(time.Millisecond))

	fmt.Printf("\nOVERALL ASSESSMENT:\n")
	fmt.Printf("  Overall Score: %.1f/100\n", overallScore)

	if overallScore >= 80 {
		fmt.Println("  Rating: Excellent - Engine is performing well")
	} else if overallScore >= 60 {
		fmt.Println("  Rating: Good - Solid foundation with room for improvement")
	} else if overallScore >= 40 {
		fmt.Println("  Rating: Fair - Significant improvements needed")
	} else {
		fmt.Println("  Rating: Poor - Critical issues require immediate attention")
	}

	fmt.Printf("\nRECOMMENDATIONS:\n")
	for i, rec := range recommendations {
		fmt.Printf("  %d. %s\n", i+1, rec)
	}

	// Save combined assessment
	assessment := EngineAssessment{
		ReportTime:       time.Now(),
		Benchmark:        *benchmark,
		Tactical:         *tactical,
		OverallScore:     overallScore,
		Performance:      perfRating,
		TacticalStrength: tacticalRating,
		Recommendations:  recommendations,
	}

	assessmentFile, err := os.Create("engine_assessment.json")
	if err != nil {
		return fmt.Errorf("failed to create assessment file: %v", err)
	}
	defer assessmentFile.Close()

	encoder := json.NewEncoder(assessmentFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(assessment); err != nil {
		return fmt.Errorf("failed to save assessment: %v", err)
	}

	fmt.Printf("\nAssessment saved to: engine_assessment.json\n")

	return nil
}

func main() {
	if err := runCombinedReport(); err != nil {
		log.Fatalf("Combined report failed: %v", err)
	}
}
