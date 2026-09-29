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
	"io"
	"os"
	"path/filepath"

	"github.com/ehrlich-b/ngn/nnue/ngnk4"
)

const (
	bridgeVersion  = "ngn-k4-768-v1-bullet629ee5-patchf7f5e0-v2"
	bulletCommit   = "629ee50000b2afb7b3337595401c830d3b1e0f42"
	bulletPatchSHA = "f7f5e0dd02695fe57ec58a630b63006da96cb1ec6b8fd4769a4e6308a8121e54"
	trailerBytes   = 48
	inputBytes     = ngnk4.PayloadSize + trailerBytes
)

var bulletTrailer = bytes.Repeat([]byte("bullet"), 8)

type conversionReceipt struct {
	BridgeVersion    string   `json:"bridge_version"`
	BulletCommit     string   `json:"bullet_commit"`
	Command          []string `json:"command"`
	SourcePath       string   `json:"source_path"`
	SourceBytes      int      `json:"source_bytes"`
	SourceSHA256     string   `json:"source_sha256"`
	ManifestPath     string   `json:"manifest_path"`
	ManifestBytes    int      `json:"manifest_bytes"`
	ManifestSHA256   string   `json:"manifest_sha256"`
	PayloadSHA256    string   `json:"payload_sha256"`
	OutputPath       string   `json:"output_path"`
	OutputBytes      int      `json:"output_bytes"`
	OutputSHA256     string   `json:"output_sha256"`
	PayloadRoundTrip bool     `json:"payload_round_trip"`
}

func usage(writer io.Writer) {
	fmt.Fprintln(writer, "usage: ngnk4bridge convert -in BULLET.nn -manifest run-manifest.json -out MODEL.nnue")
	fmt.Fprintln(writer, "       ngnk4bridge parity -raw DEPLOYED.raw -model MODEL.nnue -fens FENS -probe BULLET.tsv")
	fmt.Fprintln(writer, "       ngnk4bridge select [-mode pilot|pilot-s1|pilot-lr1|probe5m|probe5m-lr1|main-m1|main-m1-lr1|main-m1-lr1-32k|archive-a1-e1|archive-a1-e10|archive-a1-e20] -candidates DIR -manifest FINALIZED.json -validation VALIDATION.bf -out RECEIPT.json")
}

func readExactFile(path string, size int) ([]byte, error) {
	if path == "" {
		return nil, errors.New("empty input path")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(size+1)))
	if err != nil {
		return nil, err
	}
	if len(data) != size {
		return nil, fmt.Errorf("%q is %d bytes, want exactly %d", path, len(data), size)
	}
	return data, nil
}

func decodeBullet(data []byte) (*ngnk4.Tensors, []byte, error) {
	if len(data) != inputBytes {
		return nil, nil, fmt.Errorf("Bullet artifact is %d bytes, want %d", len(data), inputBytes)
	}
	if !bytes.Equal(data[ngnk4.PayloadSize:], bulletTrailer) {
		return nil, nil, errors.New("invalid Bullet trailer")
	}
	payload := data[:ngnk4.PayloadSize]
	tensors := new(ngnk4.Tensors)
	offset := 0
	readI16 := func() int16 {
		value := int16(binary.LittleEndian.Uint16(payload[offset : offset+2]))
		offset += 2
		return value
	}
	for input := range tensors.InputWeights {
		for hidden := range tensors.InputWeights[input] {
			tensors.InputWeights[input][hidden] = readI16()
		}
	}
	for hidden := range tensors.InputBiases {
		tensors.InputBiases[hidden] = readI16()
	}
	for bucket := range tensors.OutputWeights {
		for perspective := range tensors.OutputWeights[bucket] {
			for hidden := range tensors.OutputWeights[bucket][perspective] {
				tensors.OutputWeights[bucket][perspective][hidden] = readI16()
			}
		}
	}
	for bucket := range tensors.OutputBiases {
		tensors.OutputBiases[bucket] = readI16()
	}
	if offset != ngnk4.PayloadSize {
		panic("ngnk4bridge: internal Bullet payload size mismatch")
	}
	return tensors, payload, nil
}

func readManifest(path string) ([]byte, [sha256.Size]byte, error) {
	if path == "" {
		return nil, [sha256.Size]byte{}, errors.New("empty manifest path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, [sha256.Size]byte{}, err
	}
	if len(data) == 0 || len(data) > 16<<20 {
		return nil, [sha256.Size]byte{}, fmt.Errorf("manifest size %d outside 1..16777216", len(data))
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return nil, [sha256.Size]byte{}, fmt.Errorf("manifest must be one JSON object: %v", err)
	}
	return data, sha256.Sum256(data), nil
}

func writeAtomicNew(path string, data []byte) error {
	if path == "" {
		return errors.New("empty output path")
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".ngnk4bridge-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	// Link is the publish point: it is atomic, stays on the same filesystem
	// because the temporary file is created beside path, and fails rather than
	// replacing an existing immutable attempt artifact.
	if err := os.Link(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func samePath(first, second string) bool {
	a, errA := filepath.Abs(first)
	b, errB := filepath.Abs(second)
	return errA == nil && errB == nil && filepath.Clean(a) == filepath.Clean(b)
}

func runConvert(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("convert", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("in", "", "Bullet quantized artifact")
	manifestPath := flags.String("manifest", "", "frozen JSON training manifest")
	output := flags.String("out", "", "strict NGN K4 model")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *input == "" || *manifestPath == "" || *output == "" {
		return errors.New("convert requires -in, -manifest, and -out")
	}
	if samePath(*input, *output) || samePath(*manifestPath, *output) {
		return errors.New("output path must differ from input and manifest")
	}
	source, err := readExactFile(*input, inputBytes)
	if err != nil {
		return fmt.Errorf("read Bullet artifact: %w", err)
	}
	tensors, payload, err := decodeBullet(source)
	if err != nil {
		return err
	}
	manifest, manifestSHA, err := readManifest(*manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	encoded, err := ngnk4.Marshal(tensors, manifestSHA)
	if err != nil {
		return err
	}
	model, err := ngnk4.Load(bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("self-validate output: %w", err)
	}
	payloadRoundTrip := bytes.Equal(payload, encoded[ngnk4.HeaderSize:])
	if !payloadRoundTrip {
		return errors.New("decoded and re-encoded tensor payloads differ")
	}
	if err := writeAtomicNew(*output, encoded); err != nil {
		return err
	}
	metadata := model.Metadata()
	sourceSHA := sha256.Sum256(source)
	receipt := conversionReceipt{
		BridgeVersion:    bridgeVersion,
		BulletCommit:     bulletCommit,
		Command:          append([]string{"ngnk4bridge", "convert"}, args...),
		SourcePath:       *input,
		SourceBytes:      len(source),
		SourceSHA256:     hex.EncodeToString(sourceSHA[:]),
		ManifestPath:     *manifestPath,
		ManifestBytes:    len(manifest),
		ManifestSHA256:   hex.EncodeToString(manifestSHA[:]),
		PayloadSHA256:    hex.EncodeToString(metadata.PayloadSHA256[:]),
		OutputPath:       *output,
		OutputBytes:      len(encoded),
		OutputSHA256:     hex.EncodeToString(metadata.FileSHA256[:]),
		PayloadRoundTrip: payloadRoundTrip,
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(receipt)
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stderr)
		return errors.New("missing subcommand")
	}
	switch args[0] {
	case "convert":
		return runConvert(args[1:], stdout, stderr)
	case "parity":
		return runParity(args[1:], stdout, stderr)
	case "select":
		return runSelect(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		usage(stdout)
		return nil
	default:
		usage(stderr)
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "ngnk4bridge:", err)
		os.Exit(1)
	}
}
