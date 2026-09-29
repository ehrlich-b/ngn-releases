package sf18small

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode"

	base "github.com/ehrlich-b/ngn/nnue"
)

const (
	officialOracleEnvironment   = "NGN_SF18_SMALL_ORACLE_JSONL"
	officialNetworkSHA256       = "37f18f62d772f3107e1d6aaca3898c130c3c86f2ab63e6555fbbca20635a899d"
	officialSourceCommit        = "cb3d4ee9b47d0c5aae855b12379378ea1439675c"
	officialOracleCaseCount     = 145
	officialOracleFixtureSHA256 = "b350019a797aa6c87ea2af55d0490aab75b3ce2adc31bead094f39a2ee756405"
)

type upstreamOracleCase struct {
	SourceCommit       string                 `json:"source_commit"`
	NetworkSHA256      string                 `json:"network_sha256"`
	FEN                string                 `json:"fen"`
	SideToMove         base.Color             `json:"side_to_move"`
	Pieces             []base.PieceOnSquare   `json:"pieces"`
	CorrectBucket      uint8                  `json:"correct_bucket"`
	Active             [2][]uint16            `json:"active"`
	Accumulator        [2][128]int16          `json:"accumulator"`
	PSQTAccumulator    [2][8]int32            `json:"psqt_accumulator"`
	Transformed        [128]uint8             `json:"transformed"`
	PSQTRaw            [8]int32               `json:"psqt_raw"`
	PSQTDifferenceWide [8]int64               `json:"psqt_difference_wide"`
	AccumulatorWideMin [2]int64               `json:"accumulator_wide_min"`
	AccumulatorWideMax [2]int64               `json:"accumulator_wide_max"`
	PSQTWideMin        [2]int64               `json:"psqt_wide_min"`
	PSQTWideMax        [2]int64               `json:"psqt_wide_max"`
	Stacks             [8]upstreamOracleStack `json:"stacks"`
	Components         [8]Components          `json:"components"`
}

type upstreamOracleStack struct {
	FC0            [16]int32 `json:"fc0"`
	Squared        [15]uint8 `json:"squared"`
	Clipped0       [15]uint8 `json:"clipped0"`
	FC1            [32]int32 `json:"fc1"`
	Clipped1       [32]uint8 `json:"clipped1"`
	FC2            int32     `json:"fc2"`
	Forward        int32     `json:"forward"`
	PositionalRaw  int32     `json:"positional_raw"`
	FC0WideMin     int64     `json:"fc0_wide_min"`
	FC0WideMax     int64     `json:"fc0_wide_max"`
	FC1WideMin     int64     `json:"fc1_wide_min"`
	FC1WideMax     int64     `json:"fc1_wide_max"`
	FC2WideMin     int64     `json:"fc2_wide_min"`
	FC2WideMax     int64     `json:"fc2_wide_max"`
	ForwardWide    int64     `json:"forward_wide"`
	PositionalWide int64     `json:"positional_wide"`
}

type oracleExpectation struct {
	id       string
	fen      string
	position base.Position
}

func TestFrozenOracleFixtureIdentityAndKingCoverage(t *testing.T) {
	_ = boundOracleExpectations(t)
}

func boundOracleExpectations(t *testing.T) []oracleExpectation {
	t.Helper()
	const path = "testdata/upstream_oracle/oracle_fens.txt"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if got := hex.EncodeToString(digest[:]); got != officialOracleFixtureSHA256 {
		t.Fatalf("oracle fixture digest = %s, want %s", got, officialOracleFixtureSHA256)
	}
	expected, err := decodeOracleExpectations(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) != officialOracleCaseCount {
		t.Fatalf("frozen oracle fixture count = %d, want %d", len(expected), officialOracleCaseCount)
	}
	foundInCheck := false
	for _, row := range expected {
		if row.id == "in-check-static" {
			foundInCheck = true
		}
	}
	if !foundInCheck {
		t.Fatal("frozen oracle fixture misses in-check direct-hook case")
	}
	assertKingBucketCoverage(t, expected)
	return expected
}

func TestOfficialAllLaneUpstreamOracle(t *testing.T) {
	expected := boundOracleExpectations(t)
	networkPath := os.Getenv(officialFileEnvironment)
	oraclePath := os.Getenv(officialOracleEnvironment)
	if networkPath == "" || oraclePath == "" {
		t.Skip("official network and upstream oracle JSONL are required for this explicit gate")
	}

	network, err := os.Open(networkPath)
	if err != nil {
		t.Fatal(err)
	}
	model, err := Load(network)
	network.Close()
	if err != nil {
		t.Fatal(err)
	}
	metadata := model.Metadata()
	if digest := hex.EncodeToString(metadata.FileSHA256[:]); digest != officialNetworkSHA256 {
		t.Fatalf("loaded model digest = %s, want %s", digest, officialNetworkSHA256)
	}

	oracle, err := os.Open(oraclePath)
	if err != nil {
		t.Fatal(err)
	}
	defer oracle.Close()
	rows, err := decodeOracleRows(oracle, expected)
	if err != nil {
		t.Fatal(err)
	}
	for index, want := range rows {
		if want.SourceCommit != officialSourceCommit || want.NetworkSHA256 != officialNetworkSHA256 {
			t.Fatalf("oracle row %d provenance = %q/%q", index, want.SourceCommit, want.NetworkSHA256)
		}
		assertOfficialOracleNoOverflow(t, index, want)
		got, err := model.evaluateAllTrace(expected[index].position)
		if err != nil {
			t.Fatalf("oracle row %d %q: %v", index, want.FEN, err)
		}
		compareUpstreamOracle(t, index, got, want)
		selected, err := model.EvaluateSelected(expected[index].position)
		if err != nil {
			t.Fatalf("oracle row %d selected evaluation: %v", index, err)
		}
		if selected.Bucket != want.CorrectBucket || selected.Components != want.Components[want.CorrectBucket] {
			t.Fatalf("oracle row %d selected = %+v, want bucket %d components %+v", index, selected, want.CorrectBucket, want.Components[want.CorrectBucket])
		}
		reversed := append([]base.PieceOnSquare(nil), expected[index].position.Pieces...)
		for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
			reversed[left], reversed[right] = reversed[right], reversed[left]
		}
		reordered, err := model.EvaluateAll(base.Position{SideToMove: want.SideToMove, Pieces: reversed})
		if err != nil || reordered != got.public {
			t.Fatalf("oracle row %d shuffled public result = %+v, %v", index, reordered, err)
		}
	}
}

func decodeOracleExpectations(reader io.Reader) ([]oracleExpectation, error) {
	var result []oracleExpectation
	ids := make(map[string]bool)
	fens := make(map[string]bool)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid oracle fixture line %q", line)
		}
		if ids[parts[0]] || fens[parts[1]] {
			return nil, fmt.Errorf("duplicate oracle fixture %q", line)
		}
		position, err := parseOracleFEN(parts[1])
		if err != nil {
			return nil, fmt.Errorf("oracle fixture %s: %w", parts[0], err)
		}
		ids[parts[0]], fens[parts[1]] = true, true
		result = append(result, oracleExpectation{id: parts[0], fen: parts[1], position: position})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("oracle fixture is empty")
	}
	return result, nil
}

func parseOracleFEN(fen string) (base.Position, error) {
	fields := strings.Fields(fen)
	if len(fields) != 6 {
		return base.Position{}, fmt.Errorf("FEN has %d fields", len(fields))
	}
	ranks := strings.Split(fields[0], "/")
	if len(ranks) != 8 {
		return base.Position{}, fmt.Errorf("FEN has %d ranks", len(ranks))
	}
	var board [64]base.PieceOnSquare
	var occupied [64]bool
	for fenRank, encoded := range ranks {
		file := 0
		for _, symbol := range encoded {
			if symbol >= '1' && symbol <= '8' {
				file += int(symbol - '0')
				continue
			}
			if file >= 8 {
				return base.Position{}, fmt.Errorf("rank %d exceeds eight files", 8-fenRank)
			}
			piece, color, ok := oracleFENPiece(symbol)
			if !ok {
				return base.Position{}, fmt.Errorf("invalid FEN piece %q", symbol)
			}
			square := (7-fenRank)*8 + file
			board[square] = sfPiece(piece, color, base.Square(square))
			occupied[square] = true
			file++
		}
		if file != 8 {
			return base.Position{}, fmt.Errorf("rank %d has %d files", 8-fenRank, file)
		}
	}
	position := base.Position{}
	switch fields[1] {
	case "w":
		position.SideToMove = base.White
	case "b":
		position.SideToMove = base.Black
	default:
		return base.Position{}, fmt.Errorf("invalid side to move %q", fields[1])
	}
	for square := range board {
		if occupied[square] {
			position.Pieces = append(position.Pieces, board[square])
		}
	}
	if _, _, _, err := validatePosition(position); err != nil {
		return base.Position{}, err
	}
	return position, nil
}

func oracleFENPiece(symbol rune) (base.PieceType, base.Color, bool) {
	color := base.Black
	if unicode.IsUpper(symbol) {
		color = base.White
		symbol = unicode.ToLower(symbol)
	}
	pieces := map[rune]base.PieceType{'p': base.Pawn, 'n': base.Knight, 'b': base.Bishop, 'r': base.Rook, 'q': base.Queen, 'k': base.King}
	piece, ok := pieces[symbol]
	return piece, color, ok
}

func decodeOracleRows(reader io.Reader, expected []oracleExpectation) ([]upstreamOracleCase, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	rows := make([]upstreamOracleCase, 0, len(expected))
	for scanner.Scan() {
		index := len(rows)
		if index >= len(expected) {
			return nil, fmt.Errorf("oracle has extra row %d", index)
		}
		row, err := decodeOracleCase(scanner.Bytes())
		if err != nil {
			return nil, fmt.Errorf("oracle row %d: %w", index, err)
		}
		want := expected[index]
		if row.FEN != want.fen {
			return nil, fmt.Errorf("oracle row %d FEN = %q, want %q", index, row.FEN, want.fen)
		}
		if row.SideToMove != want.position.SideToMove || !reflect.DeepEqual(row.Pieces, want.position.Pieces) {
			return nil, fmt.Errorf("oracle row %d board/side does not match independently parsed fixture %s", index, want.id)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(rows) != len(expected) {
		return nil, fmt.Errorf("oracle has %d rows, want %d", len(rows), len(expected))
	}
	return rows, nil
}

func decodeOracleCase(data []byte) (upstreamOracleCase, error) {
	if err := rejectJSONNull(data); err != nil {
		return upstreamOracleCase{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return upstreamOracleCase{}, err
	}
	if err := requireJSONObject(raw, "row", []string{
		"source_commit", "network_sha256", "fen", "side_to_move", "pieces", "correct_bucket",
		"active", "accumulator", "psqt_accumulator", "transformed", "psqt_raw",
		"psqt_difference_wide", "accumulator_wide_min", "accumulator_wide_max",
		"psqt_wide_min", "psqt_wide_max", "stacks", "components",
	}); err != nil {
		return upstreamOracleCase{}, err
	}
	pieces, err := requireJSONArray(raw["pieces"], "pieces", -1)
	if err != nil {
		return upstreamOracleCase{}, err
	}
	for index, piece := range pieces {
		object, err := rawObject(piece, fmt.Sprintf("pieces[%d]", index))
		if err != nil {
			return upstreamOracleCase{}, err
		}
		if err := requireJSONObject(object, fmt.Sprintf("pieces[%d]", index), []string{"Piece", "Color", "Square"}); err != nil {
			return upstreamOracleCase{}, err
		}
	}
	active, err := requireJSONArray(raw["active"], "active", 2)
	if err != nil {
		return upstreamOracleCase{}, err
	}
	for perspective := range active {
		if _, err := requireJSONArray(active[perspective], fmt.Sprintf("active[%d]", perspective), len(pieces)); err != nil {
			return upstreamOracleCase{}, err
		}
	}
	if err := requireMatrix(raw["accumulator"], "accumulator", 2, 128); err != nil {
		return upstreamOracleCase{}, err
	}
	if err := requireMatrix(raw["psqt_accumulator"], "psqt_accumulator", 2, 8); err != nil {
		return upstreamOracleCase{}, err
	}
	for _, shape := range []struct {
		name string
		len  int
	}{{"transformed", 128}, {"psqt_raw", 8}, {"psqt_difference_wide", 8}, {"accumulator_wide_min", 2}, {"accumulator_wide_max", 2}, {"psqt_wide_min", 2}, {"psqt_wide_max", 2}} {
		if _, err := requireJSONArray(raw[shape.name], shape.name, shape.len); err != nil {
			return upstreamOracleCase{}, err
		}
	}
	stacks, err := requireJSONArray(raw["stacks"], "stacks", 8)
	if err != nil {
		return upstreamOracleCase{}, err
	}
	for index, stack := range stacks {
		name := fmt.Sprintf("stacks[%d]", index)
		object, err := rawObject(stack, name)
		if err != nil {
			return upstreamOracleCase{}, err
		}
		if err := requireJSONObject(object, name, []string{
			"fc0", "squared", "clipped0", "fc1", "clipped1", "fc2", "forward", "positional_raw",
			"fc0_wide_min", "fc0_wide_max", "fc1_wide_min", "fc1_wide_max",
			"fc2_wide_min", "fc2_wide_max", "forward_wide", "positional_wide",
		}); err != nil {
			return upstreamOracleCase{}, err
		}
		for _, shape := range []struct {
			name string
			len  int
		}{{"fc0", 16}, {"squared", 15}, {"clipped0", 15}, {"fc1", 32}, {"clipped1", 32}} {
			if _, err := requireJSONArray(object[shape.name], name+"."+shape.name, shape.len); err != nil {
				return upstreamOracleCase{}, err
			}
		}
	}
	components, err := requireJSONArray(raw["components"], "components", 8)
	if err != nil {
		return upstreamOracleCase{}, err
	}
	for index, component := range components {
		name := fmt.Sprintf("components[%d]", index)
		object, err := rawObject(component, name)
		if err != nil {
			return upstreamOracleCase{}, err
		}
		if err := requireJSONObject(object, name, []string{"PSQT", "Positional"}); err != nil {
			return upstreamOracleCase{}, err
		}
	}

	var result upstreamOracleCase
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return upstreamOracleCase{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return upstreamOracleCase{}, fmt.Errorf("trailing JSON value")
	}
	return result, nil
}

func rejectJSONNull(data []byte) error {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	return rejectJSONNullValue(value, "row")
}

func rejectJSONNullValue(value any, path string) error {
	switch value := value.(type) {
	case nil:
		return fmt.Errorf("%s is null", path)
	case []any:
		for index, child := range value {
			if err := rejectJSONNullValue(child, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	case map[string]any:
		for name, child := range value {
			if err := rejectJSONNullValue(child, path+"."+name); err != nil {
				return err
			}
		}
	}
	return nil
}

func rawObject(raw json.RawMessage, name string) (map[string]json.RawMessage, error) {
	var result map[string]json.RawMessage
	if err := json.Unmarshal(raw, &result); err != nil || result == nil {
		return nil, fmt.Errorf("%s is not an object", name)
	}
	return result, nil
}

func requireJSONObject(object map[string]json.RawMessage, name string, fields []string) error {
	if len(object) != len(fields) {
		return fmt.Errorf("%s has %d fields, want %d", name, len(object), len(fields))
	}
	for _, field := range fields {
		if _, ok := object[field]; !ok {
			return fmt.Errorf("%s is missing field %s", name, field)
		}
	}
	return nil
}

func requireJSONArray(raw json.RawMessage, name string, length int) ([]json.RawMessage, error) {
	var result []json.RawMessage
	if err := json.Unmarshal(raw, &result); err != nil || result == nil {
		return nil, fmt.Errorf("%s is not an array", name)
	}
	if length >= 0 && len(result) != length {
		return nil, fmt.Errorf("%s has length %d, want %d", name, len(result), length)
	}
	return result, nil
}

func requireMatrix(raw json.RawMessage, name string, outer, inner int) error {
	rows, err := requireJSONArray(raw, name, outer)
	if err != nil {
		return err
	}
	for index, row := range rows {
		if _, err := requireJSONArray(row, fmt.Sprintf("%s[%d]", name, index), inner); err != nil {
			return err
		}
	}
	return nil
}

func assertKingBucketCoverage(t *testing.T, expected []oracleExpectation) {
	t.Helper()
	var covered [2][32][2]bool
	for _, row := range expected {
		var kings [2]base.Square
		for _, piece := range row.position.Pieces {
			if piece.Piece == base.King {
				kings[piece.Color] = piece.Square
			}
		}
		for perspective := base.White; perspective <= base.Black; perspective++ {
			canonical := kings[perspective] ^ base.Square(56*perspective)
			half := 0
			if canonical&7 >= 4 {
				half = 1
			}
			covered[perspective][kingBucket[canonical]][half] = true
		}
	}
	for perspective := base.White; perspective <= base.Black; perspective++ {
		for bucket := 0; bucket < 32; bucket++ {
			for half := 0; half < 2; half++ {
				if !covered[perspective][bucket][half] {
					t.Fatalf("fixture misses perspective %d king bucket %d mirror half %d", perspective, bucket, half)
				}
			}
		}
	}
}

func assertOfficialOracleNoOverflow(t *testing.T, row int, oracle upstreamOracleCase) {
	t.Helper()
	const (
		minimumInt16 = -1 << 15
		maximumInt16 = 1<<15 - 1
		minimumInt32 = -1 << 31
		maximumInt32 = 1<<31 - 1
	)
	for perspective := 0; perspective < 2; perspective++ {
		if oracle.AccumulatorWideMin[perspective] < minimumInt16 || oracle.AccumulatorWideMax[perspective] > maximumInt16 {
			t.Fatalf("oracle row %d perspective %d accumulator overflows int16: [%d,%d]", row, perspective, oracle.AccumulatorWideMin[perspective], oracle.AccumulatorWideMax[perspective])
		}
		if oracle.PSQTWideMin[perspective] < minimumInt32 || oracle.PSQTWideMax[perspective] > maximumInt32 {
			t.Fatalf("oracle row %d perspective %d PSQT accumulation overflows int32: [%d,%d]", row, perspective, oracle.PSQTWideMin[perspective], oracle.PSQTWideMax[perspective])
		}
	}
	for bucket, difference := range oracle.PSQTDifferenceWide {
		if difference < minimumInt32 || difference > maximumInt32 {
			t.Fatalf("oracle row %d bucket %d PSQT subtraction overflows int32: %d", row, bucket, difference)
		}
		stack := oracle.Stacks[bucket]
		for name, bounds := range map[string][2]int64{
			"fc0": {stack.FC0WideMin, stack.FC0WideMax},
			"fc1": {stack.FC1WideMin, stack.FC1WideMax},
			"fc2": {stack.FC2WideMin, stack.FC2WideMax},
		} {
			if bounds[0] < minimumInt32 || bounds[1] > maximumInt32 {
				t.Fatalf("oracle row %d bucket %d %s overflows int32: [%d,%d]", row, bucket, name, bounds[0], bounds[1])
			}
		}
		if stack.ForwardWide < minimumInt32 || stack.ForwardWide > maximumInt32 {
			t.Fatalf("oracle row %d bucket %d forward multiply overflows int32: %d", row, bucket, stack.ForwardWide)
		}
		if stack.PositionalWide < minimumInt32 || stack.PositionalWide > maximumInt32 {
			t.Fatalf("oracle row %d bucket %d positional addition overflows int32: %d", row, bucket, stack.PositionalWide)
		}
	}
}

func compareUpstreamOracle(t *testing.T, row int, got evaluationTrace, want upstreamOracleCase) {
	t.Helper()
	if got.public.CorrectBucket != want.CorrectBucket || got.public.Buckets != want.Components {
		t.Fatalf("oracle row %d public trace differs", row)
	}
	for perspective := 0; perspective < 2; perspective++ {
		active := got.accumulator.active[perspective][:got.accumulator.activeCount[perspective]]
		if !reflect.DeepEqual(active, want.Active[perspective]) {
			t.Fatalf("oracle row %d perspective %d active indices differ", row, perspective)
		}
	}
	if got.accumulator.values != want.Accumulator || got.accumulator.psqt != want.PSQTAccumulator ||
		got.accumulator.wideMin != want.AccumulatorWideMin || got.accumulator.wideMax != want.AccumulatorWideMax ||
		got.accumulator.psqtWideMin != want.PSQTWideMin || got.accumulator.psqtWideMax != want.PSQTWideMax ||
		got.transformed != want.Transformed || got.psqtRaw != want.PSQTRaw ||
		got.psqtDifferenceWide != want.PSQTDifferenceWide {
		t.Fatalf("oracle row %d transformer trace differs", row)
	}
	for bucket := 0; bucket < layerStacks; bucket++ {
		stack := got.stacks[bucket]
		wantStack := want.Stacks[bucket]
		if stack.fc0 != wantStack.FC0 || stack.squared != wantStack.Squared ||
			stack.clipped0 != wantStack.Clipped0 || stack.fc1 != wantStack.FC1 ||
			stack.clipped1 != wantStack.Clipped1 || stack.fc2 != wantStack.FC2 ||
			stack.forward != wantStack.Forward || stack.positionalRaw != wantStack.PositionalRaw ||
			stack.fc0WideMin != wantStack.FC0WideMin || stack.fc0WideMax != wantStack.FC0WideMax ||
			stack.fc1WideMin != wantStack.FC1WideMin || stack.fc1WideMax != wantStack.FC1WideMax ||
			stack.fc2WideMin != wantStack.FC2WideMin || stack.fc2WideMax != wantStack.FC2WideMax ||
			stack.forwardWide != wantStack.ForwardWide || stack.positionalWide != wantStack.PositionalWide {
			t.Fatalf("oracle row %d stack %d differs", row, bucket)
		}
	}
}

func TestOracleRowSequenceBindsFrozenFixture(t *testing.T) {
	firstFEN := "4k3/8/8/8/8/8/8/4K3 w - - 0 1"
	secondFEN := "7k/8/8/8/8/8/8/K7 b - - 0 1"
	expected := []oracleExpectation{
		oracleExpectationForTest(t, "first", firstFEN),
		oracleExpectationForTest(t, "second", secondFEN),
	}
	first := oracleJSONForTest(t, firstFEN)
	second := oracleJSONForTest(t, secondFEN)
	changedFEN := mutateOracleJSON(t, first, func(object map[string]any) {
		object["fen"] = secondFEN
	})
	changedBoard := mutateOracleJSON(t, first, func(object map[string]any) {
		object["pieces"] = json.RawMessage(`[ {"Piece":5,"Color":0,"Square":0}, {"Piece":5,"Color":1,"Square":63} ]`)
	})
	tests := []struct {
		name string
		rows [][]byte
	}{
		{"missing row", [][]byte{first}},
		{"duplicate row", [][]byte{first, first}},
		{"changed FEN", [][]byte{changedFEN, second}},
		{"changed board", [][]byte{changedBoard, second}},
		{"extra row", [][]byte{first, second, second}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var input bytes.Buffer
			for _, row := range test.rows {
				input.Write(row)
				input.WriteByte('\n')
			}
			if _, err := decodeOracleRows(&input, expected); err == nil {
				t.Fatal("malformed oracle row sequence was accepted")
			}
		})
	}
}

func TestOracleShapeRejectsShortExtraAndMissingZeroField(t *testing.T) {
	fen := "4k3/8/8/8/8/8/8/4K3 w - - 0 1"
	valid := oracleJSONForTest(t, fen)
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"short tensor", func(object map[string]any) {
			object["transformed"] = make([]int, 127)
		}},
		{"extra tensor lane", func(object map[string]any) {
			stacks := object["stacks"].([]any)
			first := stacks[0].(map[string]any)
			first["fc0"] = append(first["fc0"].([]any), 0)
		}},
		{"missing zero-valued field", func(object map[string]any) {
			stacks := object["stacks"].([]any)
			delete(stacks[0].(map[string]any), "forward")
		}},
		{"null zero-valued scalar", func(object map[string]any) {
			object["correct_bucket"] = nil
		}},
		{"null zero-valued tensor lane", func(object map[string]any) {
			transformed := object["transformed"].([]any)
			transformed[0] = nil
		}},
	}
	if _, err := decodeOracleCase(valid); err != nil {
		t.Fatalf("valid synthetic oracle shape: %v", err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeOracleCase(mutateOracleJSON(t, valid, test.mutate)); err == nil {
				t.Fatal("malformed oracle shape was accepted")
			}
		})
	}
}

func oracleExpectationForTest(t *testing.T, id, fen string) oracleExpectation {
	t.Helper()
	position, err := parseOracleFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	return oracleExpectation{id: id, fen: fen, position: position}
}

func oracleJSONForTest(t *testing.T, fen string) []byte {
	t.Helper()
	position, err := parseOracleFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	row := upstreamOracleCase{
		SourceCommit:  officialSourceCommit,
		NetworkSHA256: officialNetworkSHA256,
		FEN:           fen,
		SideToMove:    position.SideToMove,
		Pieces:        position.Pieces,
	}
	row.Active[0] = make([]uint16, len(position.Pieces))
	row.Active[1] = make([]uint16, len(position.Pieces))
	result, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func mutateOracleJSON(t *testing.T, original []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	var object map[string]any
	decoder := json.NewDecoder(bytes.NewReader(original))
	decoder.UseNumber()
	if err := decoder.Decode(&object); err != nil {
		t.Fatal(err)
	}
	mutate(object)
	result, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
