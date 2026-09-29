// Package k4finalize freezes exact accepted-position corpora from the ordered
// sampler, teacher-label and independent packer artifacts.
package k4finalize

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ehrlich-b/ngn/training/nnue/k4label"
	"github.com/ehrlich-b/ngn/training/nnue/k4pack"
)

const (
	SamplerSchema        = "ngn-k4-sampler-output-v2"
	SamplerContract      = "ngn-k4-sampler-v2"
	ManifestSchema       = "ngn-k4-finalized-corpus-v1"
	IdentityContract     = "sha256(ngn-k4-final-segments-v1\\0 || segment_name || count:u64le || sha256(ngn-k4-final-accepted-v1\\0 || id:32 || k4_key:32 || ordinal:u64le) for each ordered segment)"
	TeacherExecutableSHA = "174270346ae9ed600713d165fa745dfa87fc084e44907c8084767f2b905e86d3"
	labelSuffix          = ".labels.jsonl"
	packedSuffix         = ".bf"
	packReceiptSuffix    = ".pack.json"
	maximumLineBytes     = 1 << 20
)

var ErrContract = errors.New("NGN K4 finalize contract violation")

type FileReceipt struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type samplerShard struct {
	Stage   string      `json:"stage"`
	Split   string      `json:"split"`
	ShardID string      `json:"shard_id"`
	Records int         `json:"records"`
	File    FileReceipt `json:"file"`
}

type samplerManifest struct {
	Schema                          string         `json:"schema"`
	ContractVersion                 string         `json:"contract_version"`
	State                           string         `json:"state"`
	Selection                       FileReceipt    `json:"selection"`
	Shards                          []samplerShard `json:"shards"`
	PilotCandidatePositions         uint64         `json:"pilot_candidate_positions"`
	MainExpansionCandidatePositions uint64         `json:"main_expansion_candidate_positions"`
	TotalTrainCandidatePositions    uint64         `json:"total_train_candidate_positions"`
	ValidationCandidatePositions    uint64         `json:"validation_candidate_positions"`
	CalibrationCandidatePositions   uint64         `json:"calibration_candidate_positions"`
	ReservedTestCandidatePositions  uint64         `json:"reserved_test_candidate_positions"`
	PilotAcceptedTarget             uint64         `json:"pilot_accepted_target"`
	MainAcceptedTarget              uint64         `json:"main_accepted_target"`
	HoldoutAcceptedTarget           uint64         `json:"holdout_accepted_target"`
}

type contract struct {
	pilotAccepted, mainAccepted, holdoutAccepted       uint64
	pilotCandidates, mainCandidates, holdoutCandidates uint64
}

func productionContract() contract {
	return contract{
		pilotAccepted: 1_000_000, mainAccepted: 20_000_000, holdoutAccepted: 100_000,
		pilotCandidates: 1_250_000, mainCandidates: 25_000_000, holdoutCandidates: 125_000,
	}
}

type AcceptedSegment struct {
	Name                   string `json:"name"`
	Records                uint64 `json:"records"`
	AcceptedIdentitySHA256 string `json:"accepted_identity_sha256"`
}

type CorpusReceipt struct {
	Name                   string            `json:"name"`
	Split                  string            `json:"split"`
	Records                uint64            `json:"records"`
	RecordBytes            int               `json:"record_bytes"`
	File                   FileReceipt       `json:"file"`
	Segments               []AcceptedSegment `json:"segments"`
	AcceptedIdentitySHA256 string            `json:"accepted_identity_sha256"`
	IdentityContract       string            `json:"identity_contract"`
	Inherited              bool              `json:"inherited,omitempty"`
}

type UsedShard struct {
	Stage                   string      `json:"stage"`
	Split                   string      `json:"split"`
	ShardID                 string      `json:"shard_id"`
	CandidateRecords        int         `json:"candidate_records"`
	AcceptedAvailable       int         `json:"accepted_available"`
	AcceptedUsed            int         `json:"accepted_used"`
	SamplerInput            FileReceipt `json:"sampler_input"`
	Labels                  FileReceipt `json:"labels"`
	PackReceipt             FileReceipt `json:"pack_receipt"`
	Packed                  FileReceipt `json:"packed"`
	PackerAcceptedStreamSHA string      `json:"packer_accepted_stream_sha256"`
}

type Manifest struct {
	Schema          string          `json:"schema"`
	ContractVersion string          `json:"contract_version"`
	State           string          `json:"state"`
	Mode            string          `json:"mode"`
	Command         []string        `json:"command"`
	Sampler         FileReceipt     `json:"sampler"`
	ParentPilot     *FileReceipt    `json:"parent_pilot,omitempty"`
	Corpora         []CorpusReceipt `json:"corpora"`
	UsedShards      []UsedShard     `json:"used_shards"`
}

type acceptedIdentity struct {
	id  [32]byte
	key [32]byte
}

type artifact struct {
	identities  []acceptedIdentity
	labels      FileReceipt
	packReceipt FileReceipt
	packed      FileReceipt
	packer      k4pack.Receipt
}

func strictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func loadJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := strictJSON(data, target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func validSHA(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func receipt(path string) (FileReceipt, error) {
	file, err := os.Open(path)
	if err != nil {
		return FileReceipt{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return FileReceipt{}, err
	}
	if !info.Mode().IsRegular() {
		return FileReceipt{}, fmt.Errorf("%w: non-regular file %s", ErrContract, path)
	}
	digest := sha256.New()
	bytes, err := io.Copy(digest, file)
	if err != nil {
		return FileReceipt{}, err
	}
	return FileReceipt{Path: path, Bytes: bytes, SHA256: hex.EncodeToString(digest.Sum(nil))}, nil
}

func verifyReceipt(want FileReceipt) error {
	if want.Path == "" || want.Bytes < 0 || !validSHA(want.SHA256) {
		return fmt.Errorf("%w: malformed file receipt", ErrContract)
	}
	got, err := receipt(want.Path)
	if err != nil {
		return err
	}
	if got.Bytes != want.Bytes || got.SHA256 != want.SHA256 {
		return fmt.Errorf("%w: file receipt mismatch %s", ErrContract, want.Path)
	}
	return nil
}

func seriesKey(stage, split string) string { return stage + "\x00" + split }

func loadSampler(path string, expected contract) (samplerManifest, FileReceipt, map[string][]samplerShard, error) {
	var manifest samplerManifest
	if err := loadJSON(path, &manifest); err != nil {
		return manifest, FileReceipt{}, nil, err
	}
	manifestReceipt, err := receipt(path)
	if err != nil {
		return manifest, FileReceipt{}, nil, err
	}
	if manifest.Schema != SamplerSchema || manifest.ContractVersion != SamplerContract || manifest.State != "COMPLETE" ||
		manifest.PilotAcceptedTarget != expected.pilotAccepted || manifest.MainAcceptedTarget != expected.mainAccepted ||
		manifest.HoldoutAcceptedTarget != expected.holdoutAccepted || manifest.PilotCandidatePositions < expected.pilotCandidates ||
		manifest.TotalTrainCandidatePositions < expected.mainCandidates || manifest.ValidationCandidatePositions < expected.holdoutCandidates ||
		manifest.CalibrationCandidatePositions < expected.holdoutCandidates || manifest.ReservedTestCandidatePositions < expected.holdoutCandidates ||
		manifest.PilotCandidatePositions+manifest.MainExpansionCandidatePositions != manifest.TotalTrainCandidatePositions {
		return manifest, FileReceipt{}, nil, fmt.Errorf("%w: sampler manifest contract mismatch", ErrContract)
	}
	if err := verifyReceipt(manifest.Selection); err != nil {
		return manifest, FileReceipt{}, nil, err
	}
	allowed := map[string]uint64{
		seriesKey("pilot", "train"):          manifest.PilotCandidatePositions,
		seriesKey("main-expansion", "train"): manifest.MainExpansionCandidatePositions,
		seriesKey("fixed", "validation"):     manifest.ValidationCandidatePositions,
		seriesKey("fixed", "calibration"):    manifest.CalibrationCandidatePositions,
		seriesKey("fixed", "reserved-test"):  manifest.ReservedTestCandidatePositions,
	}
	groups := make(map[string][]samplerShard)
	seen := make(map[string]struct{})
	for _, shard := range manifest.Shards {
		key := seriesKey(shard.Stage, shard.Split)
		if _, ok := allowed[key]; !ok || shard.Records < 1 || shard.Records > k4label.MaximumShardRecords {
			return manifest, FileReceipt{}, nil, fmt.Errorf("%w: invalid sampler shard series %s/%s", ErrContract, shard.Stage, shard.Split)
		}
		if _, duplicate := seen[shard.ShardID]; duplicate {
			return manifest, FileReceipt{}, nil, fmt.Errorf("%w: duplicate shard %s", ErrContract, shard.ShardID)
		}
		seen[shard.ShardID] = struct{}{}
		wantID := fmt.Sprintf("%s-%s-%06d", shard.Stage, shard.Split, len(groups[key]))
		if shard.ShardID != wantID {
			return manifest, FileReceipt{}, nil, fmt.Errorf("%w: shard order %s expected %s", ErrContract, shard.ShardID, wantID)
		}
		if err := verifyReceipt(shard.File); err != nil {
			return manifest, FileReceipt{}, nil, err
		}
		groups[key] = append(groups[key], shard)
	}
	for key, want := range allowed {
		var got uint64
		for _, shard := range groups[key] {
			got += uint64(shard.Records)
		}
		if got != want {
			return manifest, FileReceipt{}, nil, fmt.Errorf("%w: sampler series %q records=%d expected=%d", ErrContract, key, got, want)
		}
	}
	return manifest, manifestReceipt, groups, nil
}

type envelope struct {
	Type string `json:"type"`
}

func parseSamplerIdentities(shard samplerShard, selectionSHA string) ([]acceptedIdentity, error) {
	file, err := os.Open(shard.File.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maximumLineBytes)
	line := 0
	identities := make([]acceptedIdentity, 0, shard.Records)
	for scanner.Scan() {
		line++
		data := append([]byte(nil), scanner.Bytes()...)
		if line == 1 {
			var header k4label.InputHeader
			if err := strictJSON(data, &header); err != nil {
				return nil, fmt.Errorf("%w: sampler header %s: %v", ErrContract, shard.File.Path, err)
			}
			if header.Type != "header" || header.Schema != k4label.InputSchema || header.ShardID != shard.ShardID ||
				header.Split != shard.Split || header.SourceManifestSHA256 != selectionSHA || header.RecordCount != shard.Records {
				return nil, fmt.Errorf("%w: sampler header contract %s", ErrContract, shard.File.Path)
			}
			continue
		}
		var position k4label.InputPosition
		if err := strictJSON(data, &position); err != nil {
			return nil, err
		}
		idBytes, idErr := hex.DecodeString(position.ID)
		keyBytes, keyErr := hex.DecodeString(position.K4InputSHA256)
		if position.Type != "position" || idErr != nil || keyErr != nil || len(idBytes) != 32 || len(keyBytes) != 32 {
			return nil, fmt.Errorf("%w: sampler position identity %s", ErrContract, shard.File.Path)
		}
		var identity acceptedIdentity
		copy(identity.id[:], idBytes)
		copy(identity.key[:], keyBytes)
		identities = append(identities, identity)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if line != shard.Records+1 || len(identities) != shard.Records {
		return nil, fmt.Errorf("%w: sampler position count %s", ErrContract, shard.File.Path)
	}
	return identities, nil
}

func parseLabels(path string, shard samplerShard, selectionSHA string) ([]acceptedIdentity, int, error) {
	expected, err := parseSamplerIdentities(shard, selectionSHA)
	if err != nil {
		return nil, 0, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maximumLineBytes)
	line := 0
	accepted := make([]acceptedIdentity, 0, shard.Records)
	rejected := 0
	records := 0
	var footer k4label.OutputFooter
	for scanner.Scan() {
		line++
		data := append([]byte(nil), scanner.Bytes()...)
		if line == 1 {
			var header k4label.OutputHeader
			if err := strictJSON(data, &header); err != nil {
				return nil, 0, fmt.Errorf("%w: label header %s: %v", ErrContract, path, err)
			}
			if header.Type != "header" || header.Schema != k4label.OutputSchema || header.ContractVersion != k4label.ContractVersion ||
				header.InputSHA256 != shard.File.SHA256 || header.Input.ShardID != shard.ShardID || header.Input.Split != shard.Split ||
				header.Input.SourceManifestSHA256 != selectionSHA || header.Input.RecordCount != shard.Records ||
				header.ScoreContract != k4pack.ScoreContract || header.Teacher.Name != "Stockfish 18" ||
				header.Teacher.ExecutableSHA256 != TeacherExecutableSHA || header.Teacher.SourceCommit != k4label.TeacherSourceCommit ||
				header.Teacher.BigNetworkSHA256 != k4label.TeacherBigNetworkSHA || header.Teacher.SmallNetworkSHA256 != k4label.TeacherSmallNetSHA ||
				!validSHA(header.Teacher.HandshakeOptionDigest) || header.Search != k4label.FrozenSearchConfig() {
				return nil, 0, fmt.Errorf("%w: label header contract %s", ErrContract, path)
			}
			continue
		}
		var kind envelope
		if err := json.Unmarshal(data, &kind); err != nil {
			return nil, 0, err
		}
		switch kind.Type {
		case "label":
			if footer.Type != "" {
				return nil, 0, fmt.Errorf("%w: label after footer", ErrContract)
			}
			var record k4label.LabelRecord
			if err := strictJSON(data, &record); err != nil {
				return nil, 0, err
			}
			idBytes, idErr := hex.DecodeString(record.ID)
			keyBytes, keyErr := hex.DecodeString(record.K4InputSHA256)
			if idErr != nil || keyErr != nil || len(idBytes) != 32 || len(keyBytes) != 32 || record.Split != shard.Split || records >= len(expected) {
				return nil, 0, fmt.Errorf("%w: invalid label identity in %s", ErrContract, path)
			}
			var identity acceptedIdentity
			copy(identity.id[:], idBytes)
			copy(identity.key[:], keyBytes)
			if identity != expected[records] {
				return nil, 0, fmt.Errorf("%w: label/sampler identity mismatch at record %d in %s", ErrContract, records, path)
			}
			records++
			switch record.Status {
			case "accepted":
				accepted = append(accepted, identity)
			case "rejected":
				rejected++
			default:
				return nil, 0, fmt.Errorf("%w: invalid label status", ErrContract)
			}
		case "footer":
			if footer.Type != "" || strictJSON(data, &footer) != nil {
				return nil, 0, fmt.Errorf("%w: malformed/duplicate footer", ErrContract)
			}
		default:
			return nil, 0, fmt.Errorf("%w: unexpected label line type %q", ErrContract, kind.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}
	if line < 2 || records != shard.Records || footer.Type != "footer" || footer.Schema != k4label.OutputSchema || footer.Records != shard.Records ||
		footer.Accepted != len(accepted) || footer.Rejected != rejected || footer.Records != len(accepted)+rejected {
		return nil, 0, fmt.Errorf("%w: label footer/count mismatch %s", ErrContract, path)
	}
	return accepted, rejected, nil
}

func packerStreamDigest(path string, identities []acceptedIdentity) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	digest.Write([]byte("ngn-k4-pack-accepted-stream-v1\x00"))
	var record [k4pack.RecordBytes]byte
	for ordinal, identity := range identities {
		if _, err := io.ReadFull(file, record[:]); err != nil {
			return "", err
		}
		// k4pack hashes the canonical lowercase hex label ID exactly as it
		// appears in the label stream, while the input key is hashed as its
		// decoded 32-byte value. Re-encode the already-validated ID here so
		// independent finalization reproduces the packer's receipt contract.
		digest.Write([]byte(hex.EncodeToString(identity.id[:])))
		digest.Write(identity.key[:])
		var word [8]byte
		binary.LittleEndian.PutUint64(word[:], uint64(ordinal))
		digest.Write(word[:])
		digest.Write(record[24:26])
	}
	var tail [1]byte
	if count, err := file.Read(tail[:]); err != io.EOF || count != 0 {
		return "", fmt.Errorf("%w: packed BF has an unexpected tail", ErrContract)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func loadArtifact(shard samplerShard, selectionSHA, labelsDir, packedDir string) (artifact, error) {
	labelPath := filepath.Join(labelsDir, shard.ShardID+labelSuffix)
	packedPath := filepath.Join(packedDir, shard.ShardID+packedSuffix)
	packReceiptPath := filepath.Join(packedDir, shard.ShardID+packReceiptSuffix)
	identities, rejected, err := parseLabels(labelPath, shard, selectionSHA)
	if err != nil {
		return artifact{}, err
	}
	labelsReceipt, err := receipt(labelPath)
	if err != nil {
		return artifact{}, err
	}
	var pack k4pack.Receipt
	if err := loadJSON(packReceiptPath, &pack); err != nil {
		return artifact{}, err
	}
	packFileReceipt, err := receipt(packReceiptPath)
	if err != nil {
		return artifact{}, err
	}
	packedReceipt, err := receipt(packedPath)
	if err != nil {
		return artifact{}, err
	}
	streamSHA, err := packerStreamDigest(packedPath, identities)
	if err != nil {
		return artifact{}, err
	}
	if pack.Schema != k4pack.ReceiptSchema || pack.ContractVersion != k4pack.ContractVersion ||
		pack.InputSHA256 != labelsReceipt.SHA256 || pack.InputBytes != labelsReceipt.Bytes || pack.InputShardSHA256 != shard.File.SHA256 ||
		pack.ShardID != shard.ShardID || pack.Split != shard.Split || pack.Records != shard.Records ||
		pack.Accepted != len(identities) || pack.Rejected != rejected || pack.RecordBytes != k4pack.RecordBytes ||
		pack.OutputBytes != int64(len(identities)*k4pack.RecordBytes) || pack.OutputBytes != packedReceipt.Bytes || pack.OutputSHA256 != packedReceipt.SHA256 ||
		pack.ResultConstant != 1 || pack.ScorePerspective != "side-to-move" || pack.K4InputKeyContract != k4pack.K4InputKeyContract ||
		pack.AcceptedStreamSHA256 != streamSHA ||
		pack.TeacherExecutableSHA256 != TeacherExecutableSHA || pack.TeacherSourceCommit != k4pack.TeacherSourceCommit ||
		pack.TeacherBigNetworkSHA256 != k4pack.TeacherBigNetworkSHA || pack.TeacherSmallNetworkSHA256 != k4pack.TeacherSmallNetSHA {
		return artifact{}, fmt.Errorf("%w: pack receipt mismatch %s", ErrContract, packReceiptPath)
	}
	return artifact{identities: identities, labels: labelsReceipt, packReceipt: packFileReceipt, packed: packedReceipt, packer: pack}, nil
}

func identityDigest(name string, identities []acceptedIdentity) string {
	digest := sha256.New()
	digest.Write([]byte("ngn-k4-final-accepted-v1\x00"))
	for ordinal, identity := range identities {
		digest.Write(identity.id[:])
		digest.Write(identity.key[:])
		var word [8]byte
		binary.LittleEndian.PutUint64(word[:], uint64(ordinal))
		digest.Write(word[:])
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func compositeIdentity(segments []AcceptedSegment) string {
	digest := sha256.New()
	digest.Write([]byte("ngn-k4-final-segments-v1\x00"))
	for _, segment := range segments {
		digest.Write([]byte(segment.Name))
		digest.Write([]byte{0})
		var count [8]byte
		binary.LittleEndian.PutUint64(count[:], segment.Records)
		digest.Write(count[:])
		decoded, _ := hex.DecodeString(segment.AcceptedIdentitySHA256)
		digest.Write(decoded)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

type corpusWriter struct {
	file   *os.File
	digest hash.Hash
	writer io.Writer
}

func newCorpusWriter(path string) (*corpusWriter, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	digest := sha256.New()
	return &corpusWriter{file: file, digest: digest, writer: io.MultiWriter(file, digest)}, nil
}

func (writer *corpusWriter) appendPrefix(path string, records int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	bytes := int64(records * k4pack.RecordBytes)
	written, err := io.CopyN(writer.writer, file, bytes)
	if err != nil || written != bytes {
		return fmt.Errorf("%w: copy packed prefix %s bytes=%d/%d: %v", ErrContract, path, written, bytes, err)
	}
	return nil
}

func (writer *corpusWriter) close(path string, records uint64) (FileReceipt, error) {
	if err := writer.file.Sync(); err != nil {
		_ = writer.file.Close()
		return FileReceipt{}, err
	}
	if err := writer.file.Close(); err != nil {
		return FileReceipt{}, err
	}
	wantBytes := int64(records * k4pack.RecordBytes)
	info, err := os.Stat(path)
	if err != nil || info.Size() != wantBytes {
		return FileReceipt{}, fmt.Errorf("%w: corpus size mismatch %s", ErrContract, path)
	}
	return FileReceipt{Path: path, Bytes: wantBytes, SHA256: hex.EncodeToString(writer.digest.Sum(nil))}, nil
}

func appendSeries(writer *corpusWriter, stage, split string, shards []samplerShard, target uint64, selectionSHA, labelsDir, packedDir string) (AcceptedSegment, []UsedShard, error) {
	remaining := target
	selected := make([]acceptedIdentity, 0, target)
	used := make([]UsedShard, 0)
	for _, shard := range shards {
		if remaining == 0 {
			break
		}
		item, err := loadArtifact(shard, selectionSHA, labelsDir, packedDir)
		if err != nil {
			return AcceptedSegment{}, nil, err
		}
		take := uint64(len(item.identities))
		if take > remaining {
			take = remaining
		}
		if take == 0 {
			return AcceptedSegment{}, nil, fmt.Errorf("%w: packed shard %s has no accepted records", ErrContract, shard.ShardID)
		}
		if err := writer.appendPrefix(item.packed.Path, int(take)); err != nil {
			return AcceptedSegment{}, nil, err
		}
		selected = append(selected, item.identities[:take]...)
		used = append(used, UsedShard{
			Stage: stage, Split: split, ShardID: shard.ShardID, CandidateRecords: shard.Records,
			AcceptedAvailable: len(item.identities), AcceptedUsed: int(take), SamplerInput: shard.File,
			Labels: item.labels, PackReceipt: item.packReceipt, Packed: item.packed,
			PackerAcceptedStreamSHA: item.packer.AcceptedStreamSHA256,
		})
		remaining -= take
	}
	if remaining != 0 {
		return AcceptedSegment{}, nil, fmt.Errorf("%w: %s/%s lacks %d accepted records", ErrContract, stage, split, remaining)
	}
	segment := AcceptedSegment{Name: stage + "/" + split, Records: target, AcceptedIdentitySHA256: identityDigest(stage+"/"+split, selected)}
	return segment, used, nil
}

func buildCorpus(outputDir, filename, name, split, stage string, shards []samplerShard, target uint64, selectionSHA, labelsDir, packedDir string) (CorpusReceipt, []UsedShard, error) {
	path := filepath.Join(outputDir, filename)
	writer, err := newCorpusWriter(path)
	if err != nil {
		return CorpusReceipt{}, nil, err
	}
	segment, used, err := appendSeries(writer, stage, split, shards, target, selectionSHA, labelsDir, packedDir)
	if err != nil {
		_ = writer.file.Close()
		return CorpusReceipt{}, nil, err
	}
	file, err := writer.close(path, target)
	if err != nil {
		return CorpusReceipt{}, nil, err
	}
	segments := []AcceptedSegment{segment}
	return CorpusReceipt{Name: name, Split: split, Records: target, RecordBytes: k4pack.RecordBytes, File: file,
		Segments: segments, AcceptedIdentitySHA256: compositeIdentity(segments), IdentityContract: IdentityContract}, used, nil
}

func writeJSONNew(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func startOutput(path string) error {
	if err := os.Mkdir(path, 0o755); err != nil {
		return fmt.Errorf("create new output %s: %w", path, err)
	}
	return os.WriteFile(filepath.Join(path, "STATE"), []byte("FINALIZING\n"), 0o644)
}

func finishOutput(path string, manifest Manifest) error {
	if err := writeJSONNew(filepath.Join(path, "manifest.json"), manifest); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(path, "STATE"), []byte("COMPLETE\n"), 0o644)
}

func failOutput(path string) {
	_ = os.WriteFile(filepath.Join(path, "STATE"), []byte("FAILED\n"), 0o644)
}

// FinalizePilot writes the exact 1M accepted pilot and three fixed 100k
// accepted holdouts. Artifacts are consumed only as a contiguous sampler-shard
// prefix, and the last packed shard is truncated only at a record boundary.
func FinalizePilot(samplerPath, labelsDir, packedDir, outputDir string, command []string) (manifest Manifest, err error) {
	return finalizePilot(samplerPath, labelsDir, packedDir, outputDir, command, productionContract())
}

func finalizePilot(samplerPath, labelsDir, packedDir, outputDir string, command []string, expected contract) (manifest Manifest, err error) {
	sampler, samplerReceipt, groups, err := loadSampler(samplerPath, expected)
	if err != nil {
		return manifest, err
	}
	if err := startOutput(outputDir); err != nil {
		return manifest, err
	}
	defer func() {
		if err != nil {
			failOutput(outputDir)
		}
	}()
	selectionSHA := sampler.Selection.SHA256
	specs := []struct {
		filename, name, stage, split string
		target                       uint64
	}{
		{"train-pilot.bf", "train-pilot", "pilot", "train", expected.pilotAccepted},
		{"validation.bf", "validation", "fixed", "validation", expected.holdoutAccepted},
		{"calibration.bf", "calibration", "fixed", "calibration", expected.holdoutAccepted},
		{"reserved-test.bf", "reserved-test", "fixed", "reserved-test", expected.holdoutAccepted},
	}
	for _, spec := range specs {
		corpus, used, buildErr := buildCorpus(outputDir, spec.filename, spec.name, spec.split, spec.stage,
			groups[seriesKey(spec.stage, spec.split)], spec.target, selectionSHA, labelsDir, packedDir)
		if buildErr != nil {
			return manifest, buildErr
		}
		manifest.Corpora = append(manifest.Corpora, corpus)
		manifest.UsedShards = append(manifest.UsedShards, used...)
	}
	manifest.Schema = ManifestSchema
	manifest.ContractVersion = k4pack.ContractVersion
	manifest.State = "COMPLETE"
	manifest.Mode = "pilot"
	manifest.Command = append([]string(nil), command...)
	manifest.Sampler = samplerReceipt
	if err := finishOutput(outputDir, manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func findCorpus(manifest Manifest, name string) (CorpusReceipt, error) {
	for _, corpus := range manifest.Corpora {
		if corpus.Name == name {
			return corpus, nil
		}
	}
	return CorpusReceipt{}, fmt.Errorf("%w: missing corpus %s", ErrContract, name)
}

func loadPilot(path string, sampler FileReceipt, expected contract) (Manifest, FileReceipt, error) {
	var pilot Manifest
	if err := loadJSON(path, &pilot); err != nil {
		return pilot, FileReceipt{}, err
	}
	pilotReceipt, err := receipt(path)
	if err != nil {
		return pilot, FileReceipt{}, err
	}
	if pilot.Schema != ManifestSchema || pilot.ContractVersion != k4pack.ContractVersion || pilot.State != "COMPLETE" || pilot.Mode != "pilot" ||
		pilot.Sampler.SHA256 != sampler.SHA256 || pilot.Sampler.Bytes != sampler.Bytes {
		return pilot, FileReceipt{}, fmt.Errorf("%w: parent pilot manifest mismatch", ErrContract)
	}
	for _, name := range []string{"train-pilot", "validation", "calibration", "reserved-test"} {
		corpus, err := findCorpus(pilot, name)
		if err != nil {
			return pilot, FileReceipt{}, err
		}
		want := expected.holdoutAccepted
		if name == "train-pilot" {
			want = expected.pilotAccepted
		}
		if corpus.Records != want || corpus.RecordBytes != k4pack.RecordBytes || corpus.IdentityContract != IdentityContract ||
			!validSHA(corpus.AcceptedIdentitySHA256) || corpus.File.Bytes != int64(want*k4pack.RecordBytes) || len(corpus.Segments) != 1 ||
			corpus.Segments[0].Records != want || !validSHA(corpus.Segments[0].AcceptedIdentitySHA256) ||
			compositeIdentity(corpus.Segments) != corpus.AcceptedIdentitySHA256 {
			return pilot, FileReceipt{}, fmt.Errorf("%w: parent corpus %s mismatch", ErrContract, name)
		}
		if err := verifyReceipt(corpus.File); err != nil {
			return pilot, FileReceipt{}, err
		}
	}
	return pilot, pilotReceipt, nil
}

// FinalizeMain prepends the exact pilot corpus and appends the first 19M
// accepted main-expansion records, producing exactly 20M records. The fixed
// holdouts are inherited by hash from the parent pilot manifest.
func FinalizeMain(samplerPath, pilotManifestPath, labelsDir, packedDir, outputDir string, command []string) (manifest Manifest, err error) {
	return finalizeMain(samplerPath, pilotManifestPath, labelsDir, packedDir, outputDir, command, productionContract())
}

func finalizeMain(samplerPath, pilotManifestPath, labelsDir, packedDir, outputDir string, command []string, expected contract) (manifest Manifest, err error) {
	return finalizeExpanded(samplerPath, pilotManifestPath, labelsDir, packedDir, outputDir, command,
		expected, "main", "train-main", expected.mainAccepted)
}

// FinalizeProbe5M freezes the first four million accepted expansion inputs after
// the exact one-million-record pilot prefix. It inherits all three pilot holdouts.
func FinalizeProbe5M(samplerPath, pilotManifestPath, labelsDir, packedDir, outputDir string, command []string) (Manifest, error) {
	return finalizeExpanded(samplerPath, pilotManifestPath, labelsDir, packedDir, outputDir, command,
		productionContract(), "probe5m", "train-probe5m", 5_000_000)
}

func finalizeExpanded(samplerPath, pilotManifestPath, labelsDir, packedDir, outputDir string,
	command []string, expected contract, mode, corpusName string, acceptedTarget uint64,
) (manifest Manifest, err error) {
	if acceptedTarget <= expected.pilotAccepted || acceptedTarget > expected.mainAccepted {
		return manifest, fmt.Errorf("%w: expansion target outside pilot/main bounds", ErrContract)
	}
	sampler, samplerReceipt, groups, err := loadSampler(samplerPath, expected)
	if err != nil {
		return manifest, err
	}
	pilot, pilotReceipt, err := loadPilot(pilotManifestPath, samplerReceipt, expected)
	if err != nil {
		return manifest, err
	}
	if err := startOutput(outputDir); err != nil {
		return manifest, err
	}
	defer func() {
		if err != nil {
			failOutput(outputDir)
		}
	}()
	pilotTrain, _ := findCorpus(pilot, "train-pilot")
	path := filepath.Join(outputDir, corpusName+".bf")
	writer, err := newCorpusWriter(path)
	if err != nil {
		return manifest, err
	}
	if err = writer.appendPrefix(pilotTrain.File.Path, int(pilotTrain.Records)); err != nil {
		_ = writer.file.Close()
		return manifest, err
	}
	expansionTarget := acceptedTarget - expected.pilotAccepted
	expansion, used, err := appendSeries(writer, "main-expansion", "train", groups[seriesKey("main-expansion", "train")],
		expansionTarget, sampler.Selection.SHA256, labelsDir, packedDir)
	if err != nil {
		_ = writer.file.Close()
		return manifest, err
	}
	mainFile, err := writer.close(path, acceptedTarget)
	if err != nil {
		return manifest, err
	}
	pilotSegment := pilotTrain.Segments[0]
	segments := []AcceptedSegment{pilotSegment, expansion}
	mainCorpus := CorpusReceipt{Name: corpusName, Split: "train", Records: acceptedTarget, RecordBytes: k4pack.RecordBytes,
		File: mainFile, Segments: segments, AcceptedIdentitySHA256: compositeIdentity(segments), IdentityContract: IdentityContract}
	manifest = Manifest{Schema: ManifestSchema, ContractVersion: k4pack.ContractVersion, State: "COMPLETE", Mode: mode,
		Command: append([]string(nil), command...), Sampler: samplerReceipt, ParentPilot: &pilotReceipt,
		Corpora: []CorpusReceipt{mainCorpus}, UsedShards: used}
	for _, name := range []string{"validation", "calibration", "reserved-test"} {
		corpus, _ := findCorpus(pilot, name)
		corpus.Inherited = true
		manifest.Corpora = append(manifest.Corpora, corpus)
	}
	if err := finishOutput(outputDir, manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}
