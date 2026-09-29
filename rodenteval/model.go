package rodenteval

const (
	InputSize         = 768
	HiddenSize        = 512
	FeaturePlaneCount = 12

	fileSize    = 789568
	payloadSize = 789506
	outputScale = 192
	inputScale  = 255
	layerScale  = 64

	V11AnandSHA256 = "5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb"
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

// Position is the complete input to the V1.1 Anand evaluator. The release
// evaluator does not attenuate scores before the 100-halfmove draw boundary,
// so a halfmove clock is deliberately not part of this value.
type Position struct {
	Board      Board
	SideToMove Color
}

// Metadata binds a loader-validated exact Anand model.
type Metadata struct {
	SHA256       string
	Bytes        int
	PayloadBytes int
	TrailerBytes int
	InputSize    int
	HiddenSize   int
	Scale        int
}

// Model is an immutable exact Rodent V1.1 Anand network.
type Model struct {
	inputWeights  [InputSize][HiddenSize]int16
	inputBiases   [HiddenSize]int16
	outputWeights [2][HiddenSize]int16
	outputBias    int16
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
