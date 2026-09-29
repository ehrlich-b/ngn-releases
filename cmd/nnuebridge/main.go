package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/nnue"
)

const (
	bridgeVersion = "ngn-nnue-v1-bullet629ee5-v1"
	bulletCommit  = "629ee50000b2afb7b3337595401c830d3b1e0f42"
)

var defaultParityFENs = []string{
	"4k3/8/8/8/8/8/P7/4K3 w - - 0 1",
	"4k3/8/8/8/8/8/P7/4K3 b - - 0 1",
	"r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 0 1",
}

type fenList []string

func (f *fenList) String() string { return fmt.Sprint([]string(*f)) }
func (f *fenList) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type conversionResult struct {
	encoded      []byte
	model        *nnue.Model
	quantized    *quantizedNetwork
	raw          *floatNetwork
	sourceSHA256 [sha256.Size]byte
	sourceFormat sourceFormat
	sourcePath   string
	sourceBytes  int
}

type conversionReport struct {
	BridgeVersion string       `json:"bridge_version"`
	BulletCommit  string       `json:"bullet_commit"`
	Command       []string     `json:"command"`
	SourceFormat  sourceFormat `json:"source_format"`
	SourcePath    string       `json:"source_path"`
	OutputPath    string       `json:"output_path"`
	SourceSHA256  string       `json:"source_sha256"`
	SourceBytes   int          `json:"source_bytes"`
	PayloadSHA256 string       `json:"payload_sha256"`
	FileSHA256    string       `json:"file_sha256"`
	FileBytes     int          `json:"file_bytes"`
}

type parityRecord struct {
	FEN                       string   `json:"fen"`
	ReferenceQuantized        int64    `json:"reference_quantized_cp"`
	GoScalar                  int64    `json:"go_scalar_cp"`
	GoContext                 int64    `json:"go_context_cp"`
	ScalarMinusReference      int64    `json:"scalar_minus_reference_cp"`
	ContextMinusReference     int64    `json:"context_minus_reference_cp"`
	ReferenceFloat            *float32 `json:"reference_float_cp,omitempty"`
	FloatMinusInteger         *float32 `json:"float_minus_integer_cp,omitempty"`
	AbsoluteFloatIntegerDelta *float32 `json:"absolute_float_integer_delta_cp,omitempty"`
	Pass                      bool     `json:"pass"`
}

type parityReport struct {
	BridgeVersion                string         `json:"bridge_version"`
	BulletCommit                 string         `json:"bullet_commit"`
	Command                      []string       `json:"command"`
	SourceFormat                 sourceFormat   `json:"source_format"`
	SourcePath                   string         `json:"source_path"`
	SourceSHA256                 string         `json:"source_sha256"`
	SourceBytes                  int            `json:"source_bytes"`
	FENFilePath                  string         `json:"fen_file_path,omitempty"`
	FENFileSHA256                string         `json:"fen_file_sha256,omitempty"`
	NGNFileSHA                   string         `json:"ngn_file_sha256"`
	MaxAbsoluteIntegerGoDelta    int64          `json:"max_absolute_integer_go_delta_cp"`
	MaxAbsoluteFloatIntegerDelta *float32       `json:"max_absolute_float_integer_delta_cp,omitempty"`
	Pass                         bool           `json:"pass"`
	Records                      []parityRecord `json:"records"`
}

func usage(writer io.Writer) {
	fmt.Fprintln(writer, "usage:")
	fmt.Fprintln(writer, "  nnuebridge convert -format bullet-quantized|bullet-raw -in FILE -out FILE")
	fmt.Fprintln(writer, "  nnuebridge parity -format bullet-quantized|bullet-raw -in FILE [-fen FEN ...] [-fens FILE]")
}

func parseSourceFormat(value string) (sourceFormat, error) {
	format := sourceFormat(value)
	if format != formatBulletQuantized && format != formatBulletRaw {
		return "", fmt.Errorf("format must be %q or %q", formatBulletQuantized, formatBulletRaw)
	}
	return format, nil
}

func loadConversion(path string, format sourceFormat) (*conversionResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	hash := sha256.New()
	encoded, model, quantized, raw, err := convertBullet(io.TeeReader(file, hash), format)
	if err != nil {
		return nil, err
	}
	result := &conversionResult{
		encoded: encoded, model: model, quantized: quantized, raw: raw,
		sourceFormat: format, sourcePath: path,
	}
	if format == formatBulletQuantized {
		result.sourceBytes = bulletQuantizedSize
	} else {
		result.sourceBytes = bulletRawSize
	}
	copy(result.sourceSHA256[:], hash.Sum(nil))
	return result, nil
}

func writeAtomic(path string, data []byte) error {
	if path == "" {
		return errors.New("empty output path")
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".nnuebridge-*")
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
	return os.Rename(temporaryPath, path)
}

func printJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func runConvert(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("convert", flag.ContinueOnError)
	flags.SetOutput(stderr)
	formatValue := flags.String("format", "", "Bullet source format")
	input := flags.String("in", "", "input Bullet checkpoint transport")
	output := flags.String("out", "", "output strict NGN v1 model")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *input == "" || *output == "" {
		return errors.New("convert requires -format, -in, and -out")
	}
	format, err := parseSourceFormat(*formatValue)
	if err != nil {
		return err
	}
	result, err := loadConversion(*input, format)
	if err != nil {
		return err
	}
	if err := writeAtomic(*output, result.encoded); err != nil {
		return err
	}
	metadata := result.model.Metadata()
	return printJSON(stdout, conversionReport{
		BridgeVersion: bridgeVersion,
		BulletCommit:  bulletCommit,
		Command:       append([]string{"nnuebridge", "convert"}, args...),
		SourceFormat:  format,
		SourcePath:    *input,
		OutputPath:    *output,
		SourceSHA256:  hex.EncodeToString(result.sourceSHA256[:]),
		SourceBytes:   result.sourceBytes,
		PayloadSHA256: hex.EncodeToString(metadata.PayloadSHA256[:]),
		FileSHA256:    hex.EncodeToString(metadata.FileSHA256[:]),
		FileBytes:     len(result.encoded),
	})
}

func readFENFile(path string) ([]string, [sha256.Size]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, [sha256.Size]byte{}, err
	}
	defer file.Close()
	hash := sha256.New()
	var fens []string
	scanner := bufio.NewScanner(io.TeeReader(file, hash))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fens = append(fens, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, [sha256.Size]byte{}, err
	}
	if len(fens) == 0 {
		return nil, [sha256.Size]byte{}, errors.New("FEN file contains no positions")
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return fens, digest, nil
}

func absoluteInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func absoluteFloat32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

func runParity(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("parity", flag.ContinueOnError)
	flags.SetOutput(stderr)
	formatValue := flags.String("format", "", "Bullet source format")
	input := flags.String("in", "", "input Bullet checkpoint transport")
	fenPath := flags.String("fens", "", "newline-delimited FEN file")
	var fens fenList
	flags.Var(&fens, "fen", "position to evaluate; repeat for more positions")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *input == "" {
		return errors.New("parity requires -format and -in")
	}
	format, err := parseSourceFormat(*formatValue)
	if err != nil {
		return err
	}
	result, err := loadConversion(*input, format)
	if err != nil {
		return err
	}
	var fenFileDigest [sha256.Size]byte
	if *fenPath != "" {
		fromFile, digest, err := readFENFile(*fenPath)
		if err != nil {
			return fmt.Errorf("read FEN file: %w", err)
		}
		fenFileDigest = digest
		fens = append(fens, fromFile...)
	}
	if len(fens) == 0 {
		fens = append(fens, defaultParityFENs...)
	}
	metadata := result.model.Metadata()
	report := parityReport{
		BridgeVersion: bridgeVersion,
		BulletCommit:  bulletCommit,
		Command:       append([]string{"nnuebridge", "parity"}, args...),
		SourceFormat:  format,
		SourcePath:    *input,
		SourceSHA256:  hex.EncodeToString(result.sourceSHA256[:]),
		SourceBytes:   result.sourceBytes,
		FENFilePath:   *fenPath,
		NGNFileSHA:    hex.EncodeToString(metadata.FileSHA256[:]),
		Pass:          true,
		Records:       make([]parityRecord, 0, len(fens)),
	}
	if *fenPath != "" {
		report.FENFileSHA256 = hex.EncodeToString(fenFileDigest[:])
	}
	context, err := nnue.NewContext(result.model)
	if err != nil {
		return err
	}
	for _, fen := range fens {
		position, err := engine.ParseFEN(fen)
		if err != nil {
			return fmt.Errorf("parse FEN %q: %w", fen, err)
		}
		referencePosition, productionPosition := referenceFromEngine(position)
		referenceScore := evaluateQuantizedReference(result.quantized, referencePosition)
		scalarScore, err := result.model.Evaluate(productionPosition)
		if err != nil {
			return err
		}
		if err := context.Reset(productionPosition); err != nil {
			return err
		}
		contextScore, err := context.Evaluate()
		if err != nil {
			return err
		}
		scalarDelta := scalarScore - referenceScore
		contextDelta := contextScore - referenceScore
		record := parityRecord{
			FEN: fen, ReferenceQuantized: referenceScore, GoScalar: scalarScore, GoContext: contextScore,
			ScalarMinusReference: scalarDelta, ContextMinusReference: contextDelta,
			Pass: scalarDelta == 0 && contextDelta == 0,
		}
		for _, delta := range []int64{scalarDelta, contextDelta} {
			if absolute := absoluteInt64(delta); absolute > report.MaxAbsoluteIntegerGoDelta {
				report.MaxAbsoluteIntegerGoDelta = absolute
			}
		}
		if !record.Pass {
			report.Pass = false
		}
		if result.raw != nil {
			floatScore := evaluateFloatReference(result.raw, referencePosition)
			difference := floatScore - float32(referenceScore)
			absolute := absoluteFloat32(difference)
			record.ReferenceFloat = &floatScore
			record.FloatMinusInteger = &difference
			record.AbsoluteFloatIntegerDelta = &absolute
			if report.MaxAbsoluteFloatIntegerDelta == nil || absolute > *report.MaxAbsoluteFloatIntegerDelta {
				maximum := absolute
				report.MaxAbsoluteFloatIntegerDelta = &maximum
			}
		}
		report.Records = append(report.Records, record)
	}
	if err := printJSON(stdout, report); err != nil {
		return err
	}
	if !report.Pass {
		return errors.New("integer parity mismatch")
	}
	return nil
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stderr)
		return errors.New("missing command")
	}
	switch args[0] {
	case "convert":
		return runConvert(args[1:], stdout, stderr)
	case "parity":
		return runParity(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		usage(stdout)
		return nil
	default:
		usage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "nnuebridge:", err)
		os.Exit(2)
	}
}
