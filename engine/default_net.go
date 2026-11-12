package engine

import (
	"bytes"
	_ "embed"
)

// DefaultNetName identifies the embedded NGN-trained network in UCI output.
const DefaultNetName = "<embedded>"

// DefaultNetSHA256 is the digest of the embedded network file.
const DefaultNetSHA256 = "2dcb6d936c0275927cb9e881f42e22558e618ad7e6fd998df6c4e5046d42c443"

//go:embed nets/ngn-h768-2dcb6d93.nnue
var defaultNetBytes []byte

// DefaultNetBytes returns a copy of the embedded network file.
func DefaultNetBytes() []byte { return append([]byte(nil), defaultNetBytes...) }

// LoadDefaultNet decodes the embedded network with the same validation as EvalFile.
func LoadDefaultNet() (*NGNN1Network, error) {
	return ReadNGNN1(bytes.NewReader(defaultNetBytes))
}
