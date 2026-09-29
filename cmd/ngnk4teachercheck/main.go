// ngnk4teachercheck runs the prospectively required 5k-versus-20k SF18
// comparison on 1,000 identity-hash-selected positions from the frozen
// calibration corpus. It never produces training labels.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"time"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/training/nnue/k4finalize"
	"github.com/ehrlich-b/ngn/training/nnue/k4label"
)

const sampleSize = 1000

type sample struct {
	ID       string  `json:"id"`
	K4Key    string  `json:"k4_input_sha256"`
	FEN      string  `json:"reset_fen"`
	Score5K  int     `json:"score_5k"`
	Target5K float64 `json:"target_5k"`
	UCICP5K  int     `json:"uci_cp_5k"`
	LabelSHA string  `json:"label_sha256"`
}

type result struct {
	sample
	Score20K       *int     `json:"score_20k,omitempty"`
	Target20K      *float64 `json:"target_20k,omitempty"`
	UCICP20K       *int     `json:"uci_cp_20k,omitempty"`
	LastExactCP20K *int     `json:"last_exact_cp_20k,omitempty"`
	MatePly20K     *int     `json:"mate_ply_20k,omitempty"`
	Depth20K       int      `json:"depth_20k,omitempty"`
	Nodes20K       uint64   `json:"reported_nodes_20k,omitempty"`
	BestMove       string   `json:"best_move_20k,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}

type summary struct {
	Status               string  `json:"status"`
	SampleCount          int     `json:"sample_count"`
	Comparable           int     `json:"comparable"`
	MeanAbsTargetDelta   float64 `json:"mean_abs_target_delta"`
	MedianAbsTargetDelta float64 `json:"median_abs_target_delta"`
	P90AbsTargetDelta    float64 `json:"p90_abs_target_delta"`
	SpearmanScore        float64 `json:"spearman_score"`
	SignEligible50       int     `json:"sign_eligible_50"`
	SignDisagree50       int     `json:"sign_disagree_50"`
	SignEligible100      int     `json:"sign_eligible_100"`
	SignDisagree100      int     `json:"sign_disagree_100"`
	MeanGateMaximum      float64 `json:"mean_gate_maximum"`
	PVMismatches         int     `json:"pv_bestmove_mismatches"`
	MateScores           int     `json:"mate_scores"`
	WorstCaseMeanBound   float64 `json:"worst_case_mean_abs_target_delta_bound"`
	BoundConvention      string  `json:"bound_convention"`
}

type report struct {
	Schema             string               `json:"schema"`
	SampleMethod       string               `json:"sample_method"`
	ManifestSHA256     string               `json:"manifest_sha256"`
	SampleSHA256       string               `json:"sample_sha256"`
	TeacherSHA256      string               `json:"teacher_sha256"`
	TeacherSource      string               `json:"teacher_source_commit"`
	TeacherBigNetSHA   string               `json:"teacher_big_net_sha256"`
	TeacherSmallNetSHA string               `json:"teacher_small_net_sha256"`
	Search             k4label.SearchConfig `json:"search"`
	Summary            summary              `json:"summary"`
	Results            []result             `json:"results"`
}

func fileSHA(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil)), n, err
}

func strictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}

func loadSamples(manifestPath, expectedSHA string) ([]sample, string, error) {
	actualSHA, _, err := fileSHA(manifestPath)
	if err != nil {
		return nil, "", err
	}
	if actualSHA != expectedSHA {
		return nil, "", fmt.Errorf("finalized manifest SHA mismatch: %s", actualSHA)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, "", err
	}
	var manifest k4finalize.Manifest
	if err := strictJSON(data, &manifest); err != nil {
		return nil, "", err
	}
	if manifest.Schema != "ngn-k4-finalized-corpus-v1" || manifest.State != "COMPLETE" || manifest.Mode != "pilot" {
		return nil, "", fmt.Errorf("not a completed K4 pilot manifest")
	}
	var calibrationRecords uint64
	for _, corpus := range manifest.Corpora {
		if corpus.Name == "calibration" && corpus.Split == "calibration" {
			calibrationRecords = corpus.Records
		}
	}
	if calibrationRecords != 100_000 {
		return nil, "", fmt.Errorf("calibration corpus has %d records", calibrationRecords)
	}
	all := make([]sample, 0, calibrationRecords)
	seen := make(map[string]bool, calibrationRecords)
	for _, shard := range manifest.UsedShards {
		if shard.Stage != "fixed" || shard.Split != "calibration" {
			continue
		}
		if shard.AcceptedUsed <= 0 || shard.AcceptedUsed > shard.AcceptedAvailable {
			return nil, "", fmt.Errorf("invalid accepted count in %s", shard.ShardID)
		}
		sha, n, err := fileSHA(shard.Labels.Path)
		if err != nil {
			return nil, "", err
		}
		if sha != shard.Labels.SHA256 || n != shard.Labels.Bytes {
			return nil, "", fmt.Errorf("label receipt mismatch in %s", shard.ShardID)
		}
		file, err := os.Open(shard.Labels.Path)
		if err != nil {
			return nil, "", err
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 4<<20)
		if !scanner.Scan() {
			file.Close()
			return nil, "", fmt.Errorf("missing label header in %s", shard.ShardID)
		}
		var header k4label.OutputHeader
		if err := strictJSON(scanner.Bytes(), &header); err != nil {
			file.Close()
			return nil, "", err
		}
		if header.Type != "header" || header.Schema != k4label.OutputSchema || header.ContractVersion != k4label.ContractVersion ||
			header.Input.ShardID != shard.ShardID || header.Input.Split != "calibration" || header.InputSHA256 != shard.SamplerInput.SHA256 ||
			header.Search != k4label.FrozenSearchConfig() || header.Teacher.SourceCommit != k4label.TeacherSourceCommit ||
			header.Teacher.BigNetworkSHA256 != k4label.TeacherBigNetworkSHA || header.Teacher.SmallNetworkSHA256 != k4label.TeacherSmallNetSHA ||
			header.Teacher.ExecutableSHA256 != k4finalize.TeacherExecutableSHA {
			file.Close()
			return nil, "", fmt.Errorf("label header contract mismatch in %s", shard.ShardID)
		}
		used := 0
		for scanner.Scan() && used < shard.AcceptedUsed {
			var envelope struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
				file.Close()
				return nil, "", err
			}
			if envelope.Type != "label" {
				file.Close()
				return nil, "", fmt.Errorf("early label footer in %s", shard.ShardID)
			}
			var record k4label.LabelRecord
			if err := strictJSON(scanner.Bytes(), &record); err != nil {
				file.Close()
				return nil, "", err
			}
			if record.Split != "calibration" {
				file.Close()
				return nil, "", fmt.Errorf("wrong split in %s", shard.ShardID)
			}
			if record.Status != "accepted" {
				continue
			}
			used++
			if len(record.ID) != 64 || len(record.K4InputSHA256) != 64 || seen[record.ID] {
				file.Close()
				return nil, "", fmt.Errorf("bad or duplicate label ID in %s", shard.ShardID)
			}
			seen[record.ID] = true
			position, err := engine.ParseFEN(record.ResetFEN)
			if err != nil {
				file.Close()
				return nil, "", err
			}
			score, target, _, _, err := k4label.ScoreTarget(position, record.UCICP)
			if err != nil || score != int(record.NGNScore) || math.Float64bits(target) != math.Float64bits(record.Target) {
				file.Close()
				return nil, "", fmt.Errorf("5k score contract mismatch for %s", record.ID)
			}
			all = append(all, sample{ID: record.ID, K4Key: record.K4InputSHA256, FEN: record.ResetFEN,
				Score5K: score, Target5K: target, UCICP5K: record.UCICP, LabelSHA: record.RecordSHA256})
		}
		if err := scanner.Err(); err != nil {
			file.Close()
			return nil, "", err
		}
		if err := file.Close(); err != nil {
			return nil, "", err
		}
		if used != shard.AcceptedUsed {
			return nil, "", fmt.Errorf("too few accepted records in %s", shard.ShardID)
		}
	}
	if uint64(len(all)) != calibrationRecords {
		return nil, "", fmt.Errorf("calibration accepted count %d, want %d", len(all), calibrationRecords)
	}
	// Identity-hash sampling ignores the teacher score, FEN, model and result.
	// It spreads the sample across the whole finalized corpus rather than taking
	// adjacent entries from the first encoded chains.
	type ranked struct {
		sample sample
		key    [sha256.Size]byte
	}
	pool := make([]ranked, len(all))
	for i, item := range all {
		pool[i] = ranked{sample: item, key: sha256.Sum256([]byte("ngn-k4-teacher-quality-sample-v1\x00" + item.ID))}
	}
	sort.Slice(pool, func(i, j int) bool {
		if order := bytes.Compare(pool[i].key[:], pool[j].key[:]); order != 0 {
			return order < 0
		}
		return pool[i].sample.ID < pool[j].sample.ID
	})
	selected := make([]sample, sampleSize)
	for i := range selected {
		selected[i] = pool[i].sample
	}
	canonical, err := json.Marshal(selected)
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(canonical)
	return selected, hex.EncodeToString(digest[:]), nil
}

func sign(value int) int {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return math.NaN()
	}
	x := p * float64(len(sorted)-1)
	lo := int(x)
	hi := min(lo+1, len(sorted)-1)
	return sorted[lo] + (x-float64(lo))*(sorted[hi]-sorted[lo])
}

func ranks(values []int) []float64 {
	indices := make([]int, len(values))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool { return values[indices[i]] < values[indices[j]] })
	result := make([]float64, len(values))
	for start := 0; start < len(indices); {
		end := start + 1
		for end < len(indices) && values[indices[end]] == values[indices[start]] {
			end++
		}
		rank := (float64(start+1) + float64(end)) / 2
		for _, index := range indices[start:end] {
			result[index] = rank
		}
		start = end
	}
	return result
}

func spearman(left, right []int) float64 {
	if len(left) < 2 || len(left) != len(right) {
		return 0
	}
	a, b := ranks(left), ranks(right)
	var sumA, sumB float64
	for i := range a {
		sumA += a[i]
		sumB += b[i]
	}
	meanA, meanB := sumA/float64(len(a)), sumB/float64(len(b))
	var covariance, varianceA, varianceB float64
	for i := range a {
		da, db := a[i]-meanA, b[i]-meanB
		covariance += da * db
		varianceA += da * da
		varianceB += db * db
	}
	if varianceA == 0 || varianceB == 0 {
		return 0
	}
	return covariance / math.Sqrt(varianceA*varianceB)
}

func summarize(results []result) summary {
	s := summary{Status: "HOLD", SampleCount: len(results), MeanGateMaximum: 0.05,
		BoundConvention: "matched exact cp uses observed target; PV/bestmove mismatches permit any target in [0,1]; mate scores map positive mate to 1 and negative mate to 0"}
	deltas := make([]float64, 0, len(results))
	left, right := make([]int, 0, len(results)), make([]int, 0, len(results))
	for _, item := range results {
		if item.Score20K == nil || item.Target20K == nil {
			switch item.Reason {
			case "pv-bestmove-mismatch":
				s.PVMismatches++
				s.WorstCaseMeanBound += math.Max(item.Target5K, 1-item.Target5K)
			case "mate-score":
				s.MateScores++
				if item.MatePly20K == nil || *item.MatePly20K == 0 {
					s.WorstCaseMeanBound += math.Max(item.Target5K, 1-item.Target5K)
				} else if *item.MatePly20K > 0 {
					s.WorstCaseMeanBound += 1 - item.Target5K
				} else {
					s.WorstCaseMeanBound += item.Target5K
				}
			default:
				s.WorstCaseMeanBound += math.Max(item.Target5K, 1-item.Target5K)
			}
			continue
		}
		s.Comparable++
		delta := math.Abs(item.Target5K - *item.Target20K)
		deltas = append(deltas, delta)
		s.WorstCaseMeanBound += delta
		left, right = append(left, item.Score5K), append(right, *item.Score20K)
		for _, threshold := range []int{50, 100} {
			if abs(item.Score5K) < threshold {
				continue
			}
			if threshold == 50 {
				s.SignEligible50++
				if sign(item.Score5K) != sign(*item.Score20K) {
					s.SignDisagree50++
				}
			}
			if threshold == 100 {
				s.SignEligible100++
				if sign(item.Score5K) != sign(*item.Score20K) {
					s.SignDisagree100++
				}
			}
		}
	}
	if len(deltas) == 0 {
		return s
	}
	s.WorstCaseMeanBound /= float64(len(results))
	sort.Float64s(deltas)
	for _, delta := range deltas {
		s.MeanAbsTargetDelta += delta
	}
	s.MeanAbsTargetDelta /= float64(len(deltas))
	s.MedianAbsTargetDelta = percentile(deltas, 0.5)
	s.P90AbsTargetDelta = percentile(deltas, 0.9)
	s.SpearmanScore = spearman(left, right)
	if s.Comparable == sampleSize && s.MeanAbsTargetDelta <= s.MeanGateMaximum {
		s.Status = "PASS"
	} else if s.Comparable+s.PVMismatches+s.MateScores == sampleSize && s.WorstCaseMeanBound <= s.MeanGateMaximum {
		s.Status = "PASS_BOUND"
	}
	return s
}

func writeNew(path string, value any) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		file.Close()
		os.Remove(path)
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(path)
		return err
	}
	return file.Close()
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("ngnk4teachercheck", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "", "frozen finalized pilot manifest")
	manifestSHA := flags.String("manifest-sha256", "", "expected finalized manifest SHA-256")
	teacherPath := flags.String("teacher", "", "pinned Stockfish 18 binary")
	outputPath := flags.String("output", "", "new audit JSON path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *manifestPath == "" || *manifestSHA == "" || *teacherPath == "" || *outputPath == "" {
		return errors.New("required: -manifest -manifest-sha256 -teacher -output")
	}
	if _, err := os.Stat(*outputPath); err == nil {
		return fmt.Errorf("refusing existing output %s", *outputPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	samples, sampleSHA, err := loadSamples(*manifestPath, *manifestSHA)
	if err != nil {
		return err
	}
	teacherSHA, _, err := fileSHA(*teacherPath)
	if err != nil {
		return err
	}
	if teacherSHA != k4finalize.TeacherExecutableSHA {
		return fmt.Errorf("teacher executable SHA mismatch: %s", teacherSHA)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	session, err := k4label.StartDiagnosticSession(ctx, []string{*teacherPath}, k4label.TeacherSourceCommit,
		k4label.TeacherBigNetworkSHA, k4label.TeacherSmallNetSHA)
	if err != nil {
		return err
	}
	results := make([]result, 0, sampleSize)
	for _, item := range samples {
		observation := result{sample: item}
		search, reason, err := session.Analyze(ctx, item.FEN)
		if err != nil {
			session.Close()
			return fmt.Errorf("20k teacher %s: %w", item.ID, err)
		}
		observation.Depth20K, observation.Nodes20K, observation.BestMove = search.Depth, search.Nodes, search.BestMove
		if reason == "" && search.PVMove != search.BestMove {
			reason = "pv-bestmove-mismatch"
		}
		if reason == "pv-bestmove-mismatch" {
			observation.LastExactCP20K = &search.CP
		} else if reason == "mate-score" {
			observation.MatePly20K = &search.CP
		}
		if reason == "" {
			position, err := engine.ParseFEN(item.FEN)
			if err != nil {
				session.Close()
				return err
			}
			score, target, _, _, err := k4label.ScoreTarget(position, search.CP)
			if err != nil || score < -k4label.MaximumTargetScore || score > k4label.MaximumTargetScore {
				reason = "score-out-of-range"
			} else {
				observation.Score20K, observation.Target20K, observation.UCICP20K = &score, &target, &search.CP
			}
		}
		observation.Reason = reason
		results = append(results, observation)
	}
	if err := session.Close(); err != nil {
		return err
	}
	config := k4label.FrozenSearchConfig()
	config.Nodes, config.TimeoutMillis = 20_000, 8_000
	finished := report{Schema: "ngn-k4-teacher-quality-v1", SampleMethod: "first 1000 SHA256(ngn-k4-teacher-quality-sample-v1\\0 || accepted_label_id) among finalized calibration records",
		ManifestSHA256: *manifestSHA, SampleSHA256: sampleSHA,
		TeacherSHA256: teacherSHA, TeacherSource: k4label.TeacherSourceCommit,
		TeacherBigNetSHA: k4label.TeacherBigNetworkSHA, TeacherSmallNetSHA: k4label.TeacherSmallNetSHA,
		Search: config, Summary: summarize(results), Results: results}
	if err := writeNew(*outputPath, finished); err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(finished.Summary)
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "ngnk4teachercheck:", err)
		os.Exit(1)
	}
}
