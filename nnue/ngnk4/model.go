package ngnk4

import "crypto/sha256"

const (
	InputSize          = 768
	TotalInputFeatures = InputBuckets * InputSize
	OutputBuckets      = 8
	FeaturePlaneCount  = 12

	HeaderSize  = 256
	PayloadSize = 2 * (TotalInputFeatures*HiddenSize + HiddenSize + OutputBuckets*2*HiddenSize + OutputBuckets)
	FileSize    = HeaderSize + PayloadSize

	InputScale  = 255
	LayerScale  = 64
	OutputScale = 400
)

// k4LaneBlocks is the AVX2 loop count over 16 int16 lanes (see go_asm.h).
const k4LaneBlocks = HiddenSize / 16

const (
	ArchitectureID  = "ngn-" + bucketLabel + "-" + widthLabel + "-v1"
	FeatureSetID    = "halfka-" + bucketLabel + "-mirrored-relative-12x64-v1"
	QuantizationID  = "i16-qa255-qb64-screlu-wide-v1"
	ScoreContractID = "stm-400z-rule50-at-engine-v1"
)

// Color is both the piece-color and accumulator-perspective encoding.
type Color uint8

const (
	White Color = iota
	Black
)

// Board planes are ordered by color, then Pawn, Knight, Bishop, Rook,
// Queen, King. Squares use A1=0 through H8=63.
type Board [FeaturePlaneCount]uint64

const (
	WhitePawn = iota
	WhiteKnight
	WhiteBishop
	WhiteRook
	WhiteQueen
	WhiteKing
	BlackPawn
	BlackKnight
	BlackBishop
	BlackRook
	BlackQueen
	BlackKing
)

// Position is the complete network input. Castling, en-passant and clock state
// do not participate in the static model score.
type Position struct {
	Board      Board
	SideToMove Color
}

// Metadata is the comparable identity of one fully validated model file.
type Metadata struct {
	FileSHA256      [sha256.Size]byte
	ManifestSHA256  [sha256.Size]byte
	PayloadSHA256   [sha256.Size]byte
	FileBytes       int64
	HeaderBytes     uint16
	PayloadBytes    uint64
	Version         uint16
	ArchitectureID  string
	FeatureSetID    string
	QuantizationID  string
	ScoreContractID string
	InputBuckets    uint32
	InputSize       uint32
	HiddenSize      uint32
	OutputBuckets   uint32
	InputScale      uint32
	LayerScale      uint32
	OutputScale     uint32
	FastOutputSafe  [OutputBuckets]bool
}

// Model owns immutable logical-order tensors populated only by Load.
type Model struct {
	loaded        bool
	metadata      Metadata
	inputWeights  [TotalInputFeatures][HiddenSize]int16
	inputBiases   [HiddenSize]int16
	outputWeights [OutputBuckets][2][HiddenSize]int16
	outputBiases  [OutputBuckets]int16
}

// Metadata returns a value copy of the validated model identity.
func (model *Model) Metadata() Metadata {
	if model == nil || !model.loaded {
		return Metadata{}
	}
	return model.metadata
}
