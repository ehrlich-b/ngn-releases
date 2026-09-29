// Command searchsnapshot records and compares deterministic one-thread search traces.
// Each fixture is recorded in a fresh child process so its first pass sees genuinely
// cold package state; a second pass in that same process records production warm state.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
)

const (
	manifestSchema = "ngn-search-snapshot-manifest/v2"
	snapshotSchema = "ngn-search-snapshot/v2"
)

var errBehaviorMismatch = errors.New("search behavior differs")

type sourceReference struct {
	Commit         string `json:"commit"`
	EngineTree     string `json:"engine_tree"`
	ArtifactPath   string `json:"artifact_path,omitempty"`
	ArtifactSHA256 string `json:"artifact_sha256,omitempty"`
}

type provenance struct {
	ArtifactSHA256 string `json:"artifact_sha256,omitempty"`
	Game           int    `json:"game,omitempty"`
	PrefixPlies    int    `json:"prefix_plies,omitempty"`
	Note           string `json:"note,omitempty"`
}

type fixture struct {
	Name            string      `json:"name"`
	Purpose         string      `json:"purpose"`
	StartFEN        string      `json:"start_fen"`
	Moves           string      `json:"moves,omitempty"`
	ExpectedRootFEN string      `json:"expected_root_fen"`
	Depth           int         `json:"depth"`
	MaxNodes        uint64      `json:"max_nodes,omitempty"`
	StopAfterDepth  int         `json:"stop_after_depth,omitempty"`
	Provenance      *provenance `json:"provenance,omitempty"`
}

type manifest struct {
	Schema          string          `json:"schema"`
	ReferenceSource sourceReference `json:"reference_source"`
	HashMB          int             `json:"hash_mb"`
	Fixtures        []fixture       `json:"fixtures"`
}

type keyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type buildIdentity struct {
	RequestedSource string          `json:"requested_source"`
	ReferenceSource sourceReference `json:"reference_source"`
	GoVersion       string          `json:"go_version"`
	Compiler        string          `json:"compiler"`
	GOOS            string          `json:"goos"`
	GOARCH          string          `json:"goarch"`
	MainPath        string          `json:"main_path,omitempty"`
	MainVersion     string          `json:"main_version,omitempty"`
	BuildSettings   []keyValue      `json:"build_settings,omitempty"`
}

type intOption struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type optionIdentity struct {
	HashMB           int             `json:"hash_mb"`
	SearchParameters []intOption     `json:"search_parameters"`
	SearchToggles    map[string]bool `json:"search_toggles"`
	EvalModelVersion string          `json:"eval_model_version"`
	EvalModelSHA256  string          `json:"eval_model_sha256"`
	EvalEnvironment  string          `json:"ngn_eval_w,omitempty"`
}

type historyCount struct {
	Hash  string `json:"hash"`
	Count int    `json:"count"`
}

type rootState struct {
	FEN         string         `json:"fen"`
	Hash        string         `json:"hash"`
	BoardSHA256 string         `json:"board_sha256"`
	LastMove    string         `json:"last_move"`
	PlayedPlies int            `json:"played_plies"`
	History     []historyCount `json:"history"`
}

type observation struct {
	Depth       int                        `json:"depth"`
	SelDepth    int                        `json:"seldepth"`
	RootDepth   int                        `json:"root_depth"`
	BestMove    string                     `json:"best_move"`
	Score       int                        `json:"score"`
	Nodes       uint64                     `json:"nodes"`
	Stopped     bool                       `json:"stopped"`
	PV          []string                   `json:"pv"`
	Diagnostics map[string]json.RawMessage `json:"diagnostics"`
}

type passSnapshot struct {
	State      string        `json:"state"`
	Iterations []observation `json:"iterations"`
	Final      observation   `json:"final"`
}

type fixtureSnapshot struct {
	Name    string         `json:"name"`
	Purpose string         `json:"purpose"`
	Input   fixture        `json:"input"`
	Root    rootState      `json:"root"`
	Passes  []passSnapshot `json:"passes"`
}

type snapshot struct {
	Schema   string            `json:"schema"`
	Identity buildIdentity     `json:"identity"`
	Options  optionIdentity    `json:"options"`
	Fixtures []fixtureSnapshot `json:"fixtures"`
}

type childRequest struct {
	HashMB  int     `json:"hash_mb"`
	Fixture fixture `json:"fixture"`
}

type difference struct {
	Path      string `json:"path"`
	Baseline  any    `json:"baseline,omitempty"`
	Candidate any    `json:"candidate,omitempty"`
}

type comparison struct {
	Equal               bool         `json:"equal"`
	MetadataDifferences []difference `json:"metadata_differences,omitempty"`
	BehaviorDifferences []difference `json:"behavior_differences,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "record":
		err = recordCommand(os.Args[2:])
	case "compare":
		err = compareCommand(os.Args[2:])
	case "record-one":
		err = recordOneCommand()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: searchsnapshot record -manifest FILE -source ID -out FILE")
	fmt.Fprintln(os.Stderr, "       searchsnapshot compare -baseline FILE -candidate FILE")
}

func recordCommand(args []string) error {
	fs := flag.NewFlagSet("record", flag.ContinueOnError)
	manifestPath := fs.String("manifest", "", "fixture manifest")
	source := fs.String("source", "", "exact accepted source or production-tree identity")
	out := fs.String("out", "", "snapshot output path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *manifestPath == "" || *source == "" || *out == "" {
		return errors.New("record requires -manifest, -source, and -out")
	}
	m, err := readManifest(*manifestPath)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate recorder executable: %w", err)
	}
	result := snapshot{
		Schema:   snapshotSchema,
		Identity: currentBuildIdentity(*source, m.ReferenceSource),
		Options:  currentOptions(m.HashMB),
		Fixtures: make([]fixtureSnapshot, 0, len(m.Fixtures)),
	}
	for _, f := range m.Fixtures {
		request, err := json.Marshal(childRequest{HashMB: m.HashMB, Fixture: f})
		if err != nil {
			return err
		}
		cmd := exec.Command(exe, "record-one")
		cmd.Stdin = bytes.NewReader(request)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("fixture %q child failed: %w: %s", f.Name, err, strings.TrimSpace(stderr.String()))
		}
		var one fixtureSnapshot
		if err := decodeExact(&stdout, &one); err != nil {
			return fmt.Errorf("fixture %q child output: %w", f.Name, err)
		}
		result.Fixtures = append(result.Fixtures, one)
	}
	return writeJSONAtomic(*out, result)
}

func recordOneCommand() error {
	var req childRequest
	if err := decodeExact(os.Stdin, &req); err != nil {
		return fmt.Errorf("decode fixture request: %w", err)
	}
	one, err := recordFixture(req.HashMB, req.Fixture)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(one)
}

func compareCommand(args []string) error {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	baselinePath := fs.String("baseline", "", "baseline snapshot")
	candidatePath := fs.String("candidate", "", "candidate snapshot")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *baselinePath == "" || *candidatePath == "" {
		return errors.New("compare requires -baseline and -candidate")
	}
	var baseline, candidate snapshot
	if err := readJSONFile(*baselinePath, &baseline); err != nil {
		return fmt.Errorf("baseline: %w", err)
	}
	if err := readJSONFile(*candidatePath, &candidate); err != nil {
		return fmt.Errorf("candidate: %w", err)
	}
	result, err := compareSnapshots(baseline, candidate)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return err
	}
	if !result.Equal {
		return errBehaviorMismatch
	}
	return nil
}

func readManifest(path string) (manifest, error) {
	var m manifest
	if err := readJSONFile(path, &m); err != nil {
		return m, err
	}
	if m.Schema != manifestSchema {
		return m, fmt.Errorf("unsupported manifest schema %q", m.Schema)
	}
	if m.HashMB < 1 {
		return m, errors.New("hash_mb must be at least 1")
	}
	if len(m.Fixtures) == 0 {
		return m, errors.New("manifest has no fixtures")
	}
	seen := make(map[string]bool, len(m.Fixtures))
	for _, f := range m.Fixtures {
		if f.Name == "" || f.StartFEN == "" || f.ExpectedRootFEN == "" || f.Depth < 1 {
			return m, fmt.Errorf("fixture %q lacks name/FEN/depth", f.Name)
		}
		if seen[f.Name] {
			return m, fmt.Errorf("duplicate fixture name %q", f.Name)
		}
		seen[f.Name] = true
		if f.StopAfterDepth < 0 || f.StopAfterDepth > f.Depth {
			return m, fmt.Errorf("fixture %q has invalid stop_after_depth", f.Name)
		}
	}
	return m, nil
}

func readJSONFile(path string, dst any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return decodeExact(f, dst)
}

func decodeExact(r io.Reader, dst any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return fmt.Errorf("trailing data: %w", err)
	}
	return nil
}

func writeJSONAtomic(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".searchsnapshot-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	enc := json.NewEncoder(tmp)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func currentBuildIdentity(source string, ref sourceReference) buildIdentity {
	id := buildIdentity{
		RequestedSource: source,
		ReferenceSource: ref,
		GoVersion:       runtime.Version(),
		Compiler:        runtime.Compiler,
		GOOS:            runtime.GOOS,
		GOARCH:          runtime.GOARCH,
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		id.MainPath = info.Main.Path
		id.MainVersion = info.Main.Version
		id.BuildSettings = make([]keyValue, 0, len(info.Settings))
		for _, s := range info.Settings {
			id.BuildSettings = append(id.BuildSettings, keyValue{Key: s.Key, Value: s.Value})
		}
		sort.Slice(id.BuildSettings, func(i, j int) bool { return id.BuildSettings[i].Key < id.BuildSettings[j].Key })
	}
	return id
}

func currentOptions(hashMB int) optionIdentity {
	params := make([]intOption, 0, len(engine.TunableSearchParams))
	for _, p := range engine.TunableSearchParams {
		params = append(params, intOption{Name: p.Name, Value: *p.Ptr})
	}
	model := engine.ExportTexelModel()
	modelJSON, err := json.Marshal(model)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(modelJSON)
	return optionIdentity{
		HashMB:           hashMB,
		SearchParameters: params,
		SearchToggles: map[string]bool{
			"NullMove": engine.SearchToggles.NullMove, "Futility": engine.SearchToggles.Futility,
			"RFP": engine.SearchToggles.RFP, "Probcut": engine.SearchToggles.Probcut,
			"LMP": engine.SearchToggles.LMP, "SEEPrune": engine.SearchToggles.SEEPrune,
			"LMR": engine.SearchToggles.LMR, "HistPrune": engine.SearchToggles.HistPrune,
			"IID": engine.SearchToggles.IID, "Singular": engine.SearchToggles.Singular,
		},
		EvalModelVersion: model.Version,
		EvalModelSHA256:  hex.EncodeToString(digest[:]),
		EvalEnvironment:  os.Getenv("NGN_EVAL_W"),
	}
}

func recordFixture(hashMB int, f fixture) (fixtureSnapshot, error) {
	if hashMB < 1 {
		return fixtureSnapshot{}, errors.New("hash_mb must be at least 1")
	}
	searcher, err := engine.NewSearchEngineWithHash(hashMB)
	if err != nil {
		return fixtureSnapshot{}, err
	}
	searcher.SetMaxNodes(f.MaxNodes)

	root, err := engine.ParseFEN(f.StartFEN)
	if err != nil {
		return fixtureSnapshot{}, fmt.Errorf("fixture %q start FEN: %w", f.Name, err)
	}
	moves := strings.Fields(f.Moves)
	for i, text := range moves {
		move, err := engine.ParseLongAlgebraicNotation(text, root)
		if err != nil {
			return fixtureSnapshot{}, fmt.Errorf("fixture %q move %d %q: %w", f.Name, i+1, text, err)
		}
		if _, _, _, ok := root.GameMakeMove(move); !ok {
			return fixtureSnapshot{}, fmt.Errorf("fixture %q move %d %q was rejected", f.Name, i+1, text)
		}
		// Match UCI handlePosition: the predecessor is part of root move-order state.
		searcher.SetLastMovePlayed(move)
	}
	rootBefore := snapshotRoot(root, len(moves), searcher.GetLastMovePlayed())
	if rootBefore.FEN != f.ExpectedRootFEN {
		return fixtureSnapshot{}, fmt.Errorf("fixture %q root FEN %q, want %q", f.Name, rootBefore.FEN, f.ExpectedRootFEN)
	}

	one := fixtureSnapshot{Name: f.Name, Purpose: f.Purpose, Input: f, Root: rootBefore}
	for _, state := range []string{"cold", "warm"} {
		searcher.ClearStop()
		searcher.SetMaxNodes(f.MaxNodes)
		searchPos := root.Copy()
		boardBefore := searchPos.Board
		pass := passSnapshot{State: state, Iterations: make([]observation, 0)}
		result := searcher.SearchIterativeDeepeningWithCallback(searchPos, f.Depth, nil, func(info *engine.SearchInfo) {
			pass.Iterations = append(pass.Iterations, snapshotObservation(info))
			if f.StopAfterDepth != 0 && info.Depth >= f.StopAfterDepth {
				searcher.RequestStop()
			}
		})
		pass.Final = snapshotObservation(result)
		after := snapshotRoot(searchPos, len(moves), searcher.GetLastMovePlayed())
		if !reflect.DeepEqual(boardBefore, searchPos.Board) {
			return fixtureSnapshot{}, fmt.Errorf("fixture %q %s search changed hidden board state", f.Name, state)
		}
		if !reflect.DeepEqual(rootBefore, after) {
			return fixtureSnapshot{}, fmt.Errorf("fixture %q %s search changed root state", f.Name, state)
		}
		one.Passes = append(one.Passes, pass)
	}
	return one, nil
}

func snapshotRoot(pos *engine.Position, played int, lastMove engine.Move) rootState {
	history := make([]historyCount, 0, len(pos.Positions))
	for hash, count := range pos.Positions {
		history = append(history, historyCount{Hash: hashString(hash), Count: count})
	}
	sort.Slice(history, func(i, j int) bool { return history[i].Hash < history[j].Hash })
	return rootState{
		FEN: engine.GenerateFEN(pos), Hash: hashString(pos.Hash()),
		BoardSHA256: boardDigest(pos.Board), LastMove: moveString(lastMove),
		PlayedPlies: played, History: history,
	}
}

// boardDigest covers every field of Bitboard, including its private mailbox and
// incremental MG/EG/phase accumulators. Reflection reads by kind rather than via
// Interface, so unexported fields remain included without unsafe memory or padding.
func boardDigest(board engine.Bitboard) string {
	h := sha256.New()
	writeDigestValue(h, reflect.ValueOf(board))
	return hex.EncodeToString(h.Sum(nil))
}

func writeDigestValue(h hash.Hash, value reflect.Value) {
	writeDigestString(h, value.Type().String())
	switch value.Kind() {
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			writeDigestString(h, value.Type().Field(i).Name)
			writeDigestValue(h, value.Field(i))
		}
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			writeDigestValue(h, value.Index(i))
		}
	case reflect.Bool:
		if value.Bool() {
			h.Write([]byte{1})
		} else {
			h.Write([]byte{0})
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var encoded [8]byte
		binary.LittleEndian.PutUint64(encoded[:], uint64(value.Int()))
		h.Write(encoded[:])
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		var encoded [8]byte
		binary.LittleEndian.PutUint64(encoded[:], value.Uint())
		h.Write(encoded[:])
	default:
		panic(fmt.Sprintf("unsupported board digest kind %s in %s", value.Kind(), value.Type()))
	}
}

func writeDigestString(h hash.Hash, value string) {
	var size [8]byte
	binary.LittleEndian.PutUint64(size[:], uint64(len(value)))
	h.Write(size[:])
	io.WriteString(h, value)
}

func snapshotObservation(info *engine.SearchInfo) observation {
	pvLength := info.PVLength
	if pvLength > info.Depth {
		pvLength = info.Depth
	}
	pv := make([]string, pvLength)
	for i := 0; i < pvLength; i++ {
		pv[i] = moveString(info.PV[i])
	}
	return observation{
		Depth: info.Depth, SelDepth: info.SelDepth, RootDepth: info.RootDepth,
		BestMove: moveString(info.BestMove), Score: info.BestScore, Nodes: info.Nodes,
		Stopped: info.Stopped, PV: pv, Diagnostics: deterministicDiagnostics(info),
	}
}

func deterministicDiagnostics(info *engine.SearchInfo) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage)
	value := reflect.ValueOf(info).Elem()
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := typ.Field(i)
		if field.Name == "Nodes" || field.PkgPath != "" || !isUint64Tree(field.Type) {
			continue
		}
		encoded, err := json.Marshal(value.Field(i).Interface())
		if err != nil {
			panic(err)
		}
		out[field.Name] = encoded
	}
	return out
}

func isUint64Tree(t reflect.Type) bool {
	if t.Kind() == reflect.Uint64 {
		return true
	}
	return t.Kind() == reflect.Array && isUint64Tree(t.Elem())
}

func moveString(move engine.Move) string {
	if move == engine.EmptyMove {
		return "0000"
	}
	return move.ToString()
}

func hashString(hash uint64) string { return fmt.Sprintf("%016x", hash) }

func compareSnapshots(baseline, candidate snapshot) (comparison, error) {
	if baseline.Schema != snapshotSchema || candidate.Schema != snapshotSchema {
		return comparison{}, fmt.Errorf("snapshots must both use schema %q", snapshotSchema)
	}
	var result comparison
	if err := compareJSON("identity", baseline.Identity, candidate.Identity, &result.MetadataDifferences); err != nil {
		return result, err
	}
	if err := compareJSON("options", baseline.Options, candidate.Options, &result.BehaviorDifferences); err != nil {
		return result, err
	}
	if err := compareJSON("fixtures", baseline.Fixtures, candidate.Fixtures, &result.BehaviorDifferences); err != nil {
		return result, err
	}
	result.Equal = len(result.BehaviorDifferences) == 0
	return result, nil
}

func compareJSON(path string, baseline, candidate any, diffs *[]difference) error {
	a, err := jsonValue(baseline)
	if err != nil {
		return err
	}
	b, err := jsonValue(candidate)
	if err != nil {
		return err
	}
	compareValue(path, a, b, diffs)
	return nil
}

func jsonValue(value any) (any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func compareValue(path string, baseline, candidate any, diffs *[]difference) {
	aMap, aIsMap := baseline.(map[string]any)
	bMap, bIsMap := candidate.(map[string]any)
	if aIsMap || bIsMap {
		if !aIsMap || !bIsMap {
			*diffs = append(*diffs, difference{Path: path, Baseline: baseline, Candidate: candidate})
			return
		}
		keys := make(map[string]struct{}, len(aMap)+len(bMap))
		for key := range aMap {
			keys[key] = struct{}{}
		}
		for key := range bMap {
			keys[key] = struct{}{}
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			a, aOK := aMap[key]
			b, bOK := bMap[key]
			if !aOK || !bOK {
				*diffs = append(*diffs, difference{Path: path + "." + key, Baseline: a, Candidate: b})
				continue
			}
			compareValue(path+"."+key, a, b, diffs)
		}
		return
	}
	aSlice, aIsSlice := baseline.([]any)
	bSlice, bIsSlice := candidate.([]any)
	if aIsSlice || bIsSlice {
		if !aIsSlice || !bIsSlice || len(aSlice) != len(bSlice) {
			*diffs = append(*diffs, difference{Path: path, Baseline: baseline, Candidate: candidate})
			return
		}
		for i := range aSlice {
			compareValue(path+"["+strconv.Itoa(i)+"]", aSlice[i], bSlice[i], diffs)
		}
		return
	}
	if !reflect.DeepEqual(baseline, candidate) {
		*diffs = append(*diffs, difference{Path: path, Baseline: baseline, Candidate: candidate})
	}
}
