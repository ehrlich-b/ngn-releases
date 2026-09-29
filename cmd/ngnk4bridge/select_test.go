package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/ehrlich-b/ngn/nnue/ngnk4"
)

func optimizerStateFixture(tensors ...struct {
	name   string
	values []float32
}) []byte {
	var output bytes.Buffer
	var word [8]byte
	for _, tensor := range tensors {
		output.WriteString(tensor.name)
		output.WriteByte('\n')
		binary.LittleEndian.PutUint64(word[:], uint64(len(tensor.values)))
		output.Write(word[:])
		for _, value := range tensor.values {
			var encoded [4]byte
			binary.LittleEndian.PutUint32(encoded[:], math.Float32bits(value))
			output.Write(encoded[:])
		}
	}
	return output.Bytes()
}

func TestDecodeSelectionPositionUsesNormalizedSideToMoveBoard(t *testing.T) {
	var record [32]byte
	occupied := uint64(1)<<4 | uint64(1)<<8 | uint64(1)<<60
	binary.LittleEndian.PutUint64(record[:8], occupied)
	// Piece nibbles follow occupied-square order: our king, our pawn, opponent king.
	record[8] = 5 | (0 << 4)
	record[9] = 13
	storedScore := int16(-321)
	binary.LittleEndian.PutUint16(record[24:26], uint16(storedScore))
	record[26] = 1
	record[27] = 4
	record[28] = 60 ^ 56

	position, score, err := decodeSelectionPosition(record[:])
	if err != nil {
		t.Fatal(err)
	}
	if score != -321 || position.SideToMove != ngnk4.White ||
		position.Board[ngnk4.WhiteKing] != uint64(1)<<4 ||
		position.Board[ngnk4.WhitePawn] != uint64(1)<<8 ||
		position.Board[ngnk4.BlackKing] != uint64(1)<<60 {
		t.Fatalf("decoded position=%+v score=%d", position, score)
	}

	record[28]++
	if _, _, err := decodeSelectionPosition(record[:]); err == nil {
		t.Fatal("accepted mismatched cached opponent king square")
	}
}

func TestChooseSelectionCandidateUsesIntegerMSEAndEarlyStop(t *testing.T) {
	metric := func(update uint64, loss float64, eligible bool) candidateMetric {
		return candidateMetric{Update: update, IntegerMeanMSE: loss, RawMeanMSE: 1 - loss, Eligible: eligible,
			Checkpoint: selectionFileReceipt{Path: string(rune(update))}}
	}
	metrics := []candidateMetric{
		metric(0, 0.50, false),
		metric(128, 0.20, true),
		metric(256, 0.19, true),
		metric(384, 0.190_005, true),
		metric(512, 0.190_004, true),
		metric(640, 0.190_003, true),
		metric(768, 0.190_002, true),
		metric(896, 0.10, true),
		metric(1024, 0.09, true),
	}
	selected, stop, err := chooseSelectionCandidate(metrics)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Update != 256 || !stop.Triggered || stop.StopUpdate != 768 {
		t.Fatalf("selected=%d stop=%+v", selected.Update, stop)
	}
}

func TestChooseSelectionCandidateBreaksExactTieAtEarlierUpdate(t *testing.T) {
	metrics := []candidateMetric{
		{Update: 128, IntegerMeanMSE: 0.1, Eligible: true},
		{Update: 256, IntegerMeanMSE: 0.1, Eligible: true},
	}
	selected, _, err := chooseSelectionCandidate(metrics)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Update != 128 {
		t.Fatalf("selected update %d, want 128", selected.Update)
	}
}

func TestPilotS1SelectionWaitsForMinimumAndEightMisses(t *testing.T) {
	mode, err := selectionModeFor("pilot-s1")
	if err != nil {
		t.Fatal(err)
	}
	if mode.totalUpdates != 16_384 || mode.checkpointUpdates != 512 || mode.minimumUpdate != 4_096 || mode.patience != 8 {
		t.Fatalf("mode=%+v", mode)
	}
	var metrics []candidateMetric
	for update := uint64(0); update <= 16_384; update += 512 {
		loss := 0.2
		if update == 512 {
			loss = 0.1
		}
		if update > 4_608 {
			loss = 0.05 // A later win must be excluded after the declared stop.
		}
		metrics = append(metrics, candidateMetric{Update: update, IntegerMeanMSE: loss, Eligible: update > 0})
	}
	selected, stop, err := chooseSelectionCandidateForMode(metrics, mode)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Update != 512 || !stop.Triggered || stop.StopUpdate != 4_608 {
		t.Fatalf("selected=%d stop=%+v", selected.Update, stop)
	}
}

func TestProbe5MSelectionModeUsesInheritedManifestAndLongerBudget(t *testing.T) {
	mode, err := selectionModeFor("probe5m")
	if err != nil {
		t.Fatal(err)
	}
	if mode.name != "probe5m" || mode.manifestMode != "probe5m" || mode.schema != selectionSchemaProbe5M ||
		mode.totalUpdates != 32_768 || mode.checkpointUpdates != 1_024 || mode.minimumUpdate != 8_192 || mode.patience != 8 {
		t.Fatalf("mode=%+v", mode)
	}
}

func TestMainM1SelectionHonorsMinimumBeforeEarlyStop(t *testing.T) {
	for _, item := range []struct{ name, schema string }{
		{"main-m1", selectionSchemaMainM1},
		{"main-m1-lr1", selectionSchemaMainM1LR1},
	} {
		t.Run(item.name, func(t *testing.T) {
			mode, err := selectionModeFor(item.name)
			if err != nil {
				t.Fatal(err)
			}
			if mode.manifestMode != "main" || mode.schema != item.schema ||
				mode.totalUpdates != 131_072 || mode.checkpointUpdates != 4_096 ||
				mode.minimumUpdate != 32_768 || mode.patience != 6 {
				t.Fatalf("mode=%+v", mode)
			}
			var metrics []candidateMetric
			for update := uint64(0); update <= 131_072; update += 4_096 {
				loss := 0.2
				if update == 4_096 {
					loss = 0.1
				}
				if update > 32_768 {
					loss = 0.05 // A later improvement cannot undo the declared stop.
				}
				metrics = append(metrics, candidateMetric{Update: update, IntegerMeanMSE: loss, Eligible: update > 0})
			}
			selected, stop, err := chooseSelectionCandidateForMode(metrics, mode)
			if err != nil {
				t.Fatal(err)
			}
			if selected.Update != 4_096 || !stop.Triggered || stop.StopUpdate != 32_768 {
				t.Fatalf("selected=%d stop=%+v", selected.Update, stop)
			}
		})
	}
}

func TestMainM1LR132KSelectionUsesShortSchedule(t *testing.T) {
	mode, err := selectionModeFor("main-m1-lr1-32k")
	if err != nil {
		t.Fatal(err)
	}
	if mode.manifestMode != "main" || mode.schema != selectionSchemaMainM1LR132K ||
		mode.totalUpdates != 32_768 || mode.checkpointUpdates != 4_096 ||
		mode.minimumUpdate != 8_192 || mode.patience != 8 {
		t.Fatalf("short-schedule selection mode=%+v", mode)
	}
}

func TestMainM1SelectionAcceptsOnlyMainTrainingManifest(t *testing.T) {
	directory := t.TempDir()
	parentPath := filepath.Join(directory, "pilot.json")
	parentBytes := []byte("{}\n")
	if err := os.WriteFile(parentPath, parentBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	parent := selectionReceiptForBytes(parentPath, parentBytes)
	validationPath := filepath.Join(directory, "validation.bf")
	validationBytes := make([]byte, pilotValidationRecords*32)
	if err := os.WriteFile(validationPath, validationBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	validation := selectionCorpus{
		Name: "validation", Split: "validation", Records: pilotValidationRecords, RecordBytes: 32,
		File: selectionReceiptForBytes(validationPath, validationBytes), Inherited: true,
	}
	train := selectionCorpus{
		Name: "train-main", Split: "train", Records: 20_000_000, RecordBytes: 32,
		File: selectionFileReceipt{Path: filepath.Join(directory, "train-main.bf"), Bytes: 20_000_000 * 32,
			SHA256: "0000000000000000000000000000000000000000000000000000000000000000"},
	}
	manifest := selectionManifest{
		Schema: finalizedSchema, ContractVersion: finalizedContract, State: "COMPLETE", Mode: "main",
		ParentPilot: &parent,
		Corpora:     []selectionCorpus{train, validation, {Name: "calibration"}, {Name: "reserved-test"}},
	}
	manifestPath := filepath.Join(directory, "manifest.json")
	writeManifest := func() {
		t.Helper()
		data, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeManifest()
	for _, name := range []string{"main-m1", "main-m1-lr1"} {
		mode, err := selectionModeFor(name)
		if err != nil {
			t.Fatal(err)
		}
		_, digest, _, loadedValidation, err := loadSelectionManifest(manifestPath, validationPath, mode)
		if err != nil || digest == ([sha256.Size]byte{}) || !bytes.Equal(loadedValidation, validationBytes) {
			t.Fatalf("%s load main manifest: sha=%x, validation=%d bytes, err=%v", name, digest, len(loadedValidation), err)
		}
	}
	manifest.Corpora[0].Records = 5_000_000
	writeManifest()
	for _, name := range []string{"main-m1", "main-m1-lr1"} {
		mode, err := selectionModeFor(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, _, err := loadSelectionManifest(manifestPath, validationPath, mode); err == nil {
			t.Fatalf("%s accepted five-million-record manifest as main training input", name)
		}
	}
}

func TestValidateOptimizerStateChecksLayoutAndFiniteValues(t *testing.T) {
	data := optimizerStateFixture(
		struct {
			name   string
			values []float32
		}{"l0w", []float32{1, -2}},
		struct {
			name   string
			values []float32
		}{"l0b", []float32{3}},
	)
	layout, err := validateOptimizerState(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout) != 2 || layout[0] != (optimizerTensorSpec{Name: "l0w", Count: 2}) ||
		layout[1] != (optimizerTensorSpec{Name: "l0b", Count: 1}) {
		t.Fatalf("layout=%+v", layout)
	}

	nonfinite := optimizerStateFixture(struct {
		name   string
		values []float32
	}{"l0w", []float32{float32(math.Inf(1))}})
	if _, err := validateOptimizerState(nonfinite); err == nil {
		t.Fatal("accepted nonfinite optimizer state")
	}

	duplicate := optimizerStateFixture(
		struct {
			name   string
			values []float32
		}{"l0w", []float32{1}},
		struct {
			name   string
			values []float32
		}{"l0w", []float32{2}},
	)
	if _, err := validateOptimizerState(duplicate); err == nil {
		t.Fatal("accepted duplicate optimizer tensor")
	}
}
