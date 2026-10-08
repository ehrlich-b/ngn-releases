package claim

import (
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"reflect"
	"sort"
)

// PrimaryFormula is the protocol's displayed primary formula, verbatim.
const PrimaryFormula = `p_i = (W_i + 0.5*D_i + 0.5)/(N_i + 1)
R_i = label_i + 400*log10(p_i/(1-p_i))
v_i = (400/ln(10))^2/(N_i*p_i*(1-p_i))
weight_i = 1/v_i
R_pool = sum(weight_i*R_i)/sum(weight_i)
SE_pool = 1/sqrt(sum(weight_i))
CI95 = [R_pool - 1.96*SE_pool, R_pool + 1.96*SE_pool]`

type AnchorAnalysis struct {
	Name                                       string
	Label                                      int
	Counts                                     Counts
	RawScore, P, Performance, Variance, Weight float64
}
type PoolAnalysis struct {
	Estimate, SE float64
	CI95         [2]float64
}
type BootstrapAnalysis struct {
	Indices, Draws int
	Seed           int64
	Method         string
	CI95           [2]float64
}
type Analysis struct {
	Schema, Mode, ReceiptSHA256, Formula string
	GameDirectory                        string
	Audit                                Artifact
	// Bind every replayed game to the saved audit, not the collector summary.
	Games                                    map[string]string
	Anchors                                  []AnchorAnalysis
	Primary                                  PoolAnalysis
	Bootstrap                                BootstrapAnalysis
	Bracketed, ScreenEligible, ClaimEligible bool
	Exclusions                               int
	Interpretation, Questions                []string
}

func Primary(anchor Anchor, c Counts) (AnchorAnalysis, error) {
	a := AnchorAnalysis{Name: anchor.Name, Label: anchor.Label, Counts: c}
	if c.W < 0 || c.D < 0 || c.L < 0 || c.N <= 0 || c.N != c.W+c.D+c.L {
		return a, fmt.Errorf("invalid W/D/L for %s", anchor.Name)
	}
	a.RawScore = (float64(c.W) + 0.5*float64(c.D)) / float64(c.N)
	a.P = (float64(c.W) + 0.5*float64(c.D) + 0.5) / (float64(c.N) + 1)
	a.Performance = float64(anchor.Label) + 400*math.Log10(a.P/(1-a.P))
	a.Variance = math.Pow(400/math.Log(10), 2) / (float64(c.N) * a.P * (1 - a.P))
	a.Weight = 1 / a.Variance
	return a, nil
}
func Pool(anchors []AnchorAnalysis) (PoolAnalysis, error) {
	var p PoolAnalysis
	if len(anchors) == 0 {
		return p, fmt.Errorf("empty anchor pool")
	}
	sum, total := 0.0, 0.0
	for _, a := range anchors {
		if a.Weight <= 0 || math.IsNaN(a.Weight) || math.IsInf(a.Weight, 0) || math.IsNaN(a.Performance) || math.IsInf(a.Performance, 0) {
			return p, fmt.Errorf("invalid pool term")
		}
		sum += a.Weight * a.Performance
		total += a.Weight
	}
	p.Estimate = sum / total
	p.SE = 1 / math.Sqrt(total)
	p.CI95 = [2]float64{p.Estimate - 1.96*p.SE, p.Estimate + 1.96*p.SE}
	return p, nil
}
func bracketed(anchors []AnchorAnalysis) bool {
	above, below := false, false
	for _, a := range anchors {
		above = above || a.RawScore > 0.5
		below = below || a.RawScore < 0.5
	}
	return above && below
}
func eligible(mode string, anchors []AnchorAnalysis, p PoolAnalysis, admitted bool, exclusions int) (bool, bool) {
	if !admitted || exclusions != 0 || !bracketed(anchors) || !(p.CI95[0] > 3000) {
		return false, false
	}
	expected := 80
	if mode == "screen" {
		expected = 40
	} else if mode != "confirmation" {
		return false, false
	}
	fixed := Anchors()
	if len(anchors) != len(fixed) {
		return false, false
	}
	for i, a := range anchors {
		if a.Name != fixed[i].Name || a.Label != fixed[i].Label || a.Counts.N != 2*expected || a.Counts.Pairs != expected {
			return false, false
		}
	}
	return mode == "screen", mode == "confirmation"
}

// openingClusters builds one score vector per opening, covering all anchors and
// both colors. A bootstrap draw selects this entire vector exactly once.
func openingClusters(anchors []Anchor, games []Game, indices int) ([][]Counts, error) {
	if indices <= 0 || len(anchors) == 0 {
		return nil, fmt.Errorf("empty opening sample")
	}
	order := map[string]int{}
	for i, a := range anchors {
		if _, ok := order[a.Name]; ok {
			return nil, fmt.Errorf("duplicate anchor")
		}
		order[a.Name] = i
	}
	clusters := make([][]Counts, indices)
	colors := make([][]uint8, indices)
	for i := range clusters {
		clusters[i] = make([]Counts, len(anchors))
		colors[i] = make([]uint8, len(anchors))
	}
	for _, g := range games {
		j, ok := order[g.Anchor]
		i := g.Opening.Index
		if !ok || i < 0 || i >= indices {
			return nil, fmt.Errorf("game outside frozen pool/indices")
		}
		bit := uint8(1)
		if g.CandidateColor == "black" {
			bit = 2
		} else if g.CandidateColor != "white" {
			return nil, fmt.Errorf("bad game color")
		}
		if colors[i][j]&bit != 0 {
			return nil, fmt.Errorf("duplicate opening/color")
		}
		colors[i][j] |= bit
		c := &clusters[i][j]
		c.N++
		switch g.Result {
		case "1/2-1/2":
			c.D++
		case "1-0":
			if g.CandidateColor == "white" {
				c.W++
			} else {
				c.L++
			}
		case "0-1":
			if g.CandidateColor == "black" {
				c.W++
			} else {
				c.L++
			}
		default:
			return nil, fmt.Errorf("unknown result")
		}
		if g.Reason == "ply-cap" {
			c.CapDraws++
		}
	}
	for i := range clusters {
		for j := range anchors {
			if colors[i][j] != 3 {
				return nil, fmt.Errorf("opening %d missing intact color pair for %s", i, anchors[j].Name)
			}
			clusters[i][j].Pairs = 1
		}
	}
	return clusters, nil
}
func resampledCounts(clusters [][]Counts, selected []int) []Counts {
	out := make([]Counts, len(clusters[0]))
	for _, i := range selected {
		for j, c := range clusters[i] {
			out[j].W += c.W
			out[j].D += c.D
			out[j].L += c.L
			out[j].N += c.N
			out[j].Pairs += c.Pairs
			out[j].CapDraws += c.CapDraws
		}
	}
	return out
}
func poolCounts(anchors []Anchor, counts []Counts) (PoolAnalysis, error) {
	values := make([]AnchorAnalysis, len(anchors))
	for i, a := range anchors {
		v, e := Primary(a, counts[i])
		if e != nil {
			return PoolAnalysis{}, e
		}
		values[i] = v
	}
	return Pool(values)
}
func percentile(sorted []float64, p float64) float64 {
	// Linear interpolation between ordered values (Hyndman-Fan type 7).
	x := p * float64(len(sorted)-1)
	lo := int(math.Floor(x))
	hi := int(math.Ceil(x))
	return sorted[lo] + (x-float64(lo))*(sorted[hi]-sorted[lo])
}
func Bootstrap(anchors []Anchor, games []Game, indices, draws int, seed int64) (BootstrapAnalysis, error) {
	out := BootstrapAnalysis{Indices: indices, Draws: draws, Seed: seed, Method: "whole-opening jointly across anchors, both colors retained; percentile CI with linear interpolation (type 7)"}
	if draws < 2 {
		return out, fmt.Errorf("at least two bootstrap draws required")
	}
	clusters, e := openingClusters(anchors, games, indices)
	if e != nil {
		return out, e
	}
	rng := rand.New(rand.NewSource(seed))
	values := make([]float64, draws)
	selected := make([]int, indices)
	for d := range values {
		for i := range selected {
			selected[i] = rng.Intn(indices)
		}
		p, e := poolCounts(anchors, resampledCounts(clusters, selected))
		if e != nil {
			return out, e
		}
		values[d] = p.Estimate
	}
	sort.Float64s(values)
	out.CI95 = [2]float64{percentile(values, 0.025), percentile(values, 0.975)}
	return out, nil
}

// Analyze accepts persisted, hash-bound evidence only. Replaying it anew prevents
// an edited audit flag or collector summary from manufacturing admission.
func Analyze(receiptPath, auditPath, gameDir string) (Analysis, error) {
	out := Analysis{Schema: Schema, Formula: PrimaryFormula}
	absoluteDir, e := filepath.Abs(gameDir)
	if e != nil {
		return out, e
	}
	out.GameDirectory = absoluteDir
	var r Receipt
	var saved Audit
	if e := ReadJSON(receiptPath, &r); e != nil {
		return out, e
	}
	if e := ReadJSON(auditPath, &saved); e != nil {
		return out, e
	}
	before, e := Bind(receiptPath)
	if e != nil {
		return out, e
	}
	auditArtifact, e := Bind(auditPath)
	if e != nil {
		return out, e
	}
	fresh, games, e := AuditDirectory(receiptPath, gameDir)
	if e != nil {
		return out, e
	}
	if !fresh.Admitted || fresh.Exclusions != 0 || !reflect.DeepEqual(saved, fresh) {
		return out, fmt.Errorf("saved audit differs from independent replay")
	}
	if fresh.ReceiptSHA256 != before.SHA256 {
		return out, fmt.Errorf("receipt changed during analysis")
	}
	out.Mode = r.Plan.Mode
	out.ReceiptSHA256 = before.SHA256
	out.Audit = auditArtifact
	out.Games = fresh.Games
	out.Exclusions = fresh.Exclusions
	clusters, e := openingClusters(r.Plan.Anchors, games, r.Plan.Count)
	if e != nil {
		return out, e
	}
	all := make([]int, r.Plan.Count)
	for i := range all {
		all[i] = i
	}
	counts := resampledCounts(clusters, all)
	for i, anchor := range r.Plan.Anchors {
		if counts[i] != fresh.Counts[anchor.Name] {
			return out, fmt.Errorf("independent analysis W/D/L mismatch")
		}
		a, e := Primary(anchor, counts[i])
		if e != nil {
			return out, e
		}
		out.Anchors = append(out.Anchors, a)
	}
	out.Primary, e = Pool(out.Anchors)
	if e != nil {
		return out, e
	}
	out.Bracketed = bracketed(out.Anchors)
	out.ScreenEligible, out.ClaimEligible = eligible(out.Mode, out.Anchors, out.Primary, fresh.Admitted, fresh.Exclusions)
	out.Bootstrap, e = Bootstrap(r.Plan.Anchors, games, r.Plan.Count, 10000, 3000)
	if e != nil {
		return out, e
	}
	out.Interpretation = []string{"Local historical-label calibration, conditional on fixed anchors; no official CCRL rating.", "Both intervals omit label uncertainty, OS/time-control transfer, shared openings, correlated Counter versions and non-transitivity.", "The supplemental bootstrap never replaces the primary decision rule."}
	out.Questions = []string{"Bootstrap interval construction is not specified; supplemental percentile interval uses linear interpolation (type 7)."}
	if out.Mode == "screen" {
		out.Questions = append(out.Questions, "Screen bootstrap index count is not specified; jointly resample the screen's frozen 40 indices, never the confirmation's 80.")
		out.Interpretation = append(out.Interpretation, "A passing screen admits only the frozen candidate for separately authorized confirmation; it supplies no >3000 claim.")
	}
	if out.Mode == "tooling-check" {
		out.Interpretation = append(out.Interpretation, "Tooling check only; no screen or strength claim is eligible.")
	}
	// Evidence must remain the same through bootstrap computation as well.
	if e = Check(before); e != nil {
		return out, e
	}
	if e = Check(auditArtifact); e != nil {
		return out, e
	}
	if e = Recheck(r); e != nil {
		return out, e
	}
	end, _, e := AuditDirectory(receiptPath, gameDir)
	if e != nil {
		return out, e
	}
	if !reflect.DeepEqual(end, fresh) {
		return out, fmt.Errorf("games changed during analysis")
	}
	return out, nil
}
