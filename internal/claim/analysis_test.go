package claim

import (
	"math"
	"reflect"
	"testing"
)

func closeValue(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9*math.Max(1, math.Abs(want)) {
		t.Fatalf("%s: got %.15g, want %.15g", label, got, want)
	}
}
func TestPrimaryHandComputed(t *testing.T) {
	// Four games, score 2.5: raw=5/8, corrected p=3/5,
	// odds=3/2, N*p*(1-p)=24/25. No pair-based variance.
	a, e := Primary(Anchor{Name: "hand", Label: 3000}, Counts{W: 2, D: 1, L: 1, N: 4, Pairs: 2})
	if e != nil {
		t.Fatal(e)
	}
	closeValue(t, "raw", a.RawScore, 0.625)
	closeValue(t, "p", a.P, 0.6)
	closeValue(t, "performance", a.Performance, 3070.4365036222725)
	const ln10 = 2.302585092994046
	wantVariance := 160000 * 25 / (ln10 * ln10 * 24)
	closeValue(t, "variance", a.Variance, wantVariance)
	closeValue(t, "weight", a.Weight, 1/wantVariance)
	// All losses stay finite through the protocol's half-point smoothing.
	loss, e := Primary(Anchor{Name: "loss", Label: 2994}, Counts{L: 160, N: 160, Pairs: 80})
	if e != nil {
		t.Fatal(e)
	}
	closeValue(t, "loss p", loss.P, 1.0/322)
	if math.IsInf(loss.Variance, 0) || math.IsNaN(loss.Performance) {
		t.Fatal("saturated score is nonfinite")
	}
}
func TestPoolHandComputed(t *testing.T) {
	// Weights 1/4 and 1 yield (1/4*3000+1*3050)/(5/4)=3040,
	// SE=2/sqrt(5), and CI=3040 +/- 1.96*2/sqrt(5).
	p, e := Pool([]AnchorAnalysis{{Performance: 3000, Weight: 0.25}, {Performance: 3050, Weight: 1}})
	if e != nil {
		t.Fatal(e)
	}
	closeValue(t, "pool", p.Estimate, 3040)
	closeValue(t, "SE", p.SE, 0.8944271909999159)
	closeValue(t, "lower", p.CI95[0], 3038.2469227056403)
	closeValue(t, "upper", p.CI95[1], 3041.7530772943597)
}
func TestAnalysisRejectsInvalidCounts(t *testing.T) {
	for _, c := range []Counts{{}, {W: 1, N: 2}, {W: -1, D: 2, N: 1}} {
		if _, e := Primary(Anchor{}, c); e == nil {
			t.Fatalf("accepted %v", c)
		}
	}
	if _, e := Pool(nil); e == nil {
		t.Fatal("accepted empty pool")
	}
}
func decisionAnchors(pairs int) []AnchorAnalysis {
	fixed := Anchors()
	out := make([]AnchorAnalysis, len(fixed))
	for i, a := range fixed {
		out[i] = AnchorAnalysis{Name: a.Name, Label: a.Label, Counts: Counts{N: pairs * 2, Pairs: pairs}, RawScore: 0.25}
	}
	out[0].RawScore = 0.75
	return out
}
func TestClaimAndScreenDecisionBoundaries(t *testing.T) {
	p := PoolAnalysis{CI95: [2]float64{3000, 3100}}
	a := decisionAnchors(80)
	if screen, claim := eligible("confirmation", a, p, true, 0); screen || claim {
		t.Fatal("equality at 3000 admitted")
	}
	p.CI95[0] = math.Nextafter(3000, math.Inf(1))
	if screen, claim := eligible("confirmation", a, p, true, 0); screen || !claim {
		t.Fatal("strict lower bound rejected")
	}
	for _, raw := range []float64{0.5, 0.75} {
		b := decisionAnchors(80)
		for i := range b {
			b[i].RawScore = raw
		}
		if bracketed(b) {
			t.Fatal("unbracketed equality/one-sided pool admitted")
		}
	}
	for _, tc := range []struct {
		mode       string
		a          []AnchorAnalysis
		admitted   bool
		exclusions int
	}{
		{"confirmation", decisionAnchors(40), true, 0},
		{"confirmation", decisionAnchors(80), false, 0},
		{"confirmation", decisionAnchors(80), true, 1},
		{"tooling-check", decisionAnchors(80), true, 0},
	} {
		if s, c := eligible(tc.mode, tc.a, p, tc.admitted, tc.exclusions); s || c {
			t.Fatalf("invalid evidence admitted: %v", tc)
		}
	}
	if s, c := eligible("screen", decisionAnchors(40), p, true, 0); !s || c {
		t.Fatal("screen eligibility became a claim")
	}
	a[0].Counts.Pairs = 79
	if _, c := eligible("confirmation", a, p, true, 0); c {
		t.Fatal("broken pair admitted")
	}
	a = decisionAnchors(80)
	a[1].Label++
	if _, c := eligible("confirmation", a, p, true, 0); c {
		t.Fatal("changed anchor label admitted")
	}
	a = decisionAnchors(80)
	if _, c := eligible("confirmation", a[:2], p, true, 0); c {
		t.Fatal("missing anchor admitted")
	}
}
func scoreGame(anchor string, index int, color, result string) Game {
	return Game{Anchor: anchor, Opening: Opening{Index: index}, CandidateColor: color, Result: result}
}
func TestBootstrapRetainsJointAnchorsAndColors(t *testing.T) {
	anchors := []Anchor{{Name: "A", Label: 3000}, {Name: "B", Label: 3200}}
	// Index 0: A wins both; B draws both. Index 1: A loses both;
	// B splits colors. Selecting [1,1,0] must duplicate whole vectors.
	games := []Game{
		scoreGame("A", 0, "white", "1-0"), scoreGame("A", 0, "black", "0-1"),
		scoreGame("B", 0, "white", "1/2-1/2"), scoreGame("B", 0, "black", "1/2-1/2"),
		scoreGame("A", 1, "white", "0-1"), scoreGame("A", 1, "black", "1-0"),
		scoreGame("B", 1, "white", "1-0"), scoreGame("B", 1, "black", "1-0"),
	}
	clusters, e := openingClusters(anchors, games, 2)
	if e != nil {
		t.Fatal(e)
	}
	got := resampledCounts(clusters, []int{1, 1, 0})
	want := []Counts{{W: 2, L: 4, N: 6, Pairs: 3}, {W: 2, D: 2, L: 2, N: 6, Pairs: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cluster split: got %v, want %v", got, want)
	}
	first, e := Bootstrap(anchors, games, 2, 10000, 3000)
	if e != nil {
		t.Fatal(e)
	}
	second, e := Bootstrap(anchors, games, 2, 10000, 3000)
	if e != nil {
		t.Fatal(e)
	}
	if first != second || first.Draws != 10000 || first.Seed != 3000 || first.Indices != 2 {
		t.Fatal("nondeterministic/wrong declared bootstrap")
	}
	if first.CI95[0] >= first.CI95[1] {
		t.Fatal("variable clusters gave no uncertainty")
	}
	if _, e = Bootstrap(anchors, games[:7], 2, 10000, 3000); e == nil {
		t.Fatal("missing color accepted")
	}
	if _, e = Bootstrap(anchors, append(games, games[0]), 2, 10000, 3000); e == nil {
		t.Fatal("duplicate color accepted")
	}
}
func TestBootstrapConstantPairsHandComputed(t *testing.T) {
	anchors := Anchors()
	var games []Game
	for _, a := range anchors {
		for i := 0; i < 80; i++ {
			games = append(games, scoreGame(a.Name, i, "white", "1-0"), scoreGame(a.Name, i, "black", "1-0"))
		}
	}
	// Every selected pair is one win and one loss, so p=1/2 and all
	// variances match. Every resample pool=(2994+3317+3359+3557)/4.
	b, e := Bootstrap(anchors, games, 80, 10000, 3000)
	if e != nil {
		t.Fatal(e)
	}
	closeValue(t, "bootstrap lower", b.CI95[0], 13227.0/4)
	closeValue(t, "bootstrap upper", b.CI95[1], 13227.0/4)
}
func TestBootstrapQuantileHandComputed(t *testing.T) {
	closeValue(t, "interpolated percentile", percentile([]float64{0, 10, 20, 30, 40}, 0.025), 1)
}
