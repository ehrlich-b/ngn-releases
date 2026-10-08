package engine

import (
	"bytes"
	_ "embed"
)

// DefaultNetName identifies the embedded NGN-trained network in UCI output.
const DefaultNetName = "<embedded>"

// DefaultNetSHA256 is the digest of the embedded network file.
const DefaultNetSHA256 = "c6c127966d1cd9848b8bbe836ae74d5672874a3e74e973a488e4d92e350416f2"

//go:embed nets/ngn-h512-c6c12796.nnue
var defaultNetBytes []byte

// DefaultNetBytes returns a copy of the embedded network file.
func DefaultNetBytes() []byte { return append([]byte(nil), defaultNetBytes...) }

// LoadDefaultNet decodes the embedded network with the same validation as EvalFile.
func LoadDefaultNet() (*NGNN1Network, error) {
	return ReadNGNN1(bytes.NewReader(defaultNetBytes))
}
