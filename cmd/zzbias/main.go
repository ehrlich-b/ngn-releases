// zzbias (throwaway diagnostic): reads a TSV of real positions with NGN's STATIC
// white-POV eval (from TestZZEvalBiasExtract), queries Stockfish for each FEN's
// white-POV eval at a fixed depth, and reports the SIGNED bias (NGN - SF) overall,
// by phase bucket, and split by which side NGN was — the direct test of whether
// NGN's static eval systematically over-favors a side. Deterministic (SF fixed
// depth); E-core (lowPower). Subsamples with -every for speed.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/internal/uci"
)

func sfWPOV(sf *uci.Engine, fen string, goCmd string) (int, bool) {
	uci.Send(sf, "position fen "+fen)
	lines := uci.Analyze(sf, goCmd, 30*time.Second)
	cp, have := 0, false
	for _, ln := range lines {
		if !strings.Contains(ln, " score ") {
			continue
		}
		f := strings.Fields(ln)
		for i := 0; i+1 < len(f); i++ {
			switch f[i] {
			case "cp":
				cp, _ = strconv.Atoi(f[i+1])
				have = true
			case "mate":
				m, _ := strconv.Atoi(f[i+1])
				s := 1
				if m < 0 {
					s, m = -1, -m
				}
				cp, have = s*(100000-m), true
			}
		}
	}
	if !have {
		return 0, false
	}
	// SF reports side-to-move POV; convert to white POV.
	if strings.Contains(fen, " b ") {
		cp = -cp
	}
	if cp > 1500 {
		cp = 1500
	}
	if cp < -1500 {
		cp = -1500
	}
	return cp, true
}

func stats(xs []float64) (mean, med, sd float64) {
	if len(xs) == 0 {
		return
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	mean = s / float64(len(xs))
	var v float64
	for _, x := range xs {
		v += (x - mean) * (x - mean)
	}
	sd = math.Sqrt(v / float64(len(xs)))
	c := append([]float64(nil), xs...)
	sort.Float64s(c)
	med = c[len(c)/2]
	return
}

func main() {
	in := flag.String("in", "", "tsv from TestZZEvalBiasExtract")
	depth := flag.Int("depth", 12, "SF depth")
	every := flag.Int("every", 1, "use every Nth row (subsample)")
	flag.Parse()
	goCmd := fmt.Sprintf("go depth %d", *depth)

	f, err := os.Open(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	sf := uci.Start("/opt/homebrew/bin/stockfish", "stockfish", true)
	if sf == nil {
		fmt.Fprintln(os.Stderr, "no sf")
		os.Exit(1)
	}
	defer uci.Stop(sf)
	uci.Send(sf, "setoption name MultiPV value 1")
	uci.Send(sf, "isready")
	uci.WaitFor(sf, "readyok", 5*time.Second)

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	sc.Scan() // header

	// signed bias = NGN(wpov) - SF(wpov). "own-side bias" = bias from NGN's POV:
	// if NGN is white, ownBias = +bias; if black, ownBias = -bias.
	var allBias, ownBias, absErr []float64
	type bucket struct{ bias, own []float64 }
	byPhase := map[string]*bucket{"mg(>=12)": {}, "lateMg(6-11)": {}, "eg(<6)": {}}
	n := 0
	row := 0
	for sc.Scan() {
		row++
		if (row-1)%*every != 0 {
			continue
		}
		fld := strings.Split(strings.TrimSpace(sc.Text()), "\t")
		if len(fld) < 6 {
			continue
		}
		color := fld[2]
		phase, _ := strconv.Atoi(fld[3])
		ngn, _ := strconv.Atoi(fld[4])
		fen := fld[5]
		sfv, ok := sfWPOV(sf, fen, goCmd)
		if !ok {
			continue
		}
		b := float64(ngn - sfv)
		ob := b
		if color == "b" {
			ob = -b
		}
		allBias = append(allBias, b)
		ownBias = append(ownBias, ob)
		absErr = append(absErr, math.Abs(b))
		var key string
		switch {
		case phase >= 12:
			key = "mg(>=12)"
		case phase >= 6:
			key = "lateMg(6-11)"
		default:
			key = "eg(<6)"
		}
		byPhase[key].bias = append(byPhase[key].bias, b)
		byPhase[key].own = append(byPhase[key].own, ob)
		n++
		if n%50 == 0 {
			fmt.Fprintf(os.Stderr, "  %d positions...\n", n)
		}
	}

	m, md, sd := stats(allBias)
	om, omd, osd := stats(ownBias)
	am, amd, _ := stats(absErr)
	fmt.Printf("\n=== STATIC EVAL BIAS  (NGN_static - SF_d%d, white-POV)  n=%d ===\n", *depth, n)
	fmt.Printf("  signed bias (wPOV):    mean %+7.1f  median %+7.1f  sd %6.1f\n", m, md, sd)
	fmt.Printf("  OWN-SIDE bias (NGNpov):mean %+7.1f  median %+7.1f  sd %6.1f   <-- + = NGN over-favors ITSELF\n", om, omd, osd)
	fmt.Printf("  abs error |NGN-SF|:    mean %7.1f  median %7.1f\n", am, amd)
	fmt.Println("  ---- by phase ----")
	for _, k := range []string{"mg(>=12)", "lateMg(6-11)", "eg(<6)"} {
		bb := byPhase[k]
		if len(bb.own) == 0 {
			continue
		}
		_, omdk, _ := stats(bb.own)
		omk, _, _ := stats(bb.own)
		bm, bmd, _ := stats(bb.bias)
		fmt.Printf("  %-13s n=%-4d  own-bias mean %+7.1f median %+7.1f   wPOV-bias mean %+7.1f median %+7.1f\n",
			k, len(bb.own), omk, omdk, bm, bmd)
	}
}
