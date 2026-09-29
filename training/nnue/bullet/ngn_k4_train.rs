use std::{
    collections::{BTreeMap, BTreeSet},
    env,
    fs::{self, OpenOptions},
    io::Write,
    path::{Path, PathBuf},
    process::Command,
    sync::{Arc, Mutex},
};

use bullet_lib::{
    game::{
        inputs::{ChessBucketsMirrored, get_num_buckets},
        outputs::MaterialCount,
    },
    nn::{
        InitSettings, Shape,
        optimiser::{AdamW, AdamWOptimiser, AdamWParams},
    },
    trainer::{
        save::SavedFormat,
        schedule::{TrainingSchedule, TrainingSteps, lr, wdl},
        settings::LocalSettings,
    },
    value::{ValueTrainer, ValueTrainerBuilder, loader::DirectSequentialDataLoader},
};
use bullet_trainer::reader::DataReader;
use bulletformat::ChessBoard;
use serde::{Deserialize, Serialize};

const BULLET_COMMIT: &str = "629ee50000b2afb7b3337595401c830d3b1e0f42";
const BULLET_PATCH_SHA256: &str = "f7f5e0dd02695fe57ec58a630b63006da96cb1ec6b8fd4769a4e6308a8121e54";
const FINALIZER_SCHEMA: &str = "ngn-k4-finalized-corpus-v1";
const FINALIZER_CONTRACT: &str = "sf18-5kn-material-inversion-v2";
const CHECKPOINT_SCHEMA: &str = "ngn-k4-bullet-checkpoint-v1";
const ATTEMPT_SCHEMA: &str = "ngn-k4-bullet-attempt-v1";
const COMPLETION_SCHEMA: &str = "ngn-k4-bullet-completion-v1";
const IDENTITY_CONTRACT: &str = "sha256(ngn-k4-final-segments-v1\\0 || segment_name || count:u64le || sha256(ngn-k4-final-accepted-v1\\0 || id:32 || k4_key:32 || ordinal:u64le) for each ordered segment)";
const MODEL_SEED: u64 = 26_092_001;
// Compile with NGN_H1024 set (any value) for the 1024-wide research variant.
const HIDDEN: usize = if option_env!("NGN_H1024").is_some() { 1024 } else { 768 };
// Compile with NGN_WDL25 set for a game-result weight ramping linearly from 0
// to 0.25 across superbatches; the default is the frozen pure-score target.
const WDL_END: f32 = match (option_env!("NGN_WDL25").is_some(), option_env!("NGN_WDL40").is_some()) {
    (false, false) => 0.0,
    (true, false) => 0.25,
    (false, true) => 0.4,
    (true, true) => panic!("select only one result-weight target"),
};
const OUTPUT_BUCKETS: usize = 8;
const SCORE_SCALE: i16 = 400;
const QA: i16 = 255;
const QB: i16 = 64;
const BATCH_SIZE: usize = 16_384;
const LOADER_THREADS: usize = 2;
const BATCH_QUEUE_SIZE: usize = 8;
const INITIAL_LR: f32 = 0.001;
const FINAL_LR: f32 = 0.00005;
const LR1_INITIAL_LR: f32 = 0.0002;
const LR1_FINAL_LR: f32 = 0.00001;
const ARCHIVE_SCHEMA: &str = "ngn-k4-archive-corpus-v1";
const ARCHIVE_FINAL_LR: f32 = 0.00001;
const DEPLOYED_VALUES: usize = INPUT_BUCKETS * 768 * HIDDEN + HIDDEN + OUTPUT_BUCKETS * 2 * HIDDEN + OUTPUT_BUCKETS;
const RAW_BYTES: usize = DEPLOYED_VALUES * 4;
const QUANT_TENSOR_BYTES: usize = DEPLOYED_VALUES * 2;
const QUANT_BYTES: usize = QUANT_TENSOR_BYTES + 48;
const RESUME_MAX_ABS_DELTA: f64 = 1.0e-3;
const RESUME_MAX_QUANTISED_DIFFERENCES: u64 = 64;
const RESUME_MAX_QUANTISED_DELTA: i32 = 1;

// Compile with NGN_K8 set (any value) for the eight-bucket research variant; the default
// build is the frozen four-bucket K4 harness.
const K8: bool = option_env!("NGN_K8").is_some();

#[rustfmt::skip]
const K4_BUCKET_LAYOUT: [usize; 32] = [
    1, 1, 0, 0,
    2, 2, 2, 2,
    3, 3, 3, 3,
    3, 3, 3, 3,
    3, 3, 3, 3,
    3, 3, 3, 3,
    3, 3, 3, 3,
    3, 3, 3, 3,
];

#[rustfmt::skip]
const K8_BUCKET_LAYOUT: [usize; 32] = [
    0, 1, 2, 3,
    4, 4, 5, 5,
    6, 6, 6, 6,
    6, 6, 6, 6,
    7, 7, 7, 7,
    7, 7, 7, 7,
    7, 7, 7, 7,
    7, 7, 7, 7,
];

const BUCKET_LAYOUT: [usize; 32] = if K8 { K8_BUCKET_LAYOUT } else { K4_BUCKET_LAYOUT };

const INPUT_BUCKETS: usize = get_num_buckets(&BUCKET_LAYOUT);

#[derive(Clone, Copy)]
struct Mode {
    name: &'static str,
    manifest_mode: &'static str,
    corpus_name: &'static str,
    positions: u64,
    updates: usize,
    checkpoint_updates: usize,
}

impl Mode {
    fn learning_rates(self) -> (f32, f32) {
        if self.name == "pilot-lr1"
            || self.name == "probe5m-lr1"
            || self.name == "main-m1-lr1"
            || self.name == "main-m1-lr1-32k"
        {
            (LR1_INITIAL_LR, LR1_FINAL_LR)
        } else if self.manifest_mode == "archive" {
            (INITIAL_LR, ARCHIVE_FINAL_LR)
        } else {
            (INITIAL_LR, FINAL_LR)
        }
    }
}

fn mode(value: &str) -> Option<Mode> {
    match value {
        "smoke" => Some(Mode {
            name: "smoke",
            manifest_mode: "pilot",
            corpus_name: "train-pilot",
            positions: 1_000_000,
            updates: 256,
            checkpoint_updates: 128,
        }),
        "pilot" => Some(Mode {
            name: "pilot",
            manifest_mode: "pilot",
            corpus_name: "train-pilot",
            positions: 1_000_000,
            updates: 1_024,
            checkpoint_updates: 128,
        }),
        "pilot-s1" => Some(Mode {
            name: "pilot-s1",
            manifest_mode: "pilot",
            corpus_name: "train-pilot",
            positions: 1_000_000,
            updates: 16_384,
            checkpoint_updates: 512,
        }),
        "pilot-lr1" => Some(Mode {
            name: "pilot-lr1",
            manifest_mode: "pilot",
            corpus_name: "train-pilot",
            positions: 1_000_000,
            updates: 16_384,
            checkpoint_updates: 512,
        }),
        "probe5m" => Some(Mode {
            name: "probe5m",
            manifest_mode: "probe5m",
            corpus_name: "train-probe5m",
            positions: 5_000_000,
            updates: 32_768,
            checkpoint_updates: 1_024,
        }),
        "probe5m-lr1" => Some(Mode {
            name: "probe5m-lr1",
            manifest_mode: "probe5m",
            corpus_name: "train-probe5m",
            positions: 5_000_000,
            updates: 32_768,
            checkpoint_updates: 1_024,
        }),
        "main" => Some(Mode {
            name: "main",
            manifest_mode: "main",
            corpus_name: "train-main",
            positions: 20_000_000,
            updates: 16_384,
            checkpoint_updates: 512,
        }),
        "main-m1" => Some(Mode {
            name: "main-m1",
            manifest_mode: "main",
            corpus_name: "train-main",
            positions: 20_000_000,
            updates: 131_072,
            checkpoint_updates: 4_096,
        }),
        "main-m1-lr1" => Some(Mode {
            name: "main-m1-lr1",
            manifest_mode: "main",
            corpus_name: "train-main",
            positions: 20_000_000,
            updates: 131_072,
            checkpoint_updates: 4_096,
        }),
        "main-m1-lr1-32k" => Some(Mode {
            name: "main-m1-lr1-32k",
            manifest_mode: "main",
            corpus_name: "train-main",
            positions: 20_000_000,
            updates: 32_768,
            checkpoint_updates: 4_096,
        }),
        // Archive modes read their record count from the archive manifest.
        "archive-a1-e1" => Some(Mode {
            name: "archive-a1-e1",
            manifest_mode: "archive",
            corpus_name: "train-archive",
            positions: 0,
            updates: 163_840,
            checkpoint_updates: 16_384,
        }),
        "archive-a1-e10" => Some(Mode {
            name: "archive-a1-e10",
            manifest_mode: "archive",
            corpus_name: "train-archive",
            positions: 0,
            updates: 1_638_400,
            checkpoint_updates: 16_384,
        }),
        "archive-a1-e20" => Some(Mode {
            name: "archive-a1-e20",
            manifest_mode: "archive",
            corpus_name: "train-archive",
            positions: 0,
            updates: 3_276_800,
            checkpoint_updates: 16_384,
        }),
        _ => None,
    }
}

fn fail(message: impl AsRef<str>) -> ! {
    eprintln!("NGN_K4_TRAIN_FAIL {}", message.as_ref());
    std::process::exit(1);
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct FileReceipt {
    path: String,
    bytes: u64,
    sha256: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct AcceptedSegment {
    name: String,
    records: u64,
    accepted_identity_sha256: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct CorpusReceipt {
    name: String,
    split: String,
    records: u64,
    record_bytes: usize,
    file: FileReceipt,
    segments: Vec<AcceptedSegment>,
    accepted_identity_sha256: String,
    identity_contract: String,
    #[serde(default)]
    inherited: bool,
}

#[allow(dead_code)]
#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct UsedShard {
    stage: String,
    split: String,
    shard_id: String,
    candidate_records: usize,
    accepted_available: usize,
    accepted_used: usize,
    sampler_input: FileReceipt,
    labels: FileReceipt,
    pack_receipt: FileReceipt,
    packed: FileReceipt,
    packer_accepted_stream_sha256: String,
}

#[allow(dead_code)]
#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct FinalizedManifest {
    schema: String,
    contract_version: String,
    state: String,
    mode: String,
    command: Vec<String>,
    sampler: FileReceipt,
    #[serde(default)]
    parent_pilot: Option<FileReceipt>,
    corpora: Vec<CorpusReceipt>,
    used_shards: Vec<UsedShard>,
}

#[derive(Clone)]
struct TrainingInput {
    manifest: FileReceipt,
    train: FileReceipt,
}

fn valid_sha(value: &str) -> bool {
    value.len() == 64 && value.bytes().all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
}

fn sha256_file(path: &Path) -> String {
    let output = Command::new("sha256sum")
        .arg("--")
        .arg(path)
        .output()
        .unwrap_or_else(|error| fail(format!("run sha256sum {}: {error}", path.display())));
    if !output.status.success() {
        fail(format!(
            "sha256sum {} exited {:?}: {}",
            path.display(),
            output.status.code(),
            String::from_utf8_lossy(&output.stderr)
        ));
    }
    let stdout = String::from_utf8(output.stdout)
        .unwrap_or_else(|error| fail(format!("sha256sum output was not UTF-8: {error}")));
    let digest = stdout.split_whitespace().next().unwrap_or_else(|| fail("sha256sum returned empty output"));
    if !valid_sha(digest) {
        fail(format!("sha256sum returned invalid digest {digest:?}"));
    }
    digest.to_string()
}

fn file_receipt(path: &Path) -> FileReceipt {
    let metadata = fs::metadata(path).unwrap_or_else(|error| fail(format!("metadata {}: {error}", path.display())));
    if !metadata.is_file() {
        fail(format!("not a regular file: {}", path.display()));
    }
    FileReceipt { path: path.display().to_string(), bytes: metadata.len(), sha256: sha256_file(path) }
}

fn verify_file_receipt(item: &FileReceipt) {
    if item.path.is_empty() || !valid_sha(&item.sha256) {
        fail("malformed file receipt");
    }
    let actual = file_receipt(Path::new(&item.path));
    if actual.bytes != item.bytes || actual.sha256 != item.sha256 {
        fail(format!("file receipt mismatch {}", item.path));
    }
}

fn verify_bound_file(item: &FileReceipt, path: &Path) {
    verify_file_receipt(item);
    let bound = file_receipt(path);
    if bound.bytes != item.bytes || bound.sha256 != item.sha256 {
        fail(format!("receipt is not bound to checkpoint file {}", path.display()));
    }
}

fn read_json<T: for<'de> Deserialize<'de>>(path: &Path) -> T {
    let bytes = fs::read(path).unwrap_or_else(|error| fail(format!("read JSON {}: {error}", path.display())));
    serde_json::from_slice(&bytes).unwrap_or_else(|error| fail(format!("decode JSON {}: {error}", path.display())))
}

fn write_json_new<T: Serialize>(path: &Path, value: &T) {
    let mut bytes = serde_json::to_vec_pretty(value)
        .unwrap_or_else(|error| fail(format!("encode JSON {}: {error}", path.display())));
    bytes.push(b'\n');
    let mut file = OpenOptions::new()
        .create_new(true)
        .write(true)
        .open(path)
        .unwrap_or_else(|error| fail(format!("create {}: {error}", path.display())));
    file.write_all(&bytes).unwrap_or_else(|error| fail(format!("write {}: {error}", path.display())));
    file.sync_all().unwrap_or_else(|error| fail(format!("sync {}: {error}", path.display())));
}

#[allow(dead_code)]
#[derive(Clone, Debug, Deserialize)]
struct ArchiveManifest {
    schema: String,
    state: String,
    command: Vec<String>,
    scatter: FileReceipt,
    score_map: FileReceipt,
    knots: Vec<[f64; 2]>,
    score_contract: String,
    shuffle_seed: u64,
    records: u64,
    record_bytes: usize,
    dropped_sentinel: u64,
    dropped_abs_score: u64,
    train: FileReceipt,
    elapsed_seconds: f64,
}

fn load_archive_input(path: &Path) -> (TrainingInput, u64) {
    let manifest_receipt = file_receipt(path);
    let manifest: ArchiveManifest = read_json(path);
    if manifest.schema != ARCHIVE_SCHEMA
        || manifest.state != "COMPLETE"
        || manifest.record_bytes != 32
        || manifest.records == 0
        || manifest.train.bytes != manifest.records * 32
        || manifest.knots.len() < 2
        || manifest.knots.windows(2).any(|pair| pair[1][0] <= pair[0][0] || pair[1][1] < pair[0][1])
    {
        fail("archive corpus manifest contract mismatch");
    }
    verify_file_receipt(&manifest.scatter);
    verify_file_receipt(&manifest.score_map);
    verify_file_receipt(&manifest.train);
    (TrainingInput { manifest: manifest_receipt, train: manifest.train }, manifest.records)
}

fn load_training_input(path: &Path, run_mode: Mode) -> TrainingInput {
    let manifest_receipt = file_receipt(path);
    let manifest: FinalizedManifest = read_json(path);
    if manifest.schema != FINALIZER_SCHEMA
        || manifest.contract_version != FINALIZER_CONTRACT
        || manifest.state != "COMPLETE"
        || manifest.mode != run_mode.manifest_mode
        || manifest.command.is_empty()
        || manifest.used_shards.is_empty()
        || ((manifest.mode == "pilot") != manifest.parent_pilot.is_none())
    {
        fail("finalized training manifest contract mismatch");
    }
    verify_file_receipt(&manifest.sampler);
    if let Some(parent) = &manifest.parent_pilot {
        verify_file_receipt(parent);
    }
    let mut names = BTreeSet::new();
    let mut selected = None;
    for corpus in manifest.corpora {
        if !names.insert(corpus.name.clone())
            || corpus.record_bytes != 32
            || corpus.file.bytes != corpus.records * 32
            || corpus.identity_contract != IDENTITY_CONTRACT
            || !valid_sha(&corpus.accepted_identity_sha256)
            || corpus.segments.is_empty()
            || corpus.segments.iter().any(|segment| {
                segment.name.is_empty() || segment.records == 0 || !valid_sha(&segment.accepted_identity_sha256)
            })
            || corpus.segments.iter().map(|segment| segment.records).sum::<u64>() != corpus.records
        {
            fail(format!("invalid finalized corpus {}", corpus.name));
        }
        verify_file_receipt(&corpus.file);
        match corpus.name.as_str() {
            name if name == run_mode.corpus_name => {
                let valid_train = corpus.split == "train"
                    && corpus.records == run_mode.positions
                    && !corpus.inherited
                    && selected.is_none()
                    && match manifest.mode.as_str() {
                        "pilot" => {
                            corpus.segments.len() == 1
                                && corpus.segments[0].name == "pilot/train"
                                && corpus.segments[0].records == 1_000_000
                        }
                        "main" => {
                            corpus.segments.len() == 2
                                && corpus.segments[0].name == "pilot/train"
                                && corpus.segments[0].records == 1_000_000
                                && corpus.segments[1].name == "main-expansion/train"
                                && corpus.segments[1].records == 19_000_000
                        }
                        "probe5m" => {
                            corpus.segments.len() == 2
                                && corpus.segments[0].name == "pilot/train"
                                && corpus.segments[0].records == 1_000_000
                                && corpus.segments[1].name == "main-expansion/train"
                                && corpus.segments[1].records == 4_000_000
                        }
                        _ => false,
                    };
                if !valid_train {
                    fail("training corpus contract mismatch");
                }
                selected = Some(corpus.file.clone());
            }
            "validation" | "calibration" | "reserved-test" => {
                if corpus.split != corpus.name
                    || corpus.records != 100_000
                    || corpus.inherited != (manifest.mode != "pilot")
                    || corpus.segments.len() != 1
                    || corpus.segments[0].name != format!("fixed/{}", corpus.name)
                    || corpus.segments[0].records != 100_000
                {
                    fail(format!("holdout corpus contract mismatch {}", corpus.name));
                }
            }
            _ => fail(format!("unexpected finalized corpus {}", corpus.name)),
        }
    }
    let expected_names = BTreeSet::from([
        run_mode.corpus_name.to_string(),
        "validation".to_string(),
        "calibration".to_string(),
        "reserved-test".to_string(),
    ]);
    if names != expected_names {
        fail("finalized corpus set mismatch");
    }
    let train = selected.unwrap_or_else(|| fail(format!("missing corpus {}", run_mode.corpus_name)));
    TrainingInput { manifest: manifest_receipt, train }
}

#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
struct ReaderWitness {
    skip_records: u64,
    start_cursor: u64,
    end_cursor: u64,
    batches: u64,
    records: u64,
}

#[derive(Clone)]
struct AuditedSequentialDataLoader {
    inner: DirectSequentialDataLoader,
    total_records: u64,
    batch_size: usize,
    expected_skip: u64,
    expected_batches: u64,
    witness: Arc<Mutex<ReaderWitness>>,
}

impl AuditedSequentialDataLoader {
    fn new(path: &str, total_records: u64, batch_size: usize, expected_skip: u64, expected_batches: u64) -> Self {
        Self {
            inner: DirectSequentialDataLoader::new(&[path]),
            total_records,
            batch_size,
            expected_skip,
            expected_batches,
            witness: Arc::new(Mutex::new(ReaderWitness::default())),
        }
    }

    fn verified_witness(&self) -> ReaderWitness {
        let witness = self.witness.lock().unwrap().clone();
        let expected_records = self.expected_batches * self.batch_size as u64;
        let expected_end = (self.expected_skip + expected_records) % self.total_records;
        if witness.skip_records != self.expected_skip
            || witness.start_cursor != self.expected_skip % self.total_records
            || witness.end_cursor != expected_end
            || witness.batches != self.expected_batches
            || witness.records != expected_records
        {
            fail(format!(
                "reader witness mismatch got={witness:?} expected_skip={} expected_batches={}",
                self.expected_skip, self.expected_batches
            ));
        }
        witness
    }
}

impl DataReader<ChessBoard> for AuditedSequentialDataLoader {
    fn read_chunks<F: FnMut(&[ChessBoard]) -> bool>(&self, skip_count: usize, mut callback: F) {
        let skip_count = skip_count as u64;
        if skip_count != self.expected_skip {
            fail(format!("Bullet requested skip={skip_count}, expected={}", self.expected_skip));
        }
        {
            let mut witness = self.witness.lock().unwrap();
            witness.skip_records = skip_count;
            witness.start_cursor = skip_count % self.total_records;
            witness.end_cursor = witness.start_cursor;
        }
        let batch_size = self.batch_size;
        let total_records = self.total_records;
        let expected_batches = self.expected_batches;
        let witness = self.witness.clone();
        let mut carry = Vec::with_capacity(batch_size);
        self.inner.read_chunks(skip_count as usize, |chunk: &[ChessBoard]| {
            let mut process = |batch: &[ChessBoard]| -> bool {
                if batch.len() != batch_size {
                    fail("audited reader produced a partial batch");
                }
                {
                    let mut state = witness.lock().unwrap();
                    state.batches += 1;
                    state.records += batch_size as u64;
                    state.end_cursor = (state.start_cursor + state.records) % total_records;
                    if state.batches > expected_batches {
                        fail("audited reader exceeded scheduled batches");
                    }
                }
                callback(batch)
            };

            let mut offset = 0;
            if !carry.is_empty() {
                let needed = batch_size - carry.len();
                let take = needed.min(chunk.len());
                carry.extend_from_slice(&chunk[..take]);
                offset += take;
                if carry.len() == batch_size {
                    if process(&carry) {
                        return true;
                    }
                    carry.clear();
                } else {
                    return false;
                }
            }
            while offset + batch_size <= chunk.len() {
                if process(&chunk[offset..offset + batch_size]) {
                    return true;
                }
                offset += batch_size;
            }
            carry.extend_from_slice(&chunk[offset..]);
            false
        });
    }
}

fn f32_values(path: &Path) -> Vec<f32> {
    let bytes = fs::read(path).unwrap_or_else(|error| fail(format!("read {}: {error}", path.display())));
    if bytes.len() != RAW_BYTES {
        fail(format!("{} bytes={} expected={RAW_BYTES}", path.display(), bytes.len()));
    }
    let values: Vec<f32> = bytes.chunks_exact(4).map(|word| f32::from_le_bytes(word.try_into().unwrap())).collect();
    if let Some((index, value)) = values.iter().enumerate().find(|(_, value)| !value.is_finite()) {
        fail(format!("{} raw[{index}] nonfinite={value}", path.display()));
    }
    values
}

fn quantise(value: f32, scale: i16) -> i16 {
    let rounded = (f64::from(value) * f64::from(scale)).round();
    if !(f64::from(i16::MIN)..=f64::from(i16::MAX)).contains(&rounded) {
        fail(format!("quantisation range value={value} scale={scale}"));
    }
    rounded as i16
}

fn validate_quantised(raw: &[f32], path: &Path) {
    let bytes = fs::read(path).unwrap_or_else(|error| fail(format!("read {}: {error}", path.display())));
    if bytes.len() != QUANT_BYTES {
        fail(format!("{} bytes={} expected={QUANT_BYTES}", path.display(), bytes.len()));
    }
    let sections = [
        (INPUT_BUCKETS * 768 * HIDDEN, QA),
        (HIDDEN, QA),
        (OUTPUT_BUCKETS * 2 * HIDDEN, QB),
        (OUTPUT_BUCKETS, QA * QB),
    ];
    let mut raw_offset = 0usize;
    let mut byte_offset = 0usize;
    for (count, scale) in sections {
        for &value in &raw[raw_offset..raw_offset + count] {
            let expected = quantise(value, scale).to_le_bytes();
            if bytes[byte_offset..byte_offset + 2] != expected {
                fail(format!(
                    "{} quantised mismatch tensor_index={} got={:?} expected={expected:?}",
                    path.display(),
                    raw_offset,
                    &bytes[byte_offset..byte_offset + 2],
                ));
            }
            raw_offset += 1;
            byte_offset += 2;
        }
    }
    if raw_offset != DEPLOYED_VALUES || byte_offset != QUANT_TENSOR_BYTES {
        fail("internal tensor count mismatch");
    }
    if bytes[QUANT_TENSOR_BYTES..].iter().enumerate().any(|(index, byte)| *byte != b"bullet"[index % 6]) {
        fail(format!("{} invalid Bullet trailer", path.display()));
    }
}

fn validate_checkpoint(path: &Path) {
    let raw = f32_values(&path.join("raw.bin"));
    validate_quantised(&raw, &path.join("quantised.bin"));
    for state in ["weights.bin", "momentum.bin", "velocity.bin"] {
        let state_path = path.join("optimiser_state").join(state);
        let metadata = fs::metadata(&state_path)
            .unwrap_or_else(|error| fail(format!("optimiser state {}: {error}", state_path.display())));
        if metadata.len() == 0 || metadata.len() % 4 != 0 {
            fail(format!("invalid optimiser state {} bytes={}", state_path.display(), metadata.len()));
        }
    }
}

fn read_labeled_f32(path: &Path) -> BTreeMap<String, Vec<f32>> {
    let bytes = fs::read(path).unwrap_or_else(|error| fail(format!("read {}: {error}", path.display())));
    let mut tensors = BTreeMap::new();
    let mut offset = 0usize;
    while offset < bytes.len() {
        let relative_end = bytes[offset..]
            .iter()
            .position(|byte| *byte == b'\n')
            .unwrap_or_else(|| fail(format!("{} tensor name is unterminated", path.display())));
        let name_end = offset + relative_end;
        let name = std::str::from_utf8(&bytes[offset..name_end])
            .unwrap_or_else(|error| fail(format!("{} tensor name is not UTF-8: {error}", path.display())))
            .to_string();
        if name.is_empty() {
            fail(format!("{} contains an empty tensor name", path.display()));
        }
        offset = name_end + 1;
        let size_end = offset
            .checked_add(size_of::<usize>())
            .filter(|end| *end <= bytes.len())
            .unwrap_or_else(|| fail(format!("{} tensor {name} lacks a size", path.display())));
        let count = usize::from_le_bytes(bytes[offset..size_end].try_into().unwrap());
        offset = size_end;
        let values_end = count
            .checked_mul(4)
            .and_then(|value_bytes| offset.checked_add(value_bytes))
            .filter(|end| *end <= bytes.len())
            .unwrap_or_else(|| fail(format!("{} tensor {name} is truncated", path.display())));
        let values: Vec<f32> = bytes[offset..values_end]
            .chunks_exact(4)
            .map(|word| f32::from_le_bytes(word.try_into().unwrap()))
            .collect();
        if values.iter().any(|value| !value.is_finite()) {
            fail(format!("{} tensor {name} contains a nonfinite value", path.display()));
        }
        if tensors.insert(name.clone(), values).is_some() {
            fail(format!("{} repeats tensor {name}", path.display()));
        }
        offset = values_end;
    }
    tensors
}

fn float_delta(label: &str, left: &[f32], right: &[f32]) -> (u64, f64) {
    if left.len() != right.len() {
        fail(format!("{label} value count mismatch {} != {}", left.len(), right.len()));
    }
    let mut different = 0u64;
    let mut max_abs = 0.0f64;
    for (&a, &b) in left.iter().zip(right) {
        let delta = (f64::from(a) - f64::from(b)).abs();
        if delta != 0.0 {
            different += 1;
            max_abs = max_abs.max(delta);
        }
    }
    if max_abs > RESUME_MAX_ABS_DELTA {
        fail(format!("{label} max_abs={max_abs:e} exceeds {RESUME_MAX_ABS_DELTA:e}"));
    }
    (different, max_abs)
}

fn compare_checkpoints(left: &Path, right: &Path) {
    validate_checkpoint(left);
    validate_checkpoint(right);
    let left_quantised = fs::read(left.join("quantised.bin"))
        .unwrap_or_else(|error| fail(format!("read quantised {}: {error}", left.display())));
    let right_quantised = fs::read(right.join("quantised.bin"))
        .unwrap_or_else(|error| fail(format!("read quantised {}: {error}", right.display())));
    if left_quantised.len() != QUANT_BYTES
        || right_quantised.len() != QUANT_BYTES
        || left_quantised[QUANT_TENSOR_BYTES..] != right_quantised[QUANT_TENSOR_BYTES..]
    {
        fail("resume quantised size/trailer mismatch");
    }
    let mut quantised_different = 0u64;
    let mut quantised_max_delta = 0i32;
    for (left_word, right_word) in
        left_quantised[..QUANT_TENSOR_BYTES].chunks_exact(2).zip(right_quantised[..QUANT_TENSOR_BYTES].chunks_exact(2))
    {
        let left_value = i32::from(i16::from_le_bytes(left_word.try_into().unwrap()));
        let right_value = i32::from(i16::from_le_bytes(right_word.try_into().unwrap()));
        let delta = (left_value - right_value).abs();
        if delta != 0 {
            quantised_different += 1;
            quantised_max_delta = quantised_max_delta.max(delta);
        }
    }
    if quantised_different > RESUME_MAX_QUANTISED_DIFFERENCES || quantised_max_delta > RESUME_MAX_QUANTISED_DELTA {
        fail(format!("resume quantised divergence different={quantised_different} max_delta={quantised_max_delta}"));
    }

    let (mut different, mut max_abs) =
        float_delta("raw", &f32_values(&left.join("raw.bin")), &f32_values(&right.join("raw.bin")));
    for state in ["weights.bin", "momentum.bin", "velocity.bin"] {
        let left_tensors = read_labeled_f32(&left.join("optimiser_state").join(state));
        let right_tensors = read_labeled_f32(&right.join("optimiser_state").join(state));
        if left_tensors.keys().ne(right_tensors.keys()) {
            fail(format!("{state} tensor set mismatch"));
        }
        for (name, left_values) in left_tensors {
            let right_values = &right_tensors[&name];
            let (this_different, this_max_abs) = float_delta(&format!("{state}:{name}"), &left_values, right_values);
            different += this_different;
            max_abs = max_abs.max(this_max_abs);
        }
    }
    println!(
        "NGN_K4_CHECKPOINT_COMPARE_PASS quantised_different={quantised_different} quantised_max_delta={quantised_max_delta} quantised_difference_limit={RESUME_MAX_QUANTISED_DIFFERENCES} different_float_values={different} max_abs={max_abs:e} tolerance={RESUME_MAX_ABS_DELTA:e}"
    );
}

fn deployed_raw_format() -> Vec<SavedFormat> {
    vec![
        SavedFormat::id("l0w").transform(|store, weights| {
            let factoriser = store.get("l0f").values.f32().repeat(INPUT_BUCKETS);
            weights.into_iter().zip(factoriser).map(|(main, shared)| main + shared).collect()
        }),
        SavedFormat::id("l0b"),
        SavedFormat::id("l1w").transpose(),
        SavedFormat::id("l1b"),
    ]
}

fn deployed_quantised_format() -> Vec<SavedFormat> {
    vec![
        SavedFormat::id("l0w")
            .transform(|store, weights| {
                let factoriser = store.get("l0f").values.f32().repeat(INPUT_BUCKETS);
                weights.into_iter().zip(factoriser).map(|(main, shared)| main + shared).collect()
            })
            .round()
            .quantise::<i16>(QA),
        SavedFormat::id("l0b").round().quantise::<i16>(QA),
        SavedFormat::id("l1w").round().quantise::<i16>(QB).transpose(),
        SavedFormat::id("l1b").round().quantise::<i16>(QA * QB),
    ]
}

fn write_deployed_raw<T, I, O>(trainer: &bullet_lib::value::ValueTrainer<T, I, O>, path: &Path)
where
    T: bullet_trainer::optimiser::OptimiserState<bullet_lib::nn::ExecutionContext>,
    I: bullet_lib::game::inputs::SparseInputType,
{
    let weights = trainer.optimiser.cpu_weights().unwrap_or_else(|error| fail(format!("read CPU weights: {error:?}")));
    let bytes = weights
        .to_quantised_buffer(&deployed_raw_format(), false)
        .unwrap_or_else(|error| fail(format!("encode deployed raw {}: {error}", path.display())));
    fs::write(path, bytes).unwrap_or_else(|error| fail(format!("write deployed raw {}: {error}", path.display())));
}

fn strict_save<T, I, O>(trainer: &bullet_lib::value::ValueTrainer<T, I, O>, root: &Path)
where
    T: bullet_trainer::optimiser::OptimiserState<bullet_lib::nn::ExecutionContext>,
    I: bullet_lib::game::inputs::SparseInputType,
{
    fs::create_dir(root).unwrap_or_else(|error| fail(format!("create {}: {error}", root.display())));
    let optimiser_state = root.join("optimiser_state");
    fs::create_dir(&optimiser_state)
        .unwrap_or_else(|error| fail(format!("create {}: {error}", optimiser_state.display())));
    trainer
        .optimiser
        .write_to_checkpoint(optimiser_state.to_str().unwrap())
        .unwrap_or_else(|error| fail(format!("write optimiser {}: {error:?}", root.display())));
    write_deployed_raw(trainer, &root.join("raw.bin"));
    trainer
        .save_quantised(root.join("quantised.bin").to_str().unwrap())
        .unwrap_or_else(|error| fail(format!("save quantised {}: {error}", root.display())));
    validate_checkpoint(root);
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct CheckpointFiles {
    raw: FileReceipt,
    quantised: FileReceipt,
    weights: FileReceipt,
    momentum: FileReceipt,
    velocity: FileReceipt,
    log: Option<FileReceipt>,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct CheckpointReceipt {
    schema: String,
    bullet_commit: String,
    bullet_patch_sha256: String,
    mode: String,
    finalized_manifest: FileReceipt,
    train: FileReceipt,
    attempt_contract: FileReceipt,
    parent_checkpoint: Option<FileReceipt>,
    completed_updates: u64,
    total_updates: u64,
    checkpoint_updates: u64,
    batch_size: u64,
    presentations: u64,
    full_epochs: u64,
    data_cursor: u64,
    reader: ReaderWitness,
    input_buckets: usize,
    hidden: usize,
    output_buckets: usize,
    model_seed: u64,
    score_scale: i16,
    initial_lr: f32,
    final_lr: f32,
    loader_threads: usize,
    batch_queue_size: usize,
    files: CheckpointFiles,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct AttemptContract {
    schema: String,
    bullet_commit: String,
    bullet_patch_sha256: String,
    mode: String,
    finalized_manifest: FileReceipt,
    train: FileReceipt,
    executable: FileReceipt,
    parent_checkpoint: Option<FileReceipt>,
    resume_load_audit: Option<CheckpointFiles>,
    start_updates: u64,
    total_updates: u64,
    checkpoint_updates: u64,
    batch_size: u64,
    train_positions: u64,
    model_seed: u64,
    input_buckets: usize,
    hidden: usize,
    output_buckets: usize,
    score_scale: i16,
    // Legacy pure-score attempts did not record this field. Their executable
    // hash still binds the old harness; new weighted attempts record it.
    #[serde(default)]
    result_weight_end: f32,
    initial_lr: f32,
    final_lr: f32,
    loader_threads: usize,
    batch_queue_size: usize,
}

#[derive(Clone, Debug, Serialize)]
struct CompletionReceipt {
    schema: String,
    attempt_contract: FileReceipt,
    final_checkpoint: FileReceipt,
    completed_updates: u64,
    presentations: u64,
    data_cursor: u64,
}

fn checkpoint_files(root: &Path) -> CheckpointFiles {
    let optional_log = root.join("log.txt");
    CheckpointFiles {
        raw: file_receipt(&root.join("raw.bin")),
        quantised: file_receipt(&root.join("quantised.bin")),
        weights: file_receipt(&root.join("optimiser_state/weights.bin")),
        momentum: file_receipt(&root.join("optimiser_state/momentum.bin")),
        velocity: file_receipt(&root.join("optimiser_state/velocity.bin")),
        log: optional_log.exists().then(|| file_receipt(&optional_log)),
    }
}

fn verify_checkpoint_files(files: &CheckpointFiles) {
    for item in [&files.raw, &files.quantised, &files.weights, &files.momentum, &files.velocity] {
        verify_file_receipt(item);
    }
    if let Some(log) = &files.log {
        verify_file_receipt(log);
    }
}

fn require_same_checkpoint_state(expected: &CheckpointFiles, actual: &CheckpointFiles) {
    for (name, left, right) in [
        ("raw", &expected.raw, &actual.raw),
        ("quantised", &expected.quantised, &actual.quantised),
        ("weights", &expected.weights, &actual.weights),
        ("momentum", &expected.momentum, &actual.momentum),
        ("velocity", &expected.velocity, &actual.velocity),
    ] {
        if left.bytes != right.bytes || left.sha256 != right.sha256 {
            fail(format!("resume load audit mismatch: {name}"));
        }
    }
}

fn write_checkpoint_receipt(
    root: &Path,
    run_mode: Mode,
    input: &TrainingInput,
    attempt: &FileReceipt,
    parent: Option<FileReceipt>,
    completed_updates: u64,
    reader: ReaderWitness,
) -> FileReceipt {
    validate_checkpoint(root);
    let (initial_lr, final_lr) = run_mode.learning_rates();
    let presentations = completed_updates * BATCH_SIZE as u64;
    let receipt_value = CheckpointReceipt {
        schema: CHECKPOINT_SCHEMA.to_string(),
        bullet_commit: BULLET_COMMIT.to_string(),
        bullet_patch_sha256: BULLET_PATCH_SHA256.to_string(),
        mode: run_mode.name.to_string(),
        finalized_manifest: input.manifest.clone(),
        train: input.train.clone(),
        attempt_contract: attempt.clone(),
        parent_checkpoint: parent,
        completed_updates,
        total_updates: run_mode.updates as u64,
        checkpoint_updates: run_mode.checkpoint_updates as u64,
        batch_size: BATCH_SIZE as u64,
        presentations,
        full_epochs: presentations / run_mode.positions,
        data_cursor: presentations % run_mode.positions,
        reader,
        input_buckets: INPUT_BUCKETS,
        hidden: HIDDEN,
        output_buckets: OUTPUT_BUCKETS,
        model_seed: MODEL_SEED,
        score_scale: SCORE_SCALE,
        initial_lr,
        final_lr,
        loader_threads: LOADER_THREADS,
        batch_queue_size: BATCH_QUEUE_SIZE,
        files: checkpoint_files(root),
    };
    let path = root.join("receipt.json");
    write_json_new(&path, &receipt_value);
    file_receipt(&path)
}

fn load_parent_checkpoint(
    path: &Path,
    run_mode: Mode,
    input: &TrainingInput,
    executable: &FileReceipt,
) -> (CheckpointReceipt, FileReceipt) {
    let (initial_lr, final_lr) = run_mode.learning_rates();
    let receipt_file = file_receipt(path);
    let checkpoint: CheckpointReceipt = read_json(path);
    if checkpoint.schema != CHECKPOINT_SCHEMA
        || checkpoint.bullet_commit != BULLET_COMMIT
        || checkpoint.bullet_patch_sha256 != BULLET_PATCH_SHA256
        || checkpoint.mode != run_mode.name
        || checkpoint.finalized_manifest.sha256 != input.manifest.sha256
        || checkpoint.train.sha256 != input.train.sha256
        || checkpoint.total_updates != run_mode.updates as u64
        || checkpoint.checkpoint_updates != run_mode.checkpoint_updates as u64
        || checkpoint.batch_size != BATCH_SIZE as u64
        || checkpoint.completed_updates >= checkpoint.total_updates
        || checkpoint.completed_updates % checkpoint.checkpoint_updates != 0
        || checkpoint.presentations != checkpoint.completed_updates * checkpoint.batch_size
        || checkpoint.full_epochs != checkpoint.presentations / run_mode.positions
        || checkpoint.data_cursor != checkpoint.presentations % run_mode.positions
        || checkpoint.input_buckets != INPUT_BUCKETS
        || checkpoint.hidden != HIDDEN
        || checkpoint.output_buckets != OUTPUT_BUCKETS
        || checkpoint.model_seed != MODEL_SEED
        || checkpoint.score_scale != SCORE_SCALE
        || checkpoint.initial_lr != initial_lr
        || checkpoint.final_lr != final_lr
        || checkpoint.loader_threads != LOADER_THREADS
        || checkpoint.batch_queue_size != BATCH_QUEUE_SIZE
    {
        fail("parent checkpoint contract mismatch");
    }
    let expected_reader = if checkpoint.completed_updates == 0 {
        ReaderWitness::default()
    } else {
        let skip_records = (checkpoint.completed_updates - checkpoint.checkpoint_updates) * checkpoint.batch_size;
        let records = checkpoint.checkpoint_updates * checkpoint.batch_size;
        ReaderWitness {
            skip_records,
            start_cursor: skip_records % run_mode.positions,
            end_cursor: (skip_records + records) % run_mode.positions,
            batches: checkpoint.checkpoint_updates,
            records,
        }
    };
    if checkpoint.reader != expected_reader {
        fail("parent checkpoint reader witness mismatch");
    }
    verify_file_receipt(&checkpoint.finalized_manifest);
    verify_file_receipt(&checkpoint.train);
    verify_file_receipt(&checkpoint.attempt_contract);
    let parent_attempt: AttemptContract = read_json(Path::new(&checkpoint.attempt_contract.path));
    if parent_attempt.schema != ATTEMPT_SCHEMA
        || parent_attempt.bullet_commit != BULLET_COMMIT
        || parent_attempt.bullet_patch_sha256 != BULLET_PATCH_SHA256
        || parent_attempt.mode != run_mode.name
        || parent_attempt.finalized_manifest.sha256 != input.manifest.sha256
        || parent_attempt.train.sha256 != input.train.sha256
        || parent_attempt.executable.bytes != executable.bytes
        || parent_attempt.executable.sha256 != executable.sha256
        || parent_attempt.parent_checkpoint.is_some() != parent_attempt.resume_load_audit.is_some()
        || parent_attempt.start_updates > checkpoint.completed_updates
        || parent_attempt.total_updates != run_mode.updates as u64
        || parent_attempt.checkpoint_updates != run_mode.checkpoint_updates as u64
        || parent_attempt.batch_size != BATCH_SIZE as u64
        || parent_attempt.train_positions != run_mode.positions
        || parent_attempt.model_seed != MODEL_SEED
        || parent_attempt.input_buckets != INPUT_BUCKETS
        || parent_attempt.hidden != HIDDEN
        || parent_attempt.output_buckets != OUTPUT_BUCKETS
        || parent_attempt.score_scale != SCORE_SCALE
        || parent_attempt.result_weight_end != WDL_END
        || parent_attempt.initial_lr != initial_lr
        || parent_attempt.final_lr != final_lr
        || parent_attempt.loader_threads != LOADER_THREADS
        || parent_attempt.batch_queue_size != BATCH_QUEUE_SIZE
    {
        fail("parent attempt contract mismatch");
    }
    verify_file_receipt(&parent_attempt.finalized_manifest);
    verify_file_receipt(&parent_attempt.train);
    verify_file_receipt(&parent_attempt.executable);
    if let (Some(parent), Some(audit)) = (&parent_attempt.parent_checkpoint, &parent_attempt.resume_load_audit) {
        verify_file_receipt(parent);
        verify_checkpoint_files(audit);
        let prior: CheckpointReceipt = read_json(Path::new(&parent.path));
        require_same_checkpoint_state(&prior.files, audit);
    }
    if let Some(parent) = &checkpoint.parent_checkpoint {
        verify_file_receipt(parent);
    }
    let parent_dir = path.parent().unwrap_or_else(|| fail("parent checkpoint receipt has no directory"));
    verify_bound_file(&checkpoint.files.raw, &parent_dir.join("raw.bin"));
    verify_bound_file(&checkpoint.files.quantised, &parent_dir.join("quantised.bin"));
    verify_bound_file(&checkpoint.files.weights, &parent_dir.join("optimiser_state/weights.bin"));
    verify_bound_file(&checkpoint.files.momentum, &parent_dir.join("optimiser_state/momentum.bin"));
    verify_bound_file(&checkpoint.files.velocity, &parent_dir.join("optimiser_state/velocity.bin"));
    if let Some(log) = &checkpoint.files.log {
        verify_bound_file(log, &parent_dir.join("log.txt"));
    }
    validate_checkpoint(parent_dir);
    (checkpoint, receipt_file)
}

fn build_trainer() -> ValueTrainer<AdamWOptimiser, ChessBucketsMirrored, MaterialCount<OUTPUT_BUCKETS>> {
    ValueTrainerBuilder::default()
        .seed(MODEL_SEED)
        .dual_perspective()
        .optimiser(AdamW)
        .inputs(ChessBucketsMirrored::new(BUCKET_LAYOUT))
        .output_buckets(MaterialCount::<OUTPUT_BUCKETS>)
        .save_format(&deployed_quantised_format())
        .loss_fn(|network_output, target| network_output.sigmoid().squared_error(target))
        .build(|builder, stm_inputs, ntm_inputs, output_buckets| {
            let factoriser = builder.new_weights("l0f", Shape::new(HIDDEN, 768), InitSettings::Zeroed);
            let expanded_factoriser = factoriser.repeat(INPUT_BUCKETS);
            let mut l0 = builder.new_affine("l0", 768 * INPUT_BUCKETS, HIDDEN);
            l0.weights = l0.weights + expanded_factoriser;
            let l1 = builder.new_affine("l1", 2 * HIDDEN, OUTPUT_BUCKETS);
            let stm_hidden = l0.forward(stm_inputs).screlu();
            let ntm_hidden = l0.forward(ntm_inputs).screlu();
            l1.forward(stm_hidden.concat(ntm_hidden)).select(output_buckets)
        })
}

fn probe(checkpoint: &Path, fen_path: &Path) {
    if !checkpoint.is_dir() {
        fail(format!("probe checkpoint is not a directory: {}", checkpoint.display()));
    }
    validate_checkpoint(checkpoint);
    let fen_text =
        fs::read_to_string(fen_path).unwrap_or_else(|error| fail(format!("read FENs {}: {error}", fen_path.display())));
    let fens: Vec<_> =
        fen_text.lines().map(str::trim).filter(|line| !line.is_empty() && !line.starts_with('#')).collect();
    if fens.is_empty() || fens.iter().any(|fen| fen.contains('\t')) {
        fail("probe FEN file must contain nonempty, tab-free positions");
    }
    let mut trainer = build_trainer();
    trainer.load_from_checkpoint(checkpoint.to_str().unwrap());
    for (index, fen) in fens.iter().enumerate() {
        let output = trainer.eval_raw_output(fen);
        if output.len() != 1 || !output[0].is_finite() {
            fail(format!("probe {index} returned {output:?}"));
        }
        println!("NGN_K4_PROBE\t{index}\t{:.9}\t{fen}", output[0]);
    }
    println!("NGN_K4_PROBE_PASS bullet_commit={BULLET_COMMIT} patch={BULLET_PATCH_SHA256} positions={}", fens.len());
}

fn main() {
    let args: Vec<_> = env::args_os().collect();
    if args.len() == 4 && args[1] == "probe" {
        probe(Path::new(&args[2]), Path::new(&args[3]));
        return;
    }
    if args.len() == 4 && args[1] == "compare" {
        compare_checkpoints(Path::new(&args[2]), Path::new(&args[3]));
        return;
    }
    let action = args.get(1).map(|value| value.to_string_lossy());
    let resume = action.as_deref() == Some("resume");
    if (action.as_deref() != Some("start") && !resume) || (!resume && args.len() != 5) || (resume && args.len() != 6) {
        fail(
            "usage: ngn_k4_train start smoke|pilot|pilot-s1|pilot-lr1|probe5m|probe5m-lr1|main|main-m1|main-m1-lr1|main-m1-lr1-32k|archive-a1-e1|archive-a1-e10|archive-a1-e20 FINALIZED_OR_ARCHIVE_MANIFEST NEW_ATTEMPT_DIRECTORY\n       ngn_k4_train resume smoke|pilot|pilot-s1|pilot-lr1|probe5m|probe5m-lr1|main|main-m1|main-m1-lr1|main-m1-lr1-32k|archive-a1-e1|archive-a1-e10|archive-a1-e20 FINALIZED_OR_ARCHIVE_MANIFEST PARENT_CHECKPOINT_RECEIPT NEW_ATTEMPT_DIRECTORY\n       ngn_k4_train probe CHECKPOINT_DIRECTORY FENS_FILE\n       ngn_k4_train compare CHECKPOINT_DIRECTORY CHECKPOINT_DIRECTORY",
        );
    }
    let mode_text = args[2].to_string_lossy();
    let run_mode = mode(&mode_text).unwrap_or_else(|| fail(format!("unknown mode {mode_text}")));
    if INPUT_BUCKETS != (if K8 { 8 } else { 4 }) || !valid_sha(BULLET_PATCH_SHA256) {
        fail("compile-time architecture/patch identity mismatch");
    }
    if run_mode.updates % run_mode.checkpoint_updates != 0 {
        fail("updates must divide evenly into checkpoint intervals");
    }
    let manifest_path = PathBuf::from(&args[3]);
    let (parent_path, output) = if resume {
        (Some(PathBuf::from(&args[4])), PathBuf::from(&args[5]))
    } else {
        (None, PathBuf::from(&args[4]))
    };
    let (input, run_mode) = if run_mode.manifest_mode == "archive" {
        let (input, records) = load_archive_input(&manifest_path);
        (input, Mode { positions: records, ..run_mode })
    } else {
        (load_training_input(&manifest_path, run_mode), run_mode)
    };
    if input.train.bytes != run_mode.positions * 32 {
        fail("training BF size differs from frozen mode");
    }
    let executable =
        file_receipt(&env::current_exe().unwrap_or_else(|error| fail(format!("current executable: {error}"))));
    let validated_parent =
        parent_path.as_ref().map(|parent_path| load_parent_checkpoint(parent_path, run_mode, &input, &executable));
    if output.exists() {
        fail(format!("refusing existing output {}", output.display()));
    }
    fs::create_dir(&output).unwrap_or_else(|error| fail(format!("create attempt {}: {error}", output.display())));
    let candidates = output.join("candidates");
    fs::create_dir(&candidates).unwrap_or_else(|error| fail(format!("create candidates: {error}")));

    let mut trainer = build_trainer();
    let stricter = AdamWParams { max_weight: 0.99, min_weight: -0.99, ..Default::default() };
    trainer.optimiser.set_params_for_weight("l0w", stricter);
    trainer.optimiser.set_params_for_weight("l0f", stricter);

    let (start_updates, mut parent_receipt, resume_load_audit) =
        if let Some((parent, parent_receipt)) = validated_parent {
            let parent_path = parent_path.as_ref().unwrap();
            let checkpoint_dir = parent_path.parent().unwrap();
            trainer.load_from_checkpoint(checkpoint_dir.to_str().unwrap());
            let audit_dir = output.join("resume-load-audit");
            strict_save(&trainer, &audit_dir);
            let audit = checkpoint_files(&audit_dir);
            require_same_checkpoint_state(&parent.files, &audit);
            println!("NGN_K4_RESUME_LOAD_AUDIT_PASS parent={}", parent_path.display());
            (parent.completed_updates, Some(parent_receipt), Some(audit))
        } else {
            (0, None, None)
        };
    let (initial_lr, final_lr) = run_mode.learning_rates();
    let attempt_value = AttemptContract {
        schema: ATTEMPT_SCHEMA.to_string(),
        bullet_commit: BULLET_COMMIT.to_string(),
        bullet_patch_sha256: BULLET_PATCH_SHA256.to_string(),
        mode: run_mode.name.to_string(),
        finalized_manifest: input.manifest.clone(),
        train: input.train.clone(),
        executable: executable.clone(),
        parent_checkpoint: parent_receipt.clone(),
        resume_load_audit,
        start_updates,
        total_updates: run_mode.updates as u64,
        checkpoint_updates: run_mode.checkpoint_updates as u64,
        batch_size: BATCH_SIZE as u64,
        train_positions: run_mode.positions,
        model_seed: MODEL_SEED,
        input_buckets: INPUT_BUCKETS,
        hidden: HIDDEN,
        output_buckets: OUTPUT_BUCKETS,
        score_scale: SCORE_SCALE,
        result_weight_end: WDL_END,
        initial_lr,
        final_lr,
        loader_threads: LOADER_THREADS,
        batch_queue_size: BATCH_QUEUE_SIZE,
    };
    let attempt_path = output.join("attempt.json");
    write_json_new(&attempt_path, &attempt_value);
    let attempt_receipt = file_receipt(&attempt_path);

    if !resume {
        let candidate = candidates.join("candidate-0");
        strict_save(&trainer, &candidate);
        parent_receipt = Some(write_checkpoint_receipt(
            &candidate,
            run_mode,
            &input,
            &attempt_receipt,
            None,
            0,
            ReaderWitness::default(),
        ));
    }

    let total_superbatches = run_mode.updates / run_mode.checkpoint_updates;
    let first_superbatch = start_updates as usize / run_mode.checkpoint_updates + 1;
    let candidate_output = candidates.to_str().unwrap().to_owned();
    for superbatch in first_superbatch..=total_superbatches {
        let completed_before = (superbatch - 1) * run_mode.checkpoint_updates;
        let schedule = TrainingSchedule {
            net_id: format!("ngn-k4-{}", run_mode.name),
            eval_scale: f32::from(SCORE_SCALE),
            steps: TrainingSteps {
                batch_size: BATCH_SIZE,
                batches_per_superbatch: run_mode.checkpoint_updates,
                start_superbatch: superbatch,
                end_superbatch: superbatch,
            },
            wdl_scheduler: wdl::ConstantWDL {
                value: WDL_END * (superbatch - 1) as f32 / (total_superbatches - 1).max(1) as f32,
            },
            lr_scheduler: lr::CosineDecayLR { initial_lr, final_lr, final_superbatch: total_superbatches },
            save_rate: 1,
        };
        let settings = LocalSettings {
            threads: LOADER_THREADS,
            test_set: None,
            output_directory: &candidate_output,
            batch_queue_size: BATCH_QUEUE_SIZE,
        };
        let loader = AuditedSequentialDataLoader::new(
            &input.train.path,
            run_mode.positions,
            BATCH_SIZE,
            completed_before as u64 * BATCH_SIZE as u64,
            run_mode.checkpoint_updates as u64,
        );
        trainer.run(&schedule, &settings, &loader);
        let witness = loader.verified_witness();
        let automatic = candidates.join(format!("ngn-k4-{}-{superbatch}", run_mode.name));
        write_deployed_raw(&trainer, &automatic.join("raw.bin"));
        validate_checkpoint(&automatic);
        let completed_updates = superbatch * run_mode.checkpoint_updates;
        let candidate = candidates.join(format!("candidate-{completed_updates}"));
        if candidate.exists() {
            fail(format!("candidate already exists {}", candidate.display()));
        }
        fs::rename(&automatic, &candidate).unwrap_or_else(|error| {
            fail(format!("rename {} to {}: {error}", automatic.display(), candidate.display()))
        });
        parent_receipt = Some(write_checkpoint_receipt(
            &candidate,
            run_mode,
            &input,
            &attempt_receipt,
            parent_receipt,
            completed_updates as u64,
            witness,
        ));
    }

    let final_checkpoint = parent_receipt.unwrap_or_else(|| fail("training produced no checkpoint receipt"));
    let presentations = run_mode.updates as u64 * BATCH_SIZE as u64;
    let completion = CompletionReceipt {
        schema: COMPLETION_SCHEMA.to_string(),
        attempt_contract: attempt_receipt,
        final_checkpoint,
        completed_updates: run_mode.updates as u64,
        presentations,
        data_cursor: presentations % run_mode.positions,
    };
    write_json_new(&output.join("completion.json"), &completion);
    println!(
        "NGN_K4_TRAIN_PASS bullet_commit={BULLET_COMMIT} patch={BULLET_PATCH_SHA256} mode={} seed={MODEL_SEED} input_buckets={INPUT_BUCKETS} hidden={HIDDEN} output_buckets={OUTPUT_BUCKETS} result_weight_start=0 result_weight_end={WDL_END} score_weight_end={} score_scale={SCORE_SCALE} batch_size={BATCH_SIZE} updates={} presentations={} train_positions={} loader_threads={LOADER_THREADS} queue={BATCH_QUEUE_SIZE} checkpoints={} resumed_from={start_updates}",
        run_mode.name,
        1.0 - WDL_END,
        run_mode.updates,
        presentations,
        run_mode.positions,
        total_superbatches + usize::from(!resume),
    );
}
