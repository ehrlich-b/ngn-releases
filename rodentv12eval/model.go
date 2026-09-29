package rodentv12eval

const (
	InputBuckets       = 4
	InputSize          = 768
	TotalInputFeatures = InputBuckets * InputSize
	HiddenSize         = 768
	OutputBuckets      = 8
	FeaturePlaneCount  = 12

	fileSize    = 4744768
	payloadSize = 4744720
	outputScale = 206
	inputScale  = 255
	layerScale  = 64

	V12DefaultSHA256 = "c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053"
)

// Color is Rodent's side/perspective encoding.
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

// Position is the complete input to the V1.2 default evaluator. The release
// evaluator does not use castling, en-passant, or clock fields in its score.
type Position struct {
	Board      Board
	SideToMove Color
}

// Metadata binds a loader-validated exact V1.2 default model.
type Metadata struct {
	SHA256             string
	Bytes              int
	PayloadBytes       int
	TrailerBytes       int
	InputBuckets       int
	InputSize          int
	TotalInputFeatures int
	HiddenSize         int
	OutputBuckets      int
	Scale              int
}

// Model is an immutable exact Rodent V1.2 default network.
type Model struct {
	inputWeights  [TotalInputFeatures][HiddenSize]int16
	inputBiases   [HiddenSize]int16
	outputWeights [OutputBuckets][2][HiddenSize]int16
	outputBiases  [OutputBuckets]int16
	metadata      Metadata
	validated     bool
}

// Metadata returns the immutable identity of a strict-loader model.
func (model *Model) Metadata() (Metadata, error) {
	if err := model.validate(); err != nil {
		return Metadata{}, err
	}
	return model.metadata, nil
}
