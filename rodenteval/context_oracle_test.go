//go:build rodentoracle

package rodenteval

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

const (
	transitionFixtureSchema  = "rodent-v1.1-anand-transition-fixtures-v1"
	transitionFixtureSHA256  = "f9e26de0ed7a9663011f1fe8844ee0d8beb7a1bea4a2e4da50805fdbbf9579d4"
	taggedOracleSchema       = "rodent-v1.1-anand-tagged-transition-oracle-v1"
	releaseOracleSchema      = "rodent-v1.1-anand-slice-b-release-raw-oracle-v1"
	taggedSourceTree         = "be0effaf52855fcbdf4a288fc1c203e1febcb5ce"
	taggedSourceArchiveSHA   = "929f560996c3b0a8609e594923e587ab0eef61c9c013afbbd4497045113523fd"
	taggedUCISHA             = "f452492b01ac32fc0cf987383bf85b4d934bdefcc6da7f1800b3686121ce082e"
	taggedOptionsSHA         = "d71f036cc77bb2788fe6351ebdf6d48911363a8ad7d65d20047c4f60999bc13c"
	taggedNNUESHA            = "bb6689c3465996fe982bcc77272c1033744fa53030958a4afb737523d55995bc"
	taggedEvalSHA            = "2a8b8b955ad2f2d692f721eb5b39b03ed27b576970d80de1d393fee8fb7a9891"
	transitionDigestEncoding = "perspective 0 lanes 0..511 as little-endian int16, followed by perspective 1 lanes 0..511; per-perspective digests hash each 1024-byte half"
)

type transitionIdentities struct {
	TaggedSourceCommit     string `json:"tagged_source_commit"`
	TaggedSourceTree       string `json:"tagged_source_tree"`
	TaggedSourceArchiveSHA string `json:"tagged_source_archive_sha256"`
	TaggedUCISHA           string `json:"tagged_uci_go_sha256"`
	TaggedOptionsSHA       string `json:"tagged_options_go_sha256"`
	TaggedNNUESHA          string `json:"tagged_nnue_go_sha256"`
	TaggedEvalSHA          string `json:"tagged_eval_go_sha256"`
	ReleaseBinarySHA       string `json:"release_binary_sha256"`
	NetworkSHA             string `json:"network_sha256"`
	PersonalitySHA         string `json:"personality_sha256"`
}

type transitionConfiguration struct {
	HCEWeight           int `json:"hce_weight"`
	NNUEWeight          int `json:"nnue_weight"`
	NNUEScale           int `json:"nnue_scale"`
	HorizontalMirroring int `json:"horizontal_mirroring"`
}

type transitionDelta struct {
	MovingPlane    int  `json:"moving_plane"`
	From           int  `json:"from"`
	To             int  `json:"to"`
	HasCapture     bool `json:"has_capture"`
	CapturedPlane  int  `json:"captured_plane"`
	CaptureSquare  int  `json:"capture_square"`
	HasPromotion   bool `json:"has_promotion"`
	PromotionPlane int  `json:"promotion_plane"`
	HasCastleRook  bool `json:"has_castle_rook"`
	CastleRookFrom int  `json:"castle_rook_from"`
	CastleRookTo   int  `json:"castle_rook_to"`
}

type transitionStep struct {
	Name        string           `json:"name"`
	Operation   string           `json:"op"`
	TaggedKind  string           `json:"tagged_kind"`
	Delta       *transitionDelta `json:"delta"`
	ExpectedFEN string           `json:"expected_fen"`
	Repeat      int              `json:"repeat"`
}

type transitionSequence struct {
	Name    string           `json:"name"`
	RootFEN string           `json:"root_fen"`
	Steps   []transitionStep `json:"steps"`
}

type transitionFixture struct {
	Schema              string                  `json:"schema"`
	Classification      string                  `json:"classification"`
	Identities          transitionIdentities    `json:"identities"`
	TaggedConfiguration transitionConfiguration `json:"tagged_configuration"`
	PlaneOrder          []string                `json:"plane_order"`
	DigestEncoding      string                  `json:"digest_encoding"`
	Sequences           []transitionSequence    `json:"sequences"`
}

type taggedTransitionRecord struct {
	Checkpoint         string `json:"checkpoint"`
	FEN                string `json:"fen"`
	Operation          string `json:"operation"`
	Depth              int    `json:"depth"`
	Raw                int    `json:"raw"`
	AccumulatorSHA     string `json:"accumulator_sha256"`
	WhiteSHA           string `json:"white_accumulator_sha256"`
	BlackSHA           string `json:"black_accumulator_sha256"`
	MatchedFullRefresh bool   `json:"matched_full_refresh"`
}

type taggedTransitionOracle struct {
	Schema              string                   `json:"schema"`
	Classification      string                   `json:"classification"`
	FixtureSHA          string                   `json:"fixture_sha256"`
	NetworkSHA          string                   `json:"network_sha256"`
	TaggedSourceCommit  string                   `json:"tagged_source_commit"`
	TaggedConfiguration transitionConfiguration  `json:"tagged_configuration"`
	DigestEncoding      string                   `json:"digest_encoding"`
	Records             []taggedTransitionRecord `json:"records"`
}

type releaseDeterminism struct {
	Passes         int  `json:"passes"`
	Matched        bool `json:"matched"`
	RecordsPerPass int  `json:"records_per_pass"`
}

type releaseTransitionRecord struct {
	Name    string   `json:"name"`
	FEN     string   `json:"fen"`
	Origins []string `json:"origins"`
	Raw     int      `json:"raw"`
}

type releaseTransitionOracle struct {
	Schema             string                    `json:"schema"`
	Classification     string                    `json:"classification"`
	ReleaseBinarySHA   string                    `json:"release_binary_sha256"`
	NetworkSHA         string                    `json:"network_sha256"`
	PersonalitySHA     string                    `json:"personality_sha256"`
	FixtureSHA         string                    `json:"fixture_sha256"`
	RawOracleCommand   string                    `json:"raw_oracle_command"`
	RawOracleSemantics string                    `json:"raw_oracle_semantics"`
	Determinism        releaseDeterminism        `json:"determinism"`
	LaneBoundary       string                    `json:"lane_boundary"`
	Records            []releaseTransitionRecord `json:"records"`
}

type expectedTransitionCheckpoint struct {
	FEN       string
	Operation string
	Depth     int
}

func TestV11AnandIncrementalTaggedAndReleaseOracleParity(t *testing.T) {
	modelPath := os.Getenv("RODENT_V11_ANAND_MODEL")
	fixturePath := os.Getenv("RODENT_V11_ANAND_TRANSITION_FIXTURES")
	taggedPath := os.Getenv("RODENT_V11_ANAND_TAGGED_TRANSITION_ORACLE")
	releasePath := os.Getenv("RODENT_V11_ANAND_RELEASE_TRANSITION_ORACLE")
	if modelPath == "" || fixturePath == "" || taggedPath == "" || releasePath == "" {
		t.Fatal("RODENT_V11_ANAND_MODEL, RODENT_V11_ANAND_TRANSITION_FIXTURES, RODENT_V11_ANAND_TAGGED_TRANSITION_ORACLE, and RODENT_V11_ANAND_RELEASE_TRANSITION_ORACLE are required")
	}

	model, err := LoadV11Anand(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	fixtureData := readTransitionFile(t, fixturePath)
	if got := transitionSHA256(fixtureData); got != transitionFixtureSHA256 {
		t.Fatalf("transition fixture SHA-256 = %s, want %s", got, transitionFixtureSHA256)
	}
	var fixture transitionFixture
	decodeTransitionJSON(t, fixtureData, &fixture)
	expected := validateTransitionFixture(t, fixture)

	var tagged taggedTransitionOracle
	decodeTransitionJSON(t, readTransitionFile(t, taggedPath), &tagged)
	taggedByCheckpoint := validateTaggedTransitionOracle(t, tagged, expected)

	var release releaseTransitionOracle
	decodeTransitionJSON(t, readTransitionFile(t, releasePath), &release)
	releaseByCheckpoint := validateReleaseTransitionOracle(t, release, expected)

	for _, sequence := range fixture.Sequences {
		t.Run(sequence.Name, func(t *testing.T) {
			root := parseTransitionPosition(t, sequence.RootFEN)
			context, err := model.NewSearchContext(root)
			if err != nil {
				t.Fatal(err)
			}
			assertTransitionCheckpoint(t, model, context, sequence.Name+"/root", taggedByCheckpoint, releaseByCheckpoint)
			for _, step := range sequence.Steps {
				checkpoint := sequence.Name + "/" + step.Name
				expectedPost := parseTransitionPosition(t, step.ExpectedFEN)
				switch step.Operation {
				case "move":
					delta := candidateTransitionDelta(t, step.Delta)
					if err := context.PushMove(delta, expectedPost); err != nil {
						t.Fatalf("%s: %v", checkpoint, err)
					}
				case "null":
					if err := context.PushNull(); err != nil {
						t.Fatalf("%s: %v", checkpoint, err)
					}
				case "pop":
					if err := context.Pop(); err != nil {
						t.Fatalf("%s: %v", checkpoint, err)
					}
				case "null_repeat":
					for repeat := 0; repeat < step.Repeat; repeat++ {
						parentDepth := context.depth
						parentPosition := context.positions[parentDepth]
						parentAccumulator := context.accumulators[parentDepth]
						if err := context.PushNull(); err != nil {
							t.Fatalf("%s repetition %d: %v", checkpoint, repeat+1, err)
						}
						if context.depth != parentDepth+1 {
							t.Fatalf("%s repetition %d: depth = %d, want %d", checkpoint, repeat+1, context.depth, parentDepth+1)
						}
						wantPosition := parentPosition
						wantPosition.SideToMove ^= 1
						if context.positions[context.depth] != wantPosition {
							t.Fatalf("%s repetition %d: null position = %+v, want %+v", checkpoint, repeat+1, context.positions[context.depth], wantPosition)
						}
						if context.accumulators[context.depth] != parentAccumulator {
							t.Fatalf("%s repetition %d: null accumulator differs from parent", checkpoint, repeat+1)
						}
					}
				default:
					t.Fatalf("%s: unknown operation %q", checkpoint, step.Operation)
				}
				if context.Position() != expectedPost {
					t.Fatalf("%s: context position = %+v, want %+v", checkpoint, context.Position(), expectedPost)
				}
				if context.Depth() != expected[checkpoint].Depth {
					t.Fatalf("%s: context depth = %d, want %d", checkpoint, context.Depth(), expected[checkpoint].Depth)
				}
				assertTransitionCheckpoint(t, model, context, checkpoint, taggedByCheckpoint, releaseByCheckpoint)
			}
		})
	}
}

func validateTransitionFixture(t *testing.T, fixture transitionFixture) map[string]expectedTransitionCheckpoint {
	t.Helper()
	wantIdentities := transitionIdentities{
		TaggedSourceCommit: taggedSourceCommit, TaggedSourceTree: taggedSourceTree,
		TaggedSourceArchiveSHA: taggedSourceArchiveSHA, TaggedUCISHA: taggedUCISHA,
		TaggedOptionsSHA: taggedOptionsSHA, TaggedNNUESHA: taggedNNUESHA,
		TaggedEvalSHA: taggedEvalSHA, ReleaseBinarySHA: releaseBinarySHA256,
		NetworkSHA: V11AnandSHA256, PersonalitySHA: anandPersonalitySHA,
	}
	wantConfiguration := transitionConfiguration{HCEWeight: 0, NNUEWeight: 100, NNUEScale: outputScale, HorizontalMirroring: 1}
	if fixture.Schema != transitionFixtureSchema || fixture.Classification != "source-selected transition semantics; no score selection" ||
		fixture.Identities != wantIdentities || fixture.TaggedConfiguration != wantConfiguration ||
		strings.Join(fixture.PlaneOrder, ",") != "WP,WN,WB,WR,WQ,WK,BP,BN,BB,BR,BQ,BK" ||
		fixture.DigestEncoding != transitionDigestEncoding {
		t.Fatal("transition fixture provenance or encoding differs")
	}
	if len(fixture.Sequences) != 22 {
		t.Fatalf("transition sequence count = %d, want 22", len(fixture.Sequences))
	}
	expected := make(map[string]expectedTransitionCheckpoint, 75)
	stepCount := 0
	operations := make(map[string]bool)
	taggedKinds := make(map[string]bool)
	for _, sequence := range fixture.Sequences {
		if sequence.Name == "" || strings.Contains(sequence.Name, "/") || sequence.RootFEN == "" || len(sequence.Steps) == 0 {
			t.Fatalf("invalid transition sequence %q", sequence.Name)
		}
		rootKey := sequence.Name + "/root"
		if _, exists := expected[rootKey]; exists {
			t.Fatalf("duplicate transition sequence %q", sequence.Name)
		}
		parseTransitionPosition(t, sequence.RootFEN)
		expected[rootKey] = expectedTransitionCheckpoint{FEN: sequence.RootFEN, Operation: "root", Depth: 0}
		depth := 0
		for _, step := range sequence.Steps {
			stepCount++
			checkpoint := sequence.Name + "/" + step.Name
			if step.Name == "" || strings.Contains(step.Name, "/") || step.ExpectedFEN == "" {
				t.Fatalf("invalid transition checkpoint %q", checkpoint)
			}
			if _, exists := expected[checkpoint]; exists {
				t.Fatalf("duplicate transition checkpoint %q", checkpoint)
			}
			parseTransitionPosition(t, step.ExpectedFEN)
			operations[step.Operation] = true
			switch step.Operation {
			case "move":
				if step.Delta == nil || step.Repeat != 0 || step.TaggedKind == "" {
					t.Fatalf("%s: malformed move fixture", checkpoint)
				}
				candidateTransitionDelta(t, step.Delta)
				taggedKinds[step.TaggedKind] = true
				depth++
			case "null":
				if step.Delta != nil || step.Repeat != 0 || step.TaggedKind != "" {
					t.Fatalf("%s: malformed null fixture", checkpoint)
				}
				depth++
			case "pop":
				if step.Delta != nil || step.Repeat != 0 || step.TaggedKind != "" || depth == 0 {
					t.Fatalf("%s: malformed pop fixture", checkpoint)
				}
				depth--
			case "null_repeat":
				if step.Delta != nil || step.Repeat != 128 || step.TaggedKind != "" || depth != 0 {
					t.Fatalf("%s: malformed growth fixture", checkpoint)
				}
				depth += step.Repeat
			default:
				t.Fatalf("%s: unknown operation %q", checkpoint, step.Operation)
			}
			expected[checkpoint] = expectedTransitionCheckpoint{FEN: step.ExpectedFEN, Operation: step.Operation, Depth: depth}
		}
	}
	if stepCount != 53 || len(expected) != 75 ||
		!sameTransitionSet(operations, []string{"move", "null", "pop", "null_repeat"}) ||
		!sameTransitionSet(taggedKinds, []string{"normal", "ep_set", "capture", "ep_capture", "castle", "promotion", "promotion_capture"}) {
		t.Fatalf("transition coverage shrank: steps=%d records=%d operations=%v kinds=%v", stepCount, len(expected), operations, taggedKinds)
	}
	return expected
}

func validateTaggedTransitionOracle(
	t *testing.T,
	oracle taggedTransitionOracle,
	expected map[string]expectedTransitionCheckpoint,
) map[string]taggedTransitionRecord {
	t.Helper()
	wantConfiguration := transitionConfiguration{HCEWeight: 0, NNUEWeight: 100, NNUEScale: outputScale, HorizontalMirroring: 1}
	if oracle.Schema != taggedOracleSchema ||
		oracle.Classification != "tagged-source mechanism oracle: incremental accumulator equals tagged full refresh; stack/null/pop/growth are harness machinery; not exact-release lane evidence" ||
		oracle.FixtureSHA != transitionFixtureSHA256 || oracle.NetworkSHA != V11AnandSHA256 ||
		oracle.TaggedSourceCommit != taggedSourceCommit || oracle.TaggedConfiguration != wantConfiguration ||
		oracle.DigestEncoding != transitionDigestEncoding || len(oracle.Records) != len(expected) {
		t.Fatal("tagged transition oracle provenance, encoding, or record count differs")
	}
	result := make(map[string]taggedTransitionRecord, len(expected))
	for _, record := range oracle.Records {
		want, exists := expected[record.Checkpoint]
		if !exists || record.FEN != want.FEN || record.Operation != want.Operation || record.Depth != want.Depth || !record.MatchedFullRefresh {
			t.Fatalf("tagged transition record %q differs from fixture: %+v", record.Checkpoint, record)
		}
		if _, duplicate := result[record.Checkpoint]; duplicate {
			t.Fatalf("duplicate tagged transition checkpoint %q", record.Checkpoint)
		}
		for label, digest := range map[string]string{"whole": record.AccumulatorSHA, "white": record.WhiteSHA, "black": record.BlackSHA} {
			if !validTransitionSHA256(digest) {
				t.Fatalf("tagged transition %s digest at %q is invalid: %q", label, record.Checkpoint, digest)
			}
		}
		result[record.Checkpoint] = record
	}
	return result
}

func validateReleaseTransitionOracle(
	t *testing.T,
	oracle releaseTransitionOracle,
	expected map[string]expectedTransitionCheckpoint,
) map[string]releaseTransitionRecord {
	t.Helper()
	wantByFEN := make(map[string]map[string]bool)
	for checkpoint, record := range expected {
		if wantByFEN[record.FEN] == nil {
			wantByFEN[record.FEN] = make(map[string]bool)
		}
		wantByFEN[record.FEN][checkpoint] = true
	}
	if oracle.Schema != releaseOracleSchema ||
		oracle.Classification != "exact-release full-refresh raw runtime oracle at transition checkpoint FENs; no exact-release lane claim" ||
		oracle.ReleaseBinarySHA != releaseBinarySHA256 || oracle.NetworkSHA != V11AnandSHA256 ||
		oracle.PersonalitySHA != anandPersonalitySHA || oracle.FixtureSHA != transitionFixtureSHA256 ||
		oracle.RawOracleCommand != "position fen <FEN>; nnue; isready; parse ^(-?[0-9]+)readyok$" ||
		oracle.RawOracleSemantics != "side-to-move full-refresh getEval integer before material factor" ||
		oracle.LaneBoundary != "the release exposes no accumulator lanes; tagged-source lane oracle is separate mechanism evidence" ||
		oracle.Determinism != (releaseDeterminism{Passes: 2, Matched: true, RecordsPerPass: len(wantByFEN)}) ||
		len(oracle.Records) != len(wantByFEN) {
		t.Fatal("exact-release transition oracle provenance, semantics, determinism, or record count differs")
	}
	result := make(map[string]releaseTransitionRecord, len(expected))
	seenNames := make(map[string]bool, len(oracle.Records))
	seenFENs := make(map[string]bool, len(oracle.Records))
	for _, record := range oracle.Records {
		wantOrigins, exists := wantByFEN[record.FEN]
		if record.Name == "" || seenNames[record.Name] || seenFENs[record.FEN] || !exists || len(record.Origins) != len(wantOrigins) {
			t.Fatalf("invalid or duplicate exact-release record %+v", record)
		}
		seenNames[record.Name] = true
		seenFENs[record.FEN] = true
		for _, origin := range record.Origins {
			if !wantOrigins[origin] {
				t.Fatalf("exact-release record %q has unexpected origin %q", record.Name, origin)
			}
			if _, duplicate := result[origin]; duplicate {
				t.Fatalf("exact-release origin %q appears more than once", origin)
			}
			result[origin] = record
		}
	}
	if len(result) != len(expected) {
		t.Fatalf("exact-release origins cover %d checkpoints, want %d", len(result), len(expected))
	}
	return result
}

func assertTransitionCheckpoint(
	t *testing.T,
	model *Model,
	context *SearchContext,
	checkpoint string,
	tagged map[string]taggedTransitionRecord,
	release map[string]releaseTransitionRecord,
) {
	t.Helper()
	position := context.Position()
	current := context.accumulators[context.depth]
	refresh := model.fullRefresh(position.Board)
	assertAccumulatorEqual(t, current, refresh)
	whole, white, black := transitionAccumulatorDigests(&current)
	taggedRecord := tagged[checkpoint]
	if whole != taggedRecord.AccumulatorSHA || white != taggedRecord.WhiteSHA || black != taggedRecord.BlackSHA {
		t.Fatalf("%s: candidate lanes differ from tagged-source mechanism oracle: whole=%s/%s white=%s/%s black=%s/%s",
			checkpoint, whole, taggedRecord.AccumulatorSHA, white, taggedRecord.WhiteSHA, black, taggedRecord.BlackSHA)
	}
	raw, err := context.EvaluateRaw()
	if err != nil {
		t.Fatal(err)
	}
	if raw != taggedRecord.Raw {
		t.Fatalf("%s: candidate output %d differs from tagged-source mechanism output %d", checkpoint, raw, taggedRecord.Raw)
	}
	releaseRecord := release[checkpoint]
	if raw != releaseRecord.Raw {
		t.Fatalf("%s: candidate raw %d differs from exact-release authority %d", checkpoint, raw, releaseRecord.Raw)
	}
	static, err := context.EvaluateReleaseStatic()
	if err != nil {
		t.Fatal(err)
	}
	if want := independentlyScaleReleaseStatic(releaseRecord.Raw, position.Board); static != want {
		t.Fatalf("%s: candidate release static %d, independently derived from release raw %d", checkpoint, static, want)
	}
}

func candidateTransitionDelta(t *testing.T, input *transitionDelta) MoveDelta {
	t.Helper()
	if input == nil {
		t.Fatal("move fixture has no delta")
	}
	values := map[string]int{
		"moving_plane": input.MovingPlane, "from": input.From, "to": input.To,
		"captured_plane": input.CapturedPlane, "capture_square": input.CaptureSquare,
		"promotion_plane": input.PromotionPlane, "castle_rook_from": input.CastleRookFrom,
		"castle_rook_to": input.CastleRookTo,
	}
	for name, value := range values {
		if value < 0 || value > 255 {
			t.Fatalf("transition delta %s = %d is outside uint8", name, value)
		}
	}
	return MoveDelta{
		MovingPlane: uint8(input.MovingPlane), From: uint8(input.From), To: uint8(input.To),
		HasCapture: input.HasCapture, CapturedPlane: uint8(input.CapturedPlane), CaptureSquare: uint8(input.CaptureSquare),
		HasPromotion: input.HasPromotion, PromotionPlane: uint8(input.PromotionPlane),
		HasCastleRook: input.HasCastleRook, CastleRookFrom: uint8(input.CastleRookFrom), CastleRookTo: uint8(input.CastleRookTo),
	}
}

func transitionAccumulatorDigests(accumulator *accumulator) (string, string, string) {
	data := make([]byte, 2*HiddenSize*2)
	offset := 0
	for perspective := White; perspective <= Black; perspective++ {
		for lane := 0; lane < HiddenSize; lane++ {
			binary.LittleEndian.PutUint16(data[offset:offset+2], uint16(accumulator[perspective][lane]))
			offset += 2
		}
	}
	half := HiddenSize * 2
	return transitionSHA256(data), transitionSHA256(data[:half]), transitionSHA256(data[half:])
}

func parseTransitionPosition(t *testing.T, fen string) Position {
	t.Helper()
	position, err := parseOracleFEN(fen)
	if err != nil {
		t.Fatalf("parse transition FEN %q: %v", fen, err)
	}
	return position
}

func sameTransitionSet(got map[string]bool, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, value := range want {
		if !got[value] {
			return false
		}
	}
	return true
}

func readTransitionFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func decodeTransitionJSON(t *testing.T, data []byte, destination any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatalf("JSON has trailing value: %v", err)
	}
}

func transitionSHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func validTransitionSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}
