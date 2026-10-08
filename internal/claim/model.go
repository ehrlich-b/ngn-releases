// Package claim is NGN's independently written, fixed-sample claim instrument.
// It uses only the independently produced NGN board/move layer and Go stdlib.
package claim

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const StartFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
const OpeningSHA = "974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222"

// Verified by the zero-game identity receipt: UCI Maelstrom v3.3.0.
const MaelstromSHA = "66ea969530d63f5904114079f518ba618b0deb4e269eb0c95d7496960379ea53"
const ViridithasSHA = "35b32c4acc3216fd9d1084bebad90452cee2e8725af4bdd0f175cc42c4e3def3"
const Schema = "NGN-CLAIM-2"

type Artifact struct {
	Path   string
	SHA256 string
}
type Identity struct {
	Engine                              Artifact
	Net                                 Artifact
	SourceRoot, SourceCommit, GoVersion string
	BuildArgv                           []string
	// Manifest, data, training, export and parity are separately bound artifacts.
	Provenance map[string]Artifact
}
type Anchor struct {
	Name         string
	Label        int
	Engine       Artifact
	Dependencies []Artifact
}

func Anchors() []Anchor {
	return []Anchor{
		{Name: "Counter 3.8", Label: 2994, Engine: Artifact{"/mnt/c/Users/ehrli/ngn/repin/anchors/counter_38.exe", "d260174182ea10c5b902a8d25e217166f5b5b54dc4fa64c220b498acce6dec11"}},
		{Name: "Maelstrom 3.3.0", Label: 3317, Engine: Artifact{Path: "/home/ehrli/nnue-owned-maelstrom-20260930-v2/maelstrom-3.3.0", SHA256: MaelstromSHA}},
		{Name: "Counter 5.5", Label: 3359, Engine: Artifact{"/home/ehrli/repos/ngn-next/output/go-opponent-preflight-20260906/counter-5.5-linux-amd64", "6c48fb52934d49d3774633e32f0b4fb4796b1e2c63925f24167ef0f0c0d761c8"}},
		{Name: "Viridithas 10.0.0", Label: 3557, Engine: Artifact{Path: "/home/ehrli/ngn-data/claim-run-20261007/anchors/viridithas10/viridithas-10.0.0-x86_64-linux-v3", SHA256: ViridithasSHA}},
	}
}

type Opening struct {
	Index   int
	FEN     string
	History []string
	Line    string
}
type Plan struct {
	Schema, Mode            string
	Candidate               Identity
	Anchors                 []Anchor
	Openings                Artifact
	Count, Concurrency, Cap int
	BaseNS, IncrementNS     int64
	Runner                  Artifact
	WindowsBridge           Artifact // NGN-written Windows affinity/priority bridge
	MoveOverhead            int
	MaxLoad                 float64
}
type Event struct {
	AtNS         int64
	Stream, Line string
}
type SessionRecord struct {
	Engine     Artifact
	Events     []Event
	Options    map[string]string
	Selected   map[string]string
	PID        int
	ExitCode   int
	LaunchArgv []string
	Scheduling string
}
type Ply struct {
	Move, STM, Role    string
	Before, After      [2]int64 // white, black, nanoseconds
	UsedNS             int64
	GoEvent, BestEvent int
}
type Game struct {
	Schema, ID, PairID, Anchor, CandidateColor string
	AttemptID, StartedUTC, EndedUTC            string
	Opening                                    Opening
	BaseNS, IncrementNS                        int64
	Cap                                        int
	Candidate, Opponent                        SessionRecord
	Plies                                      []Ply
	Result, Reason, Error                      string // result is white POV
}
type Counts struct{ W, D, L, N, Pairs, CapDraws int }
type Summary struct {
	Schema string
	Counts map[string]Counts
	Games  int
}
type Audit struct {
	Schema, ReceiptSHA256 string
	Games                 map[string]string
	Counts                map[string]Counts
	Admitted              bool
	Exclusions            int
}

func Digest(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
func Bind(path string) (Artifact, error) {
	p, e := filepath.Abs(path)
	if e != nil {
		return Artifact{}, e
	}
	h, e := Digest(p)
	return Artifact{p, h}, e
}
func Check(a Artifact) error {
	if a.Path == "" || len(a.SHA256) != 64 {
		return fmt.Errorf("unresolved artifact: %v", a)
	}
	h, e := Digest(a.Path)
	if e != nil {
		return e
	}
	if h != a.SHA256 {
		return fmt.Errorf("hash mismatch: %s: %s != %s", a.Path, h, a.SHA256)
	}
	return nil
}
func ReadJSON(path string, v any) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if e = d.Decode(v); e != nil {
		return e
	}
	var extra any
	if e = d.Decode(&extra); e != io.EOF {
		return fmt.Errorf("extra JSON in %s", path)
	}
	return nil
}

// Exclusive creation prevents overwriting/restarting a frozen attempt.
func WriteJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	b = append(b, '\n')
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0444)
	if e != nil {
		return e
	}
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	return f.Close()
}
