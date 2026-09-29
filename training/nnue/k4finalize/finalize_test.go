package k4finalize

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ehrlich-b/ngn/training/nnue/k4label"
	"github.com/ehrlich-b/ngn/training/nnue/k4pack"
)

func testSHA(label string) string {
	sum := sha256.Sum256([]byte(label))
	return hex.EncodeToString(sum[:])
}

func writeLines(t *testing.T, path string, values ...any) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := bufio.NewWriter(file)
	for _, value := range values {
		if err := json.NewEncoder(writer).Encode(value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

type testLayout struct {
	sampler, labels, packed string
	expected                contract
}

func makeArtifacts(t *testing.T, layout testLayout, shard samplerShard, accepted []bool, selectionSHA string) samplerShard {
	t.Helper()
	positions := make([]k4label.InputPosition, len(accepted))
	inputValues := make([]any, 0, len(accepted)+1)
	inputValues = append(inputValues, k4label.InputHeader{
		Type: "header", Schema: k4label.InputSchema, ShardID: shard.ShardID, Split: shard.Split,
		SourceManifestSHA256: selectionSHA, RecordCount: len(accepted),
	})
	for index := range accepted {
		positions[index] = k4label.InputPosition{
			Type: "position", ID: testSHA(shard.ShardID + "-id-" + string(rune('a'+index))),
			K4InputSHA256: testSHA(shard.ShardID + "-key-" + string(rune('a'+index))),
		}
		inputValues = append(inputValues, positions[index])
	}
	writeLines(t, shard.File.Path, inputValues...)
	shard.File, _ = receipt(shard.File.Path)

	header := k4label.OutputHeader{
		Type: "header", Schema: k4label.OutputSchema, ContractVersion: k4label.ContractVersion,
		InputSHA256: shard.File.SHA256,
		Input: k4label.InputHeader{Type: "header", Schema: k4label.InputSchema, ShardID: shard.ShardID, Split: shard.Split,
			SourceManifestSHA256: selectionSHA, RecordCount: len(accepted)},
		Teacher: k4label.TeacherProvenance{Name: "Stockfish 18", ExecutablePath: "/teacher", ExecutableSHA256: TeacherExecutableSHA,
			SourceCommit: k4label.TeacherSourceCommit, BigNetworkSHA256: k4label.TeacherBigNetworkSHA,
			SmallNetworkSHA256: k4label.TeacherSmallNetSHA, Command: []string{"teacher"}, HandshakeOptionDigest: testSHA("handshake")},
		Search: k4label.FrozenSearchConfig(), ScoreContract: k4pack.ScoreContract,
	}
	labelValues := []any{header}
	identities := make([]acceptedIdentity, 0)
	rejected := 0
	for index, keep := range accepted {
		status := "accepted"
		reason := ""
		if !keep {
			status = "rejected"
			reason = "test-rejection"
			rejected++
		}
		labelValues = append(labelValues, k4label.LabelRecord{Type: "label", ID: positions[index].ID,
			K4InputSHA256: positions[index].K4InputSHA256, Split: shard.Split, Status: status, RejectionReason: reason})
		if keep {
			var identity acceptedIdentity
			id, _ := hex.DecodeString(positions[index].ID)
			key, _ := hex.DecodeString(positions[index].K4InputSHA256)
			copy(identity.id[:], id)
			copy(identity.key[:], key)
			identities = append(identities, identity)
		}
	}
	labelValues = append(labelValues, k4label.OutputFooter{Type: "footer", Schema: k4label.OutputSchema,
		Records: len(accepted), Accepted: len(identities), Rejected: rejected, Rejections: map[string]int{"test-rejection": rejected}})
	labelPath := filepath.Join(layout.labels, shard.ShardID+labelSuffix)
	writeLines(t, labelPath, labelValues...)
	labelReceipt, _ := receipt(labelPath)

	packedPath := filepath.Join(layout.packed, shard.ShardID+packedSuffix)
	packed := make([]byte, len(identities)*k4pack.RecordBytes)
	for index := range identities {
		packed[index*k4pack.RecordBytes+24] = byte(index + 1)
	}
	if err := os.WriteFile(packedPath, packed, 0o644); err != nil {
		t.Fatal(err)
	}
	packedReceipt, _ := receipt(packedPath)
	streamSHA, err := packerStreamDigest(packedPath, identities)
	if err != nil {
		t.Fatal(err)
	}
	pack := k4pack.Receipt{
		Schema: k4pack.ReceiptSchema, ContractVersion: k4pack.ContractVersion,
		InputBytes: labelReceipt.Bytes, InputSHA256: labelReceipt.SHA256, InputShardSHA256: shard.File.SHA256,
		ShardID: shard.ShardID, Split: shard.Split, OutputBytes: packedReceipt.Bytes, OutputSHA256: packedReceipt.SHA256,
		Records: len(accepted), Accepted: len(identities), Rejected: rejected, RecordBytes: k4pack.RecordBytes,
		ResultConstant: 1, ScorePerspective: "side-to-move", K4InputKeyContract: k4pack.K4InputKeyContract,
		AcceptedStreamSHA256: streamSHA, TeacherExecutableSHA256: TeacherExecutableSHA,
		TeacherSourceCommit: k4pack.TeacherSourceCommit, TeacherBigNetworkSHA256: k4pack.TeacherBigNetworkSHA,
		TeacherSmallNetworkSHA256: k4pack.TeacherSmallNetSHA,
	}
	writeJSON(t, filepath.Join(layout.packed, shard.ShardID+packReceiptSuffix), pack)
	return shard
}

func makeLayout(t *testing.T) testLayout {
	t.Helper()
	root := t.TempDir()
	inputs := filepath.Join(root, "inputs")
	labels := filepath.Join(root, "labels")
	packed := filepath.Join(root, "packed")
	for _, path := range []string{inputs, labels, packed} {
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	selectionPath := filepath.Join(root, "selection.json")
	if err := os.WriteFile(selectionPath, []byte("selection\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	selection, _ := receipt(selectionPath)
	expected := contract{pilotAccepted: 2, mainAccepted: 4, holdoutAccepted: 1, pilotCandidates: 3, mainCandidates: 6, holdoutCandidates: 2}
	specs := []struct {
		stage, split string
		accepted     []bool
	}{
		{"pilot", "train", []bool{true, false, true}},
		{"main-expansion", "train", []bool{true, true, false}},
		{"fixed", "validation", []bool{true, false}},
		{"fixed", "calibration", []bool{false, true}},
		{"fixed", "reserved-test", []bool{true, false}},
	}
	manifest := samplerManifest{Schema: SamplerSchema, ContractVersion: SamplerContract, State: "COMPLETE", Selection: selection,
		PilotCandidatePositions: 3, MainExpansionCandidatePositions: 3, TotalTrainCandidatePositions: 6,
		ValidationCandidatePositions: 2, CalibrationCandidatePositions: 2, ReservedTestCandidatePositions: 2,
		PilotAcceptedTarget: 2, MainAcceptedTarget: 4, HoldoutAcceptedTarget: 1}
	for _, spec := range specs {
		id := spec.stage + "-" + spec.split + "-000000"
		shard := samplerShard{Stage: spec.stage, Split: spec.split, ShardID: id, Records: len(spec.accepted),
			File: FileReceipt{Path: filepath.Join(inputs, id+".jsonl")}}
		manifest.Shards = append(manifest.Shards, makeArtifacts(t, testLayout{labels: labels, packed: packed}, shard, spec.accepted, selection.SHA256))
	}
	sampler := filepath.Join(root, "sampler.json")
	writeJSON(t, sampler, manifest)
	return testLayout{sampler: sampler, labels: labels, packed: packed, expected: expected}
}

func TestPackerStreamDigestUsesCanonicalHexLabelID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accepted.bf")
	record := make([]byte, k4pack.RecordBytes)
	record[24] = 0x34
	record[25] = 0x12
	if err := os.WriteFile(path, record, 0o644); err != nil {
		t.Fatal(err)
	}
	var identity acceptedIdentity
	for index := range identity.id {
		identity.id[index] = byte(index)
		identity.key[index] = byte(255 - index)
	}

	got, err := packerStreamDigest(path, []acceptedIdentity{identity})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.New()
	digest.Write([]byte("ngn-k4-pack-accepted-stream-v1\x00"))
	digest.Write([]byte(hex.EncodeToString(identity.id[:])))
	digest.Write(identity.key[:])
	var ordinal [8]byte
	binary.LittleEndian.PutUint64(ordinal[:], 0)
	digest.Write(ordinal[:])
	digest.Write(record[24:26])
	want := hex.EncodeToString(digest.Sum(nil))
	if got != want {
		t.Fatalf("accepted stream digest = %s, want %s", got, want)
	}
}

func TestPilotAndMainFreezeExactAcceptedSets(t *testing.T) {
	layout := makeLayout(t)
	pilotDir := filepath.Join(filepath.Dir(layout.sampler), "pilot")
	pilot, err := finalizePilot(layout.sampler, layout.labels, layout.packed, pilotDir, []string{"test-pilot"}, layout.expected)
	if err != nil {
		t.Fatal(err)
	}
	pilotTrain, err := findCorpus(pilot, "train-pilot")
	if err != nil || pilotTrain.Records != 2 || pilotTrain.File.Bytes != 2*k4pack.RecordBytes {
		t.Fatalf("pilot corpus = %+v err=%v", pilotTrain, err)
	}
	mainDir := filepath.Join(filepath.Dir(layout.sampler), "main")
	main, err := finalizeMain(layout.sampler, filepath.Join(pilotDir, "manifest.json"), layout.labels, layout.packed,
		mainDir, []string{"test-main"}, layout.expected)
	if err != nil {
		t.Fatal(err)
	}
	mainTrain, err := findCorpus(main, "train-main")
	if err != nil || mainTrain.Records != 4 || mainTrain.File.Bytes != 4*k4pack.RecordBytes {
		t.Fatalf("main corpus = %+v err=%v", mainTrain, err)
	}
	pilotBytes, _ := os.ReadFile(pilotTrain.File.Path)
	mainBytes, _ := os.ReadFile(mainTrain.File.Path)
	if len(mainBytes) < len(pilotBytes) || string(mainBytes[:len(pilotBytes)]) != string(pilotBytes) {
		t.Fatal("pilot corpus is not an exact main prefix")
	}
	probeDir := filepath.Join(filepath.Dir(layout.sampler), "probe5m")
	probe, err := finalizeExpanded(layout.sampler, filepath.Join(pilotDir, "manifest.json"), layout.labels, layout.packed,
		probeDir, []string{"test-probe5m"}, layout.expected, "probe5m", "train-probe5m", 3)
	if err != nil {
		t.Fatal(err)
	}
	probeTrain, err := findCorpus(probe, "train-probe5m")
	if err != nil || probe.Mode != "probe5m" || probeTrain.Records != 3 || len(probeTrain.Segments) != 2 ||
		probeTrain.Segments[0].Records != 2 || probeTrain.Segments[1].Records != 1 || len(probe.UsedShards) != 1 {
		t.Fatalf("probe corpus = %+v mode=%s used=%d err=%v", probeTrain, probe.Mode, len(probe.UsedShards), err)
	}
	probeBytes, err := os.ReadFile(probeTrain.File.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(probeBytes) != 3*k4pack.RecordBytes || string(probeBytes) != string(mainBytes[:len(probeBytes)]) {
		t.Fatal("probe corpus is not an exact main prefix")
	}
	for _, name := range []string{"validation", "calibration", "reserved-test"} {
		corpus, err := findCorpus(probe, name)
		if err != nil || !corpus.Inherited {
			t.Fatalf("probe did not inherit %s: %+v %v", name, corpus, err)
		}
	}
}
