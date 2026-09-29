package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ehrlich-b/ngn/nnue/ngnk4"
)

func writeBridgeFixture(t *testing.T) (string, string, []byte, [sha256.Size]byte) {
	t.Helper()
	tensors := new(ngnk4.Tensors)
	tensors.InputWeights[17][23] = -1234
	tensors.InputBiases[31] = 2345
	tensors.OutputWeights[6][1][47] = -3210
	tensors.OutputBiases[5] = 4567
	placeholder := sha256.Sum256([]byte("placeholder"))
	strict, err := ngnk4.Marshal(tensors, placeholder)
	if err != nil {
		t.Fatal(err)
	}
	bullet := append([]byte(nil), strict[ngnk4.HeaderSize:]...)
	bullet = append(bullet, bulletTrailer...)
	directory := t.TempDir()
	input := filepath.Join(directory, "checkpoint-512.nn")
	if err := os.WriteFile(input, bullet, 0o600); err != nil {
		t.Fatal(err)
	}
	manifestData := []byte("{\n  \"schema\": \"ngn-owned-nnue-run-v1\",\n  \"seed\": 26092001\n}\n")
	manifest := filepath.Join(directory, "run-manifest.json")
	if err := os.WriteFile(manifest, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	return input, manifest, bullet, sha256.Sum256(manifestData)
}

func TestConvertStrictRoundTripAndReceipt(t *testing.T) {
	input, manifest, bullet, manifestSHA := writeBridgeFixture(t)
	output := filepath.Join(t.TempDir(), "candidate.nnue")
	var stdout, stderr bytes.Buffer
	args := []string{"-in", input, "-manifest", manifest, "-out", output}
	if err := runConvert(args, &stdout, &stderr); err != nil {
		t.Fatalf("runConvert: %v\nstderr=%s", err, stderr.String())
	}
	encoded, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded[ngnk4.HeaderSize:], bullet[:ngnk4.PayloadSize]) {
		t.Fatal("strict output payload differs from Bullet payload")
	}
	model, err := ngnk4.Load(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if model.Metadata().ManifestSHA256 != manifestSHA {
		t.Fatal("strict model does not bind exact manifest bytes")
	}
	var receipt conversionReceipt
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	metadata := model.Metadata()
	if receipt.BridgeVersion != bridgeVersion || receipt.BulletCommit != bulletCommit ||
		receipt.SourceBytes != inputBytes || receipt.OutputBytes != ngnk4.FileSize ||
		!receipt.PayloadRoundTrip || receipt.ManifestSHA256 != hex.EncodeToString(manifestSHA[:]) ||
		receipt.OutputSHA256 != hex.EncodeToString(metadata.FileSHA256[:]) {
		t.Fatalf("receipt = %+v", receipt)
	}
	if err := runConvert(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("conversion overwrote an existing model")
	}
	unchanged, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(unchanged, encoded) {
		t.Fatalf("failed replacement changed model: err=%v", err)
	}
}

func TestConvertRejectsMalformedInputsWithoutOutput(t *testing.T) {
	input, manifest, bullet, _ := writeBridgeFixture(t)
	tests := []struct {
		name    string
		prepare func(string) (string, string, string)
	}{
		{"truncated", func(directory string) (string, string, string) {
			path := filepath.Join(directory, "short.nn")
			if err := os.WriteFile(path, bullet[:len(bullet)-1], 0o600); err != nil {
				t.Fatal(err)
			}
			return path, manifest, filepath.Join(directory, "out.nnue")
		}},
		{"trailer", func(directory string) (string, string, string) {
			bad := append([]byte(nil), bullet...)
			bad[len(bad)-1] ^= 1
			path := filepath.Join(directory, "bad-trailer.nn")
			if err := os.WriteFile(path, bad, 0o600); err != nil {
				t.Fatal(err)
			}
			return path, manifest, filepath.Join(directory, "out.nnue")
		}},
		{"manifest", func(directory string) (string, string, string) {
			path := filepath.Join(directory, "bad.json")
			if err := os.WriteFile(path, []byte("[]"), 0o600); err != nil {
				t.Fatal(err)
			}
			return input, path, filepath.Join(directory, "out.nnue")
		}},
		{"same output", func(directory string) (string, string, string) { return input, manifest, input }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			fixture, manifestPath, output := test.prepare(directory)
			err := runConvert([]string{"-in", fixture, "-manifest", manifestPath, "-out", output}, &bytes.Buffer{}, &bytes.Buffer{})
			if err == nil {
				t.Fatal("malformed conversion succeeded")
			}
			if output != input {
				if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
					t.Fatalf("failed conversion published output: %v", statErr)
				}
			}
		})
	}
}

func TestRunUsageAndRequiredFlags(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"convert"}, {"convert", "-in", "x"}} {
		if err := run(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatalf("run(%q) succeeded", args)
		}
	}
	var stdout bytes.Buffer
	if err := run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil || !strings.Contains(stdout.String(), "ngnk4bridge convert") {
		t.Fatalf("help = %v, %q", err, stdout.String())
	}
}

func TestParityChecksTensorAndBulletGoEvaluationContracts(t *testing.T) {
	directory := t.TempDir()
	rawPath := filepath.Join(directory, "deployed.raw")
	raw := make([]byte, deployedRawBytes)
	if err := os.WriteFile(rawPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	manifestSHA := sha256.Sum256([]byte("parity-manifest"))
	modelBytes, err := ngnk4.Marshal(new(ngnk4.Tensors), manifestSHA)
	if err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(directory, "model.nnue")
	if err := os.WriteFile(modelPath, modelBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	fens := []string{
		"4k3/8/8/8/8/8/8/4K3 w - - 0 1",
		"r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1",
	}
	fenPath := filepath.Join(directory, "parity.fens")
	if err := os.WriteFile(fenPath, []byte(strings.Join(fens, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	probePath := filepath.Join(directory, "probe.tsv")
	probe := "NGN_K4_PROBE\t0\t0.000000000\t" + fens[0] + "\n" +
		"NGN_K4_PROBE\t1\t0.000000000\t" + fens[1] + "\n" +
		"NGN_K4_PROBE_PASS bullet_commit=" + bulletCommit + " patch=" + bulletPatchSHA + " positions=2\n"
	if err := os.WriteFile(probePath, []byte(probe), 0o600); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(directory, "parity.json")
	args := []string{"-raw", rawPath, "-model", modelPath, "-fens", fenPath, "-probe", probePath, "-out", receiptPath}
	var stdout bytes.Buffer
	if err := runParity(args, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var report k4ParityReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Pass || !report.TensorQuantizationPass || report.BulletPatchSHA256 != bulletPatchSHA || len(report.Records) != len(fens) ||
		report.Records[0].OutputBucket != 0 || report.Records[1].OutputBucket != 1 {
		t.Fatalf("parity report = %+v", report)
	}
	written, err := os.ReadFile(receiptPath)
	if err != nil || !bytes.Equal(written, stdout.Bytes()) {
		t.Fatalf("receipt differs from stdout: err=%v", err)
	}
	if err := runParity(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("parity overwrote an existing receipt")
	}

	badRaw := append([]byte(nil), raw...)
	binary.LittleEndian.PutUint32(badRaw[:4], math.Float32bits(1))
	badRawPath := filepath.Join(directory, "bad.raw")
	if err := os.WriteFile(badRawPath, badRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	badArgs := []string{"-raw", badRawPath, "-model", modelPath, "-fens", fenPath, "-probe", probePath}
	if err := runParity(badArgs, &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "tensor mismatch") {
		t.Fatalf("mutated raw parity error = %v", err)
	}
}
