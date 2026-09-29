package k4label

import (
	"reflect"
	"testing"

	"github.com/ehrlich-b/ngn/engine"
)

func parseKeyPosition(t *testing.T, fen string) *engine.Position {
	t.Helper()
	position, err := engine.ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	return position
}

func TestK4InputKeyMirroredArchitectureEquivalence(t *testing.T) {
	original := parseKeyPosition(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1")
	mirrored := parseKeyPosition(t, "rnbkqbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBKQBNR w - - 0 1")
	originalFeatures, originalHead, err := k4InputComponents(original)
	if err != nil {
		t.Fatal(err)
	}
	mirroredFeatures, mirroredHead, err := k4InputComponents(mirrored)
	if err != nil {
		t.Fatal(err)
	}
	if originalHead != 7 || mirroredHead != originalHead || !reflect.DeepEqual(originalFeatures, mirroredFeatures) {
		t.Fatalf("mirrored components differ: head %d/%d", originalHead, mirroredHead)
	}
	originalKey, err := K4InputSHA256(original)
	if err != nil {
		t.Fatal(err)
	}
	mirroredKey, err := K4InputSHA256(mirrored)
	if err != nil {
		t.Fatal(err)
	}
	if originalKey != mirroredKey || !validLowerHexSHA(originalKey) {
		t.Fatalf("mirrored keys %s / %s", originalKey, mirroredKey)
	}
	const golden = "754c73a16b5b3d57e57872163f8df31d7f867efe6b6dd6d3590edb2e2dae1b16"
	if originalKey != golden {
		t.Fatalf("K4 key = %s, want cross-implementation golden %s", originalKey, golden)
	}
}

func TestK4InputKeyPerspectiveOrderAndStateContract(t *testing.T) {
	white := parseKeyPosition(t, "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e3 7 1")
	black := parseKeyPosition(t, "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b - - 0 99")
	whiteNoState := parseKeyPosition(t, "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR w - - 0 99")
	whiteKey, err := K4InputSHA256(white)
	if err != nil {
		t.Fatal(err)
	}
	blackKey, err := K4InputSHA256(black)
	if err != nil {
		t.Fatal(err)
	}
	noStateKey, err := K4InputSHA256(whiteNoState)
	if err != nil {
		t.Fatal(err)
	}
	if whiteKey != noStateKey {
		t.Fatalf("non-model state changed K4 key: %s / %s", whiteKey, noStateKey)
	}
	if whiteKey == blackKey {
		t.Fatalf("side-to-move perspective order did not change key: %s", whiteKey)
	}
}
