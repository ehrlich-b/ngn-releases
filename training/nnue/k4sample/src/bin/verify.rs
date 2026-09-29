use std::{
    cmp::Ordering,
    collections::{BTreeMap, BTreeSet, BinaryHeap, HashMap},
    env,
    fs::File,
    io::{BufRead, BufReader, Read},
    path::{Path, PathBuf},
};

use anyhow::{Context, Result, bail};
use serde::Deserialize;
use sfbinpack::{
    CompressedTrainingDataEntryReader, TrainingDataEntry,
    chess::{
        color::Color, coords::Square, r#move::MoveType, piece::Piece, piecetype::PieceType,
        position::Position,
    },
};
use sha2::{Digest, Sha256};

const CONTRACT: &str = "ngn-k4-sampler-v2";
const OUTPUT_SCHEMA: &str = "ngn-k4-sampler-output-v2";
const SELECTION_SCHEMA: &str = "ngn-k4-selection-v3";
const SCAN_SCHEMA: &str = "ngn-k4-scan-manifest-v1";
const CHUNK_SCHEMA: &str = "ngn-k4-scan-chunk-v1";
const INPUT_SCHEMA: &str = "ngn-k4-label-input-v2";
const SOURCE_BYTES: u64 = 10_809_713_086;
const SOURCE_SHA256: &str = "0d22957b8d4f0f8e6f2913be7b744b2dab5178c3563e0f916312d0d94c28b92b";
const SPLIT_SEED: u64 = 26_092_001;
const SPLIT_DOMAIN: &[u8] = b"ngn-k4-chain-split-v1\0";
const CHAIN_PRIORITY_DOMAIN: &[u8] = b"ngn-k4-chain-priority-v1\0";
const POSITION_PRIORITY_DOMAIN: &[u8] = b"ngn-k4-position-priority-v1\0";
const POSITION_ID_DOMAIN: &[u8] = b"ngn-k4-position-id-v1\0";
const K4_INPUT_DOMAIN: &[u8] = b"ngn-k4-input-v1\0";
const PILOT_ACCEPTED_TARGET: u64 = 1_000_000;
const MAIN_ACCEPTED_TARGET: u64 = 20_000_000;
const HOLDOUT_ACCEPTED_TARGET: u64 = 100_000;
const PILOT_CANDIDATE_TARGET: u64 = 1_250_000;
const MAIN_CANDIDATE_TARGET: u64 = 25_000_000;
const HOLDOUT_CANDIDATE_TARGET: u64 = 125_000;
const RUN_RECORDS: usize = 1_000_000;
const MAX_SCAN_SECONDS: u64 = 14_400;
const MAX_QUARANTINE_CHAINS: usize = 50_000_000;
const KEY_BYTES: usize = 48;
const CHAIN_BYTES: usize = 64;
const CONFLICT_BYTES: usize = 40;
const OWNER_BYTES: usize = 48;
const CANDIDATE_BYTES: usize = 48;
const SELECTED_BYTES: usize = 16;

const KING_BUCKETS: [u16; 64] = [
    1, 1, 0, 0, 0, 0, 1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
    3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
];

#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
#[repr(u8)]
enum Split {
    Train = 0,
    Validation = 1,
    Calibration = 2,
    ReservedTest = 3,
}

impl Split {
    const ALL: [Self; 4] = [
        Self::Train,
        Self::Validation,
        Self::Calibration,
        Self::ReservedTest,
    ];

    fn from_byte(value: u8) -> Result<Self> {
        match value {
            0 => Ok(Self::Train),
            1 => Ok(Self::Validation),
            2 => Ok(Self::Calibration),
            3 => Ok(Self::ReservedTest),
            _ => bail!("invalid split byte {value}"),
        }
    }

    fn name(self) -> &'static str {
        match self {
            Self::Train => "train",
            Self::Validation => "validation",
            Self::Calibration => "calibration",
            Self::ReservedTest => "reserved-test",
        }
    }
}

fn hex(bytes: impl AsRef<[u8]>) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let bytes = bytes.as_ref();
    let mut output = String::with_capacity(bytes.len() * 2);
    for &byte in bytes {
        output.push(DIGITS[(byte >> 4) as usize] as char);
        output.push(DIGITS[(byte & 15) as usize] as char);
    }
    output
}

fn decode_hex_32(value: &str) -> Result<[u8; 32]> {
    if value.len() != 64 {
        bail!("expected 64 lowercase hex characters");
    }
    let mut result = [0u8; 32];
    for (index, pair) in value.as_bytes().chunks_exact(2).enumerate() {
        let digit = |byte: u8| -> Result<u8> {
            match byte {
                b'0'..=b'9' => Ok(byte - b'0'),
                b'a'..=b'f' => Ok(byte - b'a' + 10),
                _ => bail!("invalid lowercase hex byte"),
            }
        };
        result[index] = (digit(pair[0])? << 4) | digit(pair[1])?;
    }
    Ok(result)
}

fn sha256_file(path: &Path) -> Result<(u64, String)> {
    let mut reader = BufReader::new(File::open(path)?);
    let mut digest = Sha256::new();
    let mut buffer = vec![0u8; 1024 * 1024];
    let mut total = 0u64;
    loop {
        let count = reader.read(&mut buffer)?;
        if count == 0 {
            break;
        }
        digest.update(&buffer[..count]);
        total += count as u64;
    }
    Ok((total, hex(digest.finalize())))
}

fn local_identity(domain: &[u8], source: &[u8; 32], chain: u64) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(domain);
    digest.update(SPLIT_SEED.to_le_bytes());
    digest.update(source);
    digest.update(chain.to_le_bytes());
    digest.finalize().into()
}

fn local_split(source: &[u8; 32], chain: u64) -> Split {
    let digest = local_identity(SPLIT_DOMAIN, source, chain);
    match u64::from_le_bytes(digest[..8].try_into().unwrap()) % 100 {
        0..=96 => Split::Train,
        97 => Split::Validation,
        98 => Split::Calibration,
        _ => Split::ReservedTest,
    }
}

fn local_chain_priority(source: &[u8; 32], chain: u64) -> [u8; 32] {
    local_identity(CHAIN_PRIORITY_DOMAIN, source, chain)
}

fn local_position_priority(source: &[u8; 32], chain: u64, entry: u32) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(POSITION_PRIORITY_DOMAIN);
    digest.update(SPLIT_SEED.to_le_bytes());
    digest.update(source);
    digest.update(chain.to_le_bytes());
    digest.update(entry.to_le_bytes());
    digest.finalize().into()
}

fn local_position_id(source: &[u8; 32], chain: u64, entry: u32, key: &[u8; 32]) -> String {
    let mut digest = Sha256::new();
    digest.update(POSITION_ID_DOMAIN);
    digest.update(source);
    digest.update(chain.to_le_bytes());
    digest.update(entry.to_le_bytes());
    digest.update(key);
    hex(digest.finalize())
}

fn local_k4_key(position: &Position) -> Result<[u8; 32]> {
    let white_kings = position.pieces_bb_color(Color::White, PieceType::King);
    let black_kings = position.pieces_bb_color(Color::Black, PieceType::King);
    if white_kings.count() != 1 || black_kings.count() != 1 {
        bail!("K4 verifier requires exactly one king per color");
    }
    let kings = [
        white_kings.bits().trailing_zeros() as usize,
        black_kings.bits().trailing_zeros() as usize,
    ];
    let colors = [Color::White, Color::Black];
    let pieces = [
        PieceType::Pawn,
        PieceType::Knight,
        PieceType::Bishop,
        PieceType::Rook,
        PieceType::Queen,
        PieceType::King,
    ];
    let mut rows = [Vec::new(), Vec::new()];
    let mut piece_count = 0usize;
    for (color_index, color) in colors.into_iter().enumerate() {
        for (piece_index, piece) in pieces.into_iter().enumerate() {
            let mut occupied = position.pieces_bb_color(color, piece).bits();
            piece_count += occupied.count_ones() as usize;
            while occupied != 0 {
                let square = occupied.trailing_zeros() as usize;
                occupied &= occupied - 1;
                for perspective in 0..2 {
                    let mut oriented_square = square;
                    let mut oriented_king = kings[perspective];
                    if perspective == 1 {
                        oriented_square ^= 56;
                        oriented_king ^= 56;
                    }
                    if kings[perspective] % 8 > 3 {
                        oriented_square ^= 7;
                    }
                    rows[perspective].push(
                        KING_BUCKETS[oriented_king] * 768
                            + ((color_index ^ perspective) * 384
                                + piece_index * 64
                                + oriented_square) as u16,
                    );
                }
            }
        }
    }
    if piece_count > 32 {
        bail!("K4 verifier found {piece_count} pieces");
    }
    rows[0].sort_unstable();
    rows[1].sort_unstable();
    if position.side_to_move() == Color::Black {
        rows.swap(0, 1);
    }
    let mut digest = Sha256::new();
    digest.update(K4_INPUT_DOMAIN);
    for perspective in rows {
        digest.update((perspective.len() as u16).to_le_bytes());
        for row in perspective {
            digest.update(row.to_le_bytes());
        }
    }
    digest.update([((piece_count.saturating_sub(2)) / 4).min(7) as u8]);
    Ok(digest.finalize().into())
}

fn sign(value: i32) -> i32 {
    value.signum()
}

fn local_path_clear(position: &Position, from: i32, to: i32, step: i32) -> bool {
    let mut square = from + step;
    while square != to {
        if position.piece_at(Square::new(square as u32)) != Piece::none() {
            return false;
        }
        square += step;
    }
    true
}

fn local_quiet_legal(entry: &TrainingDataEntry) -> bool {
    if entry.mv.mtype() != MoveType::Normal {
        return false;
    }
    let from = entry.mv.from().index() as i32;
    let to = entry.mv.to().index() as i32;
    if !(0..64).contains(&from) || !(0..64).contains(&to) || from == to {
        return false;
    }
    let piece = entry.pos.piece_at(entry.mv.from());
    if piece == Piece::none()
        || piece.color() != entry.pos.side_to_move()
        || entry.pos.piece_at(entry.mv.to()) != Piece::none()
    {
        return false;
    }
    let ff = from % 8;
    let fr = from / 8;
    let tf = to % 8;
    let tr = to / 8;
    let df = tf - ff;
    let dr = tr - fr;
    let pseudo = match piece.piece_type() {
        PieceType::Pawn => {
            let (direction, home) = if piece.color() == Color::White {
                (1, 1)
            } else {
                (-1, 6)
            };
            df == 0
                && (dr == direction
                    || (fr == home
                        && dr == 2 * direction
                        && entry
                            .pos
                            .piece_at(Square::new((from + 8 * direction) as u32))
                            == Piece::none()))
                && tr != 0
                && tr != 7
        }
        PieceType::Knight => matches!((df.abs(), dr.abs()), (1, 2) | (2, 1)),
        PieceType::Bishop => {
            df.abs() == dr.abs() && local_path_clear(&entry.pos, from, to, sign(df) + 8 * sign(dr))
        }
        PieceType::Rook => {
            (df == 0 || dr == 0)
                && local_path_clear(
                    &entry.pos,
                    from,
                    to,
                    if df == 0 { 8 * sign(dr) } else { sign(df) },
                )
        }
        PieceType::Queen => {
            let step = if df.abs() == dr.abs() {
                sign(df) + 8 * sign(dr)
            } else if df == 0 {
                8 * sign(dr)
            } else if dr == 0 {
                sign(df)
            } else {
                0
            };
            step != 0 && local_path_clear(&entry.pos, from, to, step)
        }
        PieceType::King => df.abs() <= 1 && dr.abs() <= 1,
        PieceType::None => false,
    };
    pseudo
        && !entry
            .pos
            .after_move(entry.mv)
            .is_checked(entry.pos.side_to_move())
}

fn local_eligible(entry: &TrainingDataEntry) -> bool {
    entry.ply >= 8
        && entry.pos.occupied().count() <= 32
        && entry
            .pos
            .pieces_bb_color(Color::White, PieceType::King)
            .count()
            == 1
        && entry
            .pos
            .pieces_bb_color(Color::Black, PieceType::King)
            .count()
            == 1
        && !entry.pos.is_checked(entry.pos.side_to_move())
        && local_quiet_legal(entry)
}

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct FileReceipt {
    path: String,
    bytes: u64,
    sha256: String,
}

fn verify_receipt(item: &FileReceipt) -> Result<()> {
    let (bytes, sha) = sha256_file(Path::new(&item.path))?;
    if bytes != item.bytes || sha != item.sha256 {
        bail!("receipt mismatch {}", item.path);
    }
    Ok(())
}

#[allow(dead_code)]
#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ChunkReceipt {
    schema: String,
    chunk: u32,
    source_position_start: u64,
    source_position_end: u64,
    chain_start: u64,
    chain_end: u64,
    decoded_positions: u64,
    eligible_positions: u64,
    rejection_counts: BTreeMap<String, u64>,
    keys: FileReceipt,
    chains: FileReceipt,
}

#[allow(dead_code)]
#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ScanManifest {
    schema: String,
    contract_version: String,
    state: String,
    source_revision: String,
    source: FileReceipt,
    split_seed: u64,
    split_domain_hex: String,
    chain_priority_domain_hex: String,
    position_priority_domain_hex: String,
    k4_input_domain_hex: String,
    sfbinpack_version: String,
    exact_argv: Vec<String>,
    run_records: usize,
    max_scan_seconds: u64,
    chunks: Vec<ChunkReceipt>,
    decoded_positions: u64,
    complete_chains: u64,
    eligible_positions: u64,
    rejection_counts: BTreeMap<String, u64>,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct SelectionCounts {
    historical_holdout_keys: u64,
    cross_split_keys: u64,
    historical_conflict_keys: u64,
    quarantined_chains: u64,
    unique_owner_positions: u64,
    selected_owner_positions: u64,
    candidate_chains: [u64; 4],
    pilot_candidate_positions: u64,
    main_candidate_positions: u64,
    validation_candidate_positions: u64,
    calibration_candidate_positions: u64,
    reserved_test_candidate_positions: u64,
    selected_chains: u64,
}

#[allow(dead_code)]
#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct SelectionManifest {
    schema: String,
    contract_version: String,
    state: String,
    exact_argv: Vec<String>,
    source: FileReceipt,
    scan_manifest: FileReceipt,
    historical_positions: FileReceipt,
    historical_keys: FileReceipt,
    conflicts: FileReceipt,
    quarantine_chains: FileReceipt,
    owners_by_chain: FileReceipt,
    owners_by_key: FileReceipt,
    selected_chains: FileReceipt,
    candidate_files: Vec<FileReceipt>,
    split_seed: u64,
    split_domain_hex: String,
    chain_priority_domain_hex: String,
    position_priority_domain_hex: String,
    position_id_domain_hex: String,
    k4_input_domain_hex: String,
    pilot_candidate_target: u64,
    main_candidate_target: u64,
    holdout_candidate_target: u64,
    pilot_accepted_target: u64,
    main_accepted_target: u64,
    holdout_accepted_target: u64,
    counts: SelectionCounts,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ShardReceipt {
    stage: String,
    split: String,
    shard_id: String,
    records: usize,
    file: FileReceipt,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct OutputManifest {
    schema: String,
    contract_version: String,
    state: String,
    selection: FileReceipt,
    shards: Vec<ShardReceipt>,
    pilot_candidate_positions: u64,
    main_expansion_candidate_positions: u64,
    total_train_candidate_positions: u64,
    validation_candidate_positions: u64,
    calibration_candidate_positions: u64,
    reserved_test_candidate_positions: u64,
    pilot_accepted_target: u64,
    main_accepted_target: u64,
    holdout_accepted_target: u64,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct InputHeader {
    r#type: String,
    schema: String,
    shard_id: String,
    split: String,
    source_manifest_sha256: String,
    record_count: usize,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
struct InputPosition {
    r#type: String,
    id: String,
    fen: String,
    source_move: String,
    encoded_chain: u64,
    chain_entry: u32,
    source_position: u64,
    k4_input_sha256: String,
    source_archive_sha256: String,
    source_manifest_ref: String,
}

struct RawReader {
    reader: BufReader<File>,
    path: PathBuf,
    width: usize,
}

impl RawReader {
    fn open(path: impl AsRef<Path>, width: usize) -> Result<Self> {
        let path = path.as_ref().to_path_buf();
        let file = File::open(&path)?;
        if file.metadata()?.len() % width as u64 != 0 {
            bail!("{} has a partial {width}-byte record", path.display());
        }
        Ok(Self {
            reader: BufReader::new(file),
            path,
            width,
        })
    }

    fn next(&mut self) -> Result<Option<Vec<u8>>> {
        let mut bytes = vec![0u8; self.width];
        let mut offset = 0;
        while offset < bytes.len() {
            let count = self.reader.read(&mut bytes[offset..])?;
            if count == 0 {
                if offset == 0 {
                    return Ok(None);
                }
                bail!("truncated fixed record in {}", self.path.display());
            }
            offset += count;
        }
        Ok(Some(bytes))
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
struct KeyRecord {
    key: [u8; 32],
    chain: u64,
    entry: u32,
    split: Split,
}

impl Ord for KeyRecord {
    fn cmp(&self, other: &Self) -> Ordering {
        (self.key, self.split, self.chain, self.entry).cmp(&(
            other.key,
            other.split,
            other.chain,
            other.entry,
        ))
    }
}

impl PartialOrd for KeyRecord {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

fn decode_key(bytes: &[u8]) -> Result<KeyRecord> {
    if bytes.len() != KEY_BYTES || bytes[45..].iter().any(|byte| *byte != 0) {
        bail!("invalid key record");
    }
    Ok(KeyRecord {
        key: bytes[..32].try_into().unwrap(),
        chain: u64::from_le_bytes(bytes[32..40].try_into().unwrap()),
        entry: u32::from_le_bytes(bytes[40..44].try_into().unwrap()),
        split: Split::from_byte(bytes[44])?,
    })
}

struct HeapKey {
    record: KeyRecord,
    run: usize,
}

impl Ord for HeapKey {
    fn cmp(&self, other: &Self) -> Ordering {
        other
            .record
            .cmp(&self.record)
            .then_with(|| other.run.cmp(&self.run))
    }
}

impl PartialOrd for HeapKey {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl PartialEq for HeapKey {
    fn eq(&self, other: &Self) -> bool {
        self.record == other.record && self.run == other.run
    }
}

impl Eq for HeapKey {}

struct KeyMerger {
    readers: Vec<RawReader>,
    heap: BinaryHeap<HeapKey>,
    previous: Option<KeyRecord>,
}

impl KeyMerger {
    fn open(paths: &[PathBuf]) -> Result<Self> {
        let mut readers = paths
            .iter()
            .map(|path| RawReader::open(path, KEY_BYTES))
            .collect::<Result<Vec<_>>>()?;
        let mut heap = BinaryHeap::new();
        for (run, reader) in readers.iter_mut().enumerate() {
            if let Some(bytes) = reader.next()? {
                heap.push(HeapKey {
                    record: decode_key(&bytes)?,
                    run,
                });
            }
        }
        Ok(Self {
            readers,
            heap,
            previous: None,
        })
    }

    fn next(&mut self) -> Result<Option<KeyRecord>> {
        let Some(item) = self.heap.pop() else {
            return Ok(None);
        };
        if self.previous.is_some_and(|prior| item.record <= prior) {
            bail!("key runs are not globally strict-sorted");
        }
        if let Some(bytes) = self.readers[item.run].next()? {
            let next = decode_key(&bytes)?;
            if next <= item.record {
                bail!("key run {} is not strict-sorted", item.run);
            }
            self.heap.push(HeapKey {
                record: next,
                run: item.run,
            });
        }
        self.previous = Some(item.record);
        Ok(Some(item.record))
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
struct Conflict {
    key: [u8; 32],
    cross: bool,
    historical: bool,
}

fn decode_conflict(bytes: &[u8]) -> Result<Conflict> {
    if bytes.len() != CONFLICT_BYTES
        || bytes[32] > 1
        || bytes[33] > 1
        || bytes[34..].iter().any(|byte| *byte != 0)
    {
        bail!("invalid conflict record");
    }
    Ok(Conflict {
        key: bytes[..32].try_into().unwrap(),
        cross: bytes[32] != 0,
        historical: bytes[33] != 0,
    })
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
struct Owner {
    chain: u64,
    entry: u32,
    split: Split,
    key: [u8; 32],
}

fn decode_owner(bytes: &[u8]) -> Result<Owner> {
    if bytes.len() != OWNER_BYTES || bytes[13..16].iter().any(|byte| *byte != 0) {
        bail!("invalid owner record");
    }
    Ok(Owner {
        chain: u64::from_le_bytes(bytes[..8].try_into().unwrap()),
        entry: u32::from_le_bytes(bytes[8..12].try_into().unwrap()),
        split: Split::from_byte(bytes[12])?,
        key: bytes[16..].try_into().unwrap(),
    })
}

fn owner_bytes(owner: Owner) -> [u8; OWNER_BYTES] {
    let mut bytes = [0u8; OWNER_BYTES];
    bytes[..8].copy_from_slice(&owner.chain.to_le_bytes());
    bytes[8..12].copy_from_slice(&owner.entry.to_le_bytes());
    bytes[12] = owner.split as u8;
    bytes[16..].copy_from_slice(&owner.key);
    bytes
}

#[derive(Clone, Copy)]
struct ChainAudit {
    split: Split,
    priority: [u8; 32],
    source_start: u64,
    decoded: u32,
    eligible: u32,
}

fn decode_chain(bytes: &[u8], expected: u64, source: &[u8; 32]) -> Result<ChainAudit> {
    if bytes.len() != CHAIN_BYTES || bytes[25..32].iter().any(|byte| *byte != 0) {
        bail!("invalid chain record");
    }
    let chain = u64::from_le_bytes(bytes[..8].try_into().unwrap());
    let split = Split::from_byte(bytes[24])?;
    let priority: [u8; 32] = bytes[32..].try_into().unwrap();
    if chain != expected
        || split != local_split(source, chain)
        || priority != local_chain_priority(source, chain)
    {
        bail!("chain identity mismatch at ordinal {expected}");
    }
    Ok(ChainAudit {
        split,
        priority,
        source_start: u64::from_le_bytes(bytes[8..16].try_into().unwrap()),
        decoded: u32::from_le_bytes(bytes[16..20].try_into().unwrap()),
        eligible: u32::from_le_bytes(bytes[20..24].try_into().unwrap()),
    })
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
struct Candidate {
    priority: [u8; 32],
    chain: u64,
    unique: u32,
    split: Split,
}

fn decode_candidate(bytes: &[u8]) -> Result<Candidate> {
    if bytes.len() != CANDIDATE_BYTES || bytes[45..].iter().any(|byte| *byte != 0) {
        bail!("invalid candidate record");
    }
    Ok(Candidate {
        priority: bytes[..32].try_into().unwrap(),
        chain: u64::from_le_bytes(bytes[32..40].try_into().unwrap()),
        unique: u32::from_le_bytes(bytes[40..44].try_into().unwrap()),
        split: Split::from_byte(bytes[44])?,
    })
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
struct Selected {
    chain: u64,
    split: Split,
    pilot: bool,
    unique: u32,
}

fn decode_selected(bytes: &[u8]) -> Result<Selected> {
    if bytes.len() != SELECTED_BYTES || bytes[9] > 1 || bytes[10..12].iter().any(|byte| *byte != 0)
    {
        bail!("invalid selected-chain record");
    }
    Ok(Selected {
        chain: u64::from_le_bytes(bytes[..8].try_into().unwrap()),
        split: Split::from_byte(bytes[8])?,
        pilot: bytes[9] != 0,
        unique: u32::from_le_bytes(bytes[12..16].try_into().unwrap()),
    })
}

#[derive(Default, Eq, PartialEq)]
struct MultisetDigest {
    count: u64,
    xor: [u8; 32],
    sum: [u8; 32],
}

impl MultisetDigest {
    fn add(&mut self, bytes: &[u8]) {
        let mut digest = Sha256::new();
        digest.update(b"ngn-k4-owner-multiset-v1\0");
        digest.update(bytes);
        let value: [u8; 32] = digest.finalize().into();
        self.count += 1;
        let mut carry = 0u16;
        for (index, byte) in value.iter().enumerate() {
            self.xor[index] ^= *byte;
            let total = u16::from(self.sum[index]) + u16::from(*byte) + carry;
            self.sum[index] = total as u8;
            carry = total >> 8;
        }
    }
}

#[derive(Deserialize)]
struct HistoricalPosition {
    source_fen: String,
    tentative_split: String,
    eligible: bool,
}

fn verify_historical(selection: &SelectionManifest) -> Result<BTreeSet<[u8; 32]>> {
    let mut expected = BTreeSet::new();
    let reader = BufReader::new(File::open(&selection.historical_positions.path)?);
    for (index, line) in reader.lines().enumerate() {
        let item: HistoricalPosition = serde_json::from_str(&line?)
            .with_context(|| format!("historical line {}", index + 1))?;
        if item.eligible
            && (item.tentative_split == "validation" || item.tentative_split == "sealed-test")
        {
            let position = Position::from_fen(&item.source_fen)
                .map_err(|error| anyhow::anyhow!("historical FEN: {error:?}"))?;
            expected.insert(local_k4_key(&position)?);
        }
    }
    let mut actual = RawReader::open(&selection.historical_keys.path, 32)?;
    for key in &expected {
        if actual.next()?.as_deref() != Some(key.as_slice()) {
            bail!("historical K4 key artifact differs from independent reconstruction");
        }
    }
    if actual.next()?.is_some() || expected.len() as u64 != selection.counts.historical_holdout_keys
    {
        bail!("historical K4 key artifact has an unexpected tail/count");
    }
    Ok(expected)
}

fn key_paths(scan: &ScanManifest) -> Vec<PathBuf> {
    scan.chunks
        .iter()
        .map(|chunk| PathBuf::from(&chunk.keys.path))
        .collect()
}

fn verify_conflicts(
    scan: &ScanManifest,
    selection: &SelectionManifest,
    historical: &BTreeSet<[u8; 32]>,
) -> Result<BTreeSet<u64>> {
    let mut keys = KeyMerger::open(&key_paths(scan))?;
    let mut conflicts = RawReader::open(&selection.conflicts.path, CONFLICT_BYTES)?;
    let mut quarantined = BTreeSet::new();
    let mut current_key = None;
    let mut members: BTreeSet<(u64, Split)> = BTreeSet::new();
    let mut cross_count = 0u64;
    let mut historical_count = 0u64;
    let mut finish = |key: [u8; 32], members: &mut BTreeSet<(u64, Split)>| -> Result<()> {
        let split_mask = members
            .iter()
            .fold(0u8, |mask, (_, split)| mask | 1 << *split as u8);
        let expected = Conflict {
            key,
            cross: split_mask.count_ones() > 1,
            historical: historical.contains(&key),
        };
        if expected.cross || expected.historical {
            let actual = conflicts.next()?.context("conflict artifact ended early")?;
            if decode_conflict(&actual)? != expected {
                bail!("conflict artifact differs at key {}", hex(key));
            }
            cross_count += u64::from(expected.cross);
            historical_count += u64::from(expected.historical);
            for &(chain, split) in members.iter() {
                if expected.cross || (expected.historical && split == Split::Train) {
                    quarantined.insert(chain);
                    if quarantined.len() > MAX_QUARANTINE_CHAINS {
                        bail!("quarantine set exceeds verifier memory bound");
                    }
                }
            }
        }
        members.clear();
        Ok(())
    };
    while let Some(record) = keys.next()? {
        if current_key.is_some_and(|key| key != record.key) {
            finish(current_key.unwrap(), &mut members)?;
        }
        current_key = Some(record.key);
        members.insert((record.chain, record.split));
    }
    if let Some(key) = current_key {
        finish(key, &mut members)?;
    }
    if conflicts.next()?.is_some()
        || cross_count != selection.counts.cross_split_keys
        || historical_count != selection.counts.historical_conflict_keys
        || quarantined.len() as u64 != selection.counts.quarantined_chains
    {
        bail!("conflict/quarantine aggregate mismatch");
    }
    let mut actual = RawReader::open(&selection.quarantine_chains.path, 8)?;
    for chain in &quarantined {
        let bytes = actual.next()?.context("quarantine artifact ended early")?;
        if u64::from_le_bytes(bytes.try_into().unwrap()) != *chain {
            bail!("quarantine artifact differs at chain {chain}");
        }
    }
    if actual.next()?.is_some() {
        bail!("quarantine artifact has a tail");
    }
    Ok(quarantined)
}

fn load_selected_mask(selection: &SelectionManifest, chain_count: usize) -> Result<Vec<bool>> {
    let mut reader = RawReader::open(&selection.selected_chains.path, SELECTED_BYTES)?;
    let mut selected = vec![false; chain_count];
    let mut previous = None;
    let mut chains = 0u64;
    let mut positions = 0u64;
    while let Some(bytes) = reader.next()? {
        let item = decode_selected(&bytes)?;
        let index: usize = item.chain.try_into()?;
        let bit = selected
            .get_mut(index)
            .context("selected chain exceeds scan chain count")?;
        if *bit || previous.is_some_and(|prior| item.chain <= prior) || item.unique == 0 {
            bail!("selected-chain mask order/identity mismatch");
        }
        *bit = true;
        previous = Some(item.chain);
        chains += 1;
        positions += u64::from(item.unique);
    }
    if chains != selection.counts.selected_chains
        || positions != selection.counts.selected_owner_positions
    {
        bail!("selected-chain mask aggregate mismatch");
    }
    Ok(selected)
}

fn verify_owners_by_key(
    scan: &ScanManifest,
    selection: &SelectionManifest,
    source: &[u8; 32],
    quarantined: &BTreeSet<u64>,
    selected: &[bool],
) -> Result<(MultisetDigest, Vec<u32>)> {
    let mut keys = KeyMerger::open(&key_paths(scan))?;
    let mut actual = RawReader::open(&selection.owners_by_key.path, OWNER_BYTES)?;
    let mut current_key = None;
    let mut owner: Option<(KeyRecord, [u8; 32], [u8; 32])> = None;
    let mut digest = MultisetDigest::default();
    let mut owner_counts = vec![0u32; selected.len()];
    let mut total_owners = 0u64;
    let mut finish = |owner: &mut Option<(KeyRecord, [u8; 32], [u8; 32])>| -> Result<()> {
        if let Some((record, _, _)) = owner.take() {
            let expected = Owner {
                chain: record.chain,
                entry: record.entry,
                split: record.split,
                key: record.key,
            };
            let index: usize = record.chain.try_into()?;
            let count = owner_counts
                .get_mut(index)
                .context("owner references missing chain")?;
            *count = count.checked_add(1).context("owner count overflow")?;
            total_owners += 1;
            if selected[index] {
                let bytes = actual.next()?.context("owners-by-key ended early")?;
                if decode_owner(&bytes)? != expected {
                    bail!("owners-by-key differs at K4 key {}", hex(record.key));
                }
                digest.add(&owner_bytes(expected));
            }
        }
        Ok(())
    };
    while let Some(record) = keys.next()? {
        if current_key.is_some_and(|key| key != record.key) {
            finish(&mut owner)?;
        }
        current_key = Some(record.key);
        if quarantined.contains(&record.chain) {
            continue;
        }
        if owner
            .as_ref()
            .is_some_and(|(prior, _, _)| prior.split != record.split)
        {
            bail!("nonquarantined key crosses splits");
        }
        let chain_priority = local_chain_priority(source, record.chain);
        let position_priority = local_position_priority(source, record.chain, record.entry);
        let replace = owner
            .as_ref()
            .is_none_or(|(prior, prior_chain, prior_position)| {
                (
                    chain_priority,
                    position_priority,
                    record.chain,
                    record.entry,
                ) < (*prior_chain, *prior_position, prior.chain, prior.entry)
            });
        if replace {
            owner = Some((record, chain_priority, position_priority));
        }
    }
    finish(&mut owner)?;
    if actual.next()?.is_some()
        || total_owners != selection.counts.unique_owner_positions
        || digest.count != selection.counts.selected_owner_positions
    {
        bail!("owners-by-key has an unexpected tail/count");
    }
    Ok((digest, owner_counts))
}

fn load_chains(scan: &ScanManifest, source: &[u8; 32]) -> Result<Vec<ChainAudit>> {
    let capacity: usize = scan
        .complete_chains
        .try_into()
        .context("chain count exceeds address space")?;
    let mut chains = Vec::with_capacity(capacity);
    for chunk in &scan.chunks {
        let mut reader = RawReader::open(&chunk.chains.path, CHAIN_BYTES)?;
        while let Some(bytes) = reader.next()? {
            chains.push(decode_chain(&bytes, chains.len() as u64, source)?);
        }
    }
    if chains.len() != capacity {
        bail!("chain metadata count mismatch");
    }
    Ok(chains)
}

fn verify_owner_chain_order(
    selection: &SelectionManifest,
    chains: &[ChainAudit],
    selected: &[bool],
    owner_counts: &[u32],
    expected_digest: &MultisetDigest,
) -> Result<()> {
    let mut reader = RawReader::open(&selection.owners_by_chain.path, OWNER_BYTES)?;
    let mut previous = None;
    let mut current_chain = None;
    let mut current_count = 0u32;
    let mut digest = MultisetDigest::default();
    while let Some(bytes) = reader.next()? {
        let owner = decode_owner(&bytes)?;
        let index: usize = owner.chain.try_into()?;
        if index >= chains.len()
            || !selected[index]
            || owner.split != chains[index].split
            || previous.is_some_and(|prior: (u64, u32)| (owner.chain, owner.entry) <= prior)
        {
            bail!("owners-by-chain order/identity mismatch");
        }
        if current_chain != Some(owner.chain) {
            if let Some(chain) = current_chain {
                if current_count != owner_counts[chain as usize] {
                    bail!("owners-by-chain selected count mismatch at chain {chain}");
                }
            }
            current_chain = Some(owner.chain);
            current_count = 0;
        }
        current_count = current_count
            .checked_add(1)
            .context("owner count overflow")?;
        previous = Some((owner.chain, owner.entry));
        digest.add(&bytes);
    }
    if let Some(chain) = current_chain {
        if current_count != owner_counts[chain as usize] {
            bail!("owners-by-chain selected count mismatch at chain {chain}");
        }
    }
    if &digest != expected_digest {
        bail!("owners-by-chain is not the same multiset as independently checked owners-by-key");
    }
    Ok(())
}

fn verify_candidates_and_selection(
    selection: &SelectionManifest,
    chains: &[ChainAudit],
    owner_counts: &[u32],
) -> Result<Vec<Option<Selected>>> {
    if selection.candidate_files.len() != 4 {
        bail!("selection must contain four candidate files");
    }
    let mut seen = vec![false; chains.len()];
    let mut expected_selected = vec![None; chains.len()];
    let mut selected_counts = [0u64; 5];
    let mut candidate_counts = [0u64; 4];
    for split in Split::ALL {
        let mut reader = RawReader::open(
            &selection.candidate_files[split as usize].path,
            CANDIDATE_BYTES,
        )?;
        let mut previous: Option<([u8; 32], u64)> = None;
        while let Some(bytes) = reader.next()? {
            let candidate = decode_candidate(&bytes)?;
            let chain_index: usize = candidate.chain.try_into()?;
            if candidate.split != split
                || chain_index >= chains.len()
                || seen[chain_index]
                || candidate.priority != chains[chain_index].priority
                || candidate.unique == 0
                || candidate.unique != owner_counts[chain_index]
                || previous.is_some_and(|prior| (candidate.priority, candidate.chain) <= prior)
            {
                bail!("candidate artifact mismatch in split {}", split.name());
            }
            previous = Some((candidate.priority, candidate.chain));
            seen[chain_index] = true;
            candidate_counts[split as usize] += 1;
            let count_index = match split {
                Split::Train => 1,
                Split::Validation => 2,
                Split::Calibration => 3,
                Split::ReservedTest => 4,
            };
            let target = if split == Split::Train {
                MAIN_CANDIDATE_TARGET
            } else {
                HOLDOUT_CANDIDATE_TARGET
            };
            if selected_counts[count_index] < target {
                let pilot = split == Split::Train && selected_counts[0] < PILOT_CANDIDATE_TARGET;
                if pilot {
                    selected_counts[0] += u64::from(candidate.unique);
                }
                selected_counts[count_index] += u64::from(candidate.unique);
                expected_selected[chain_index] = Some(Selected {
                    chain: candidate.chain,
                    split,
                    pilot,
                    unique: candidate.unique,
                });
            }
        }
    }
    if owner_counts
        .iter()
        .zip(&seen)
        .any(|(&count, &seen)| (count > 0) != seen)
        || candidate_counts != selection.counts.candidate_chains
    {
        bail!("candidate coverage/count mismatch");
    }
    let manifest_counts = [
        selection.counts.pilot_candidate_positions,
        selection.counts.main_candidate_positions,
        selection.counts.validation_candidate_positions,
        selection.counts.calibration_candidate_positions,
        selection.counts.reserved_test_candidate_positions,
    ];
    if selected_counts != manifest_counts {
        bail!("selected position counts differ: {selected_counts:?}/{manifest_counts:?}");
    }
    let mut actual = RawReader::open(&selection.selected_chains.path, SELECTED_BYTES)?;
    let mut selected_total = 0u64;
    for expected in expected_selected.iter().flatten() {
        let bytes = actual
            .next()?
            .context("selected-chain artifact ended early")?;
        if decode_selected(&bytes)? != *expected {
            bail!(
                "selected-chain artifact differs at chain {}",
                expected.chain
            );
        }
        selected_total += 1;
    }
    if actual.next()?.is_some() || selected_total != selection.counts.selected_chains {
        bail!("selected-chain artifact has a tail/count mismatch");
    }
    Ok(expected_selected)
}

struct SeriesReader {
    receipts: Vec<ShardReceipt>,
    next_shard: usize,
    current: Option<std::io::Lines<BufReader<File>>>,
    remaining: usize,
    stage: String,
    split: Split,
    selection_sha: String,
    total: u64,
}

impl SeriesReader {
    fn new(receipts: Vec<ShardReceipt>, stage: &str, split: Split, selection_sha: &str) -> Self {
        Self {
            receipts,
            next_shard: 0,
            current: None,
            remaining: 0,
            stage: stage.to_string(),
            split,
            selection_sha: selection_sha.to_string(),
            total: 0,
        }
    }

    fn open_next(&mut self) -> Result<bool> {
        if self.next_shard >= self.receipts.len() {
            return Ok(false);
        }
        let receipt = &self.receipts[self.next_shard];
        let want_id = format!(
            "{}-{}-{:06}",
            self.stage,
            self.split.name(),
            self.next_shard
        );
        if receipt.stage != self.stage
            || receipt.split != self.split.name()
            || receipt.shard_id != want_id
            || receipt.records == 0
            || receipt.records > 100_000
        {
            bail!("invalid shard series receipt {want_id}");
        }
        let mut lines = BufReader::new(File::open(&receipt.file.path)?).lines();
        let header_line = lines.next().context("shard lacks header")??;
        let header: InputHeader = serde_json::from_str(&header_line)?;
        if header.r#type != "header"
            || header.schema != INPUT_SCHEMA
            || header.shard_id != want_id
            || header.split != self.split.name()
            || header.source_manifest_sha256 != self.selection_sha
            || header.record_count != receipt.records
        {
            bail!("invalid shard header {want_id}");
        }
        self.remaining = receipt.records;
        self.current = Some(lines);
        self.next_shard += 1;
        Ok(true)
    }

    fn next(&mut self) -> Result<Option<InputPosition>> {
        loop {
            if self.current.is_none() && !self.open_next()? {
                return Ok(None);
            }
            if self.remaining == 0 {
                if self.current.as_mut().unwrap().next().is_some() {
                    bail!("shard contains more records than declared");
                }
                self.current = None;
                continue;
            }
            let line = self
                .current
                .as_mut()
                .unwrap()
                .next()
                .context("shard ended before declared count")??;
            self.remaining -= 1;
            self.total += 1;
            return Ok(Some(serde_json::from_str(&line)?));
        }
    }
}

fn shard_series(output: &OutputManifest, selection_sha: &str) -> Result<Vec<SeriesReader>> {
    let specifications = [
        ("pilot", Split::Train),
        ("main-expansion", Split::Train),
        ("fixed", Split::Validation),
        ("fixed", Split::Calibration),
        ("fixed", Split::ReservedTest),
    ];
    let mut result = Vec::new();
    let mut used = BTreeSet::new();
    for (stage, split) in specifications {
        let mut receipts = Vec::new();
        for (index, item) in output.shards.iter().enumerate() {
            if item.stage == stage && item.split == split.name() {
                receipts.push(item.clone());
                used.insert(index);
            }
        }
        result.push(SeriesReader::new(receipts, stage, split, selection_sha));
    }
    if used.len() != output.shards.len() {
        bail!("output manifest contains an unknown/duplicate shard series");
    }
    Ok(result)
}

fn verify_source_and_shards(
    source_path: &Path,
    scan: &ScanManifest,
    selection: &SelectionManifest,
    output: &OutputManifest,
    chains: &[ChainAudit],
    selected: &[Option<Selected>],
) -> Result<()> {
    let source_sha = decode_hex_32(SOURCE_SHA256)?;
    let mut series = shard_series(output, &output.selection.sha256)?;
    let mut owners = RawReader::open(&selection.owners_by_chain.path, OWNER_BYTES)?;
    let mut owner = owners
        .next()?
        .map(|bytes| decode_owner(&bytes))
        .transpose()?;
    let mut chunk_keys: HashMap<(u64, u32), ([u8; 32], Split)> = HashMap::new();
    let mut chunk_index = 0usize;
    let mut file = CompressedTrainingDataEntryReader::new(File::open(source_path)?)
        .map_err(|error| anyhow::anyhow!("open source reader: {error:?}"))?;
    let mut source_position = 0u64;
    let mut chain = 0u64;
    let mut chain_start = 0u64;
    let mut chain_eligible = 0u32;
    let mut owner_count = 0u32;
    while file.has_next() {
        if chunk_index >= scan.chunks.len() {
            bail!("source has more chains than scan chunks");
        }
        let chunk = &scan.chunks[chunk_index];
        if chain == chunk.chain_start && source_position == chunk.source_position_start {
            chunk_keys.clear();
            let mut reader = RawReader::open(&chunk.keys.path, KEY_BYTES)?;
            while let Some(bytes) = reader.next()? {
                let record = decode_key(&bytes)?;
                if record.chain < chunk.chain_start
                    || record.chain >= chunk.chain_end
                    || chunk_keys
                        .insert((record.chain, record.entry), (record.key, record.split))
                        .is_some()
                {
                    bail!("scan chunk key membership mismatch");
                }
            }
        }
        let entry = file.next();
        let continuation = file.is_next_entry_continuation();
        let entry_ordinal: u32 = (source_position - chain_start).try_into()?;
        let eligible = local_eligible(&entry);
        let key = if eligible {
            chain_eligible += 1;
            let key = local_k4_key(&entry.pos)?;
            let actual = chunk_keys
                .remove(&(chain, entry_ordinal))
                .context("eligible source entry missing from scan key chunk")?;
            if actual != (key, local_split(&source_sha, chain)) {
                bail!("scan key differs at chain {chain} entry {entry_ordinal}");
            }
            Some(key)
        } else {
            if chunk_keys.contains_key(&(chain, entry_ordinal)) {
                bail!("ineligible source entry appears in scan key chunk");
            }
            None
        };
        if owner.is_some_and(|item| item.chain == chain && item.entry == entry_ordinal) {
            let item = owner.unwrap();
            let key = key.context("owner points at an ineligible source entry")?;
            if item.key != key || item.split != local_split(&source_sha, chain) {
                bail!("owner differs from source at chain {chain} entry {entry_ordinal}");
            }
            owner_count += 1;
            if let Some(selected_chain) = selected[chain as usize] {
                let expected = InputPosition {
                    r#type: "position".to_string(),
                    id: local_position_id(&source_sha, chain, entry_ordinal, &key),
                    fen: entry
                        .pos
                        .fen()
                        .map_err(|error| anyhow::anyhow!("source FEN: {error:?}"))?,
                    source_move: entry.mv.as_uci(),
                    encoded_chain: chain,
                    chain_entry: entry_ordinal,
                    source_position,
                    k4_input_sha256: hex(key),
                    source_archive_sha256: SOURCE_SHA256.to_string(),
                    source_manifest_ref: output.selection.path.clone(),
                };
                let series_index = match selected_chain.split {
                    Split::Train if selected_chain.pilot => 0,
                    Split::Train => 1,
                    Split::Validation => 2,
                    Split::Calibration => 3,
                    Split::ReservedTest => 4,
                };
                let actual = series[series_index]
                    .next()?
                    .context("selected shard series ended early")?;
                if actual != expected {
                    bail!("shard differs at chain {chain} entry {entry_ordinal}");
                }
            }
            owner = owners
                .next()?
                .map(|bytes| decode_owner(&bytes))
                .transpose()?;
        }
        source_position += 1;
        if !continuation {
            let audit = chains
                .get(chain as usize)
                .context("source has excess chain")?;
            let decoded: u32 = (source_position - chain_start).try_into()?;
            if audit.source_start != chain_start
                || audit.decoded != decoded
                || audit.eligible != chain_eligible
            {
                bail!("chain metadata/source mismatch at chain {chain}");
            }
            if let Some(selected_chain) = selected[chain as usize] {
                if owner_count != selected_chain.unique {
                    bail!("selected owner count mismatch at chain {chain}");
                }
            }
            owner_count = 0;
            chain_eligible = 0;
            chain += 1;
            chain_start = source_position;
            if chain == chunk.chain_end {
                if source_position != chunk.source_position_end || !chunk_keys.is_empty() {
                    bail!("scan chunk/source boundary mismatch");
                }
                chunk_index += 1;
            }
        }
    }
    if owner.is_some()
        || source_position != scan.decoded_positions
        || chain != scan.complete_chains
        || chunk_index != scan.chunks.len()
    {
        bail!("source replay ended with unmatched scan/owner state");
    }
    for item in &mut series {
        if item.next()?.is_some() {
            bail!(
                "shard series {} {} has a tail",
                item.stage,
                item.split.name()
            );
        }
    }
    let totals = [
        series[0].total,
        series[1].total,
        series[2].total,
        series[3].total,
        series[4].total,
    ];
    let expected = [
        output.pilot_candidate_positions,
        output.main_expansion_candidate_positions,
        output.validation_candidate_positions,
        output.calibration_candidate_positions,
        output.reserved_test_candidate_positions,
    ];
    if totals != expected {
        bail!("shard totals differ: {totals:?}/{expected:?}");
    }
    Ok(())
}

fn verify_contracts(
    source_path: &Path,
    output: &OutputManifest,
    selection: &SelectionManifest,
    scan: &ScanManifest,
) -> Result<()> {
    if output.schema != OUTPUT_SCHEMA
        || output.contract_version != CONTRACT
        || output.state != "COMPLETE"
        || selection.schema != SELECTION_SCHEMA
        || selection.contract_version != CONTRACT
        || selection.state != "SELECTED"
        || scan.schema != SCAN_SCHEMA
        || scan.contract_version != CONTRACT
        || scan.state != "COMPLETE"
        || scan.source.bytes != SOURCE_BYTES
        || scan.source.sha256 != SOURCE_SHA256
        || selection.source.bytes != SOURCE_BYTES
        || selection.source.sha256 != SOURCE_SHA256
        || scan.split_seed != SPLIT_SEED
        || selection.split_seed != SPLIT_SEED
        || scan.split_domain_hex != hex(SPLIT_DOMAIN)
        || selection.split_domain_hex != hex(SPLIT_DOMAIN)
        || scan.chain_priority_domain_hex != hex(CHAIN_PRIORITY_DOMAIN)
        || selection.chain_priority_domain_hex != hex(CHAIN_PRIORITY_DOMAIN)
        || scan.position_priority_domain_hex != hex(POSITION_PRIORITY_DOMAIN)
        || selection.position_priority_domain_hex != hex(POSITION_PRIORITY_DOMAIN)
        || selection.position_id_domain_hex != hex(POSITION_ID_DOMAIN)
        || scan.k4_input_domain_hex != hex(K4_INPUT_DOMAIN)
        || selection.k4_input_domain_hex != hex(K4_INPUT_DOMAIN)
        || scan.sfbinpack_version != "0.6.4"
        || scan.run_records != RUN_RECORDS
        || scan.max_scan_seconds != MAX_SCAN_SECONDS
        || selection.pilot_candidate_target != PILOT_CANDIDATE_TARGET
        || selection.main_candidate_target != MAIN_CANDIDATE_TARGET
        || selection.holdout_candidate_target != HOLDOUT_CANDIDATE_TARGET
        || selection.pilot_accepted_target != PILOT_ACCEPTED_TARGET
        || selection.main_accepted_target != MAIN_ACCEPTED_TARGET
        || selection.holdout_accepted_target != HOLDOUT_ACCEPTED_TARGET
        || output.pilot_candidate_positions != selection.counts.pilot_candidate_positions
        || output.main_expansion_candidate_positions
            != selection.counts.main_candidate_positions
                - selection.counts.pilot_candidate_positions
        || output.total_train_candidate_positions != selection.counts.main_candidate_positions
        || output.validation_candidate_positions != selection.counts.validation_candidate_positions
        || output.calibration_candidate_positions
            != selection.counts.calibration_candidate_positions
        || output.reserved_test_candidate_positions
            != selection.counts.reserved_test_candidate_positions
        || output.pilot_accepted_target != PILOT_ACCEPTED_TARGET
        || output.main_accepted_target != MAIN_ACCEPTED_TARGET
        || output.holdout_accepted_target != HOLDOUT_ACCEPTED_TARGET
    {
        bail!("manifest contract fields differ from verifier constants");
    }
    let (source_bytes, source_sha) = sha256_file(source_path)?;
    if source_bytes != SOURCE_BYTES || source_sha != SOURCE_SHA256 {
        bail!("source argument identity mismatch");
    }
    for item in [
        &output.selection,
        &selection.source,
        &selection.scan_manifest,
        &selection.historical_positions,
        &selection.historical_keys,
        &selection.conflicts,
        &selection.quarantine_chains,
        &selection.owners_by_chain,
        &selection.owners_by_key,
        &selection.selected_chains,
        &scan.source,
    ] {
        verify_receipt(item)?;
    }
    for item in selection.candidate_files.iter() {
        verify_receipt(item)?;
    }
    let mut source_end = 0u64;
    let mut chain_end = 0u64;
    let mut eligible = 0u64;
    for (index, chunk) in scan.chunks.iter().enumerate() {
        if chunk.schema != CHUNK_SCHEMA
            || chunk.chunk != index as u32
            || chunk.source_position_start != source_end
            || chunk.chain_start != chain_end
            || chunk.decoded_positions != chunk.source_position_end - chunk.source_position_start
            || chunk.keys.bytes / KEY_BYTES as u64 != chunk.eligible_positions
            || chunk.chains.bytes / CHAIN_BYTES as u64 != chunk.chain_end - chunk.chain_start
        {
            bail!("scan chunk contract mismatch at {index}");
        }
        verify_receipt(&chunk.keys)?;
        verify_receipt(&chunk.chains)?;
        source_end = chunk.source_position_end;
        chain_end = chunk.chain_end;
        eligible += chunk.eligible_positions;
    }
    if source_end != scan.decoded_positions
        || chain_end != scan.complete_chains
        || eligible != scan.eligible_positions
    {
        bail!("scan aggregate mismatch");
    }
    for shard in &output.shards {
        verify_receipt(&shard.file)?;
    }
    Ok(())
}

fn main() -> Result<()> {
    let args: Vec<_> = env::args_os().collect();
    if args.len() != 5 || args[1] != "--manifest" || args[3] != "--source" {
        bail!("usage: ngnk4sampleverify --manifest OUTPUT/manifest.json --source SOURCE.binpack");
    }
    let manifest_path = PathBuf::from(&args[2]);
    let source_path = PathBuf::from(&args[4]);
    let output: OutputManifest = serde_json::from_reader(File::open(&manifest_path)?)?;
    let selection: SelectionManifest =
        serde_json::from_reader(File::open(&output.selection.path)?)?;
    let scan: ScanManifest = serde_json::from_reader(File::open(&selection.scan_manifest.path)?)?;
    verify_contracts(&source_path, &output, &selection, &scan)?;
    let source_sha = decode_hex_32(SOURCE_SHA256)?;
    let historical = verify_historical(&selection)?;
    let quarantined = verify_conflicts(&scan, &selection, &historical)?;
    let chain_count: usize = scan.complete_chains.try_into()?;
    let selected_mask = load_selected_mask(&selection, chain_count)?;
    let (owner_digest, owner_counts) =
        verify_owners_by_key(&scan, &selection, &source_sha, &quarantined, &selected_mask)?;
    let chains = load_chains(&scan, &source_sha)?;
    verify_owner_chain_order(
        &selection,
        &chains,
        &selected_mask,
        &owner_counts,
        &owner_digest,
    )?;
    let selected = verify_candidates_and_selection(&selection, &chains, &owner_counts)?;
    verify_source_and_shards(&source_path, &scan, &selection, &output, &chains, &selected)?;
    println!(
        "verified {} selected chains and {} train candidate positions",
        selection.counts.selected_chains, selection.counts.main_candidate_positions
    );
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn independent_k4_goldens_match_go() {
        let start =
            Position::from_fen("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1").unwrap();
        let black =
            Position::from_fen("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b - - 0 1").unwrap();
        assert_eq!(
            hex(local_k4_key(&start).unwrap()),
            "754c73a16b5b3d57e57872163f8df31d7f867efe6b6dd6d3590edb2e2dae1b16"
        );
        assert_eq!(
            hex(local_k4_key(&black).unwrap()),
            "e8dc98ae95a55187e5498a2ee1678a7877a5bcf2fd154756f8ea46a80a0827df"
        );
    }

    #[test]
    fn owner_multiset_is_order_independent_and_multiplicity_sensitive() {
        let a = Owner {
            chain: 1,
            entry: 2,
            split: Split::Train,
            key: [3; 32],
        };
        let b = Owner {
            chain: 4,
            entry: 5,
            split: Split::Validation,
            key: [6; 32],
        };
        let mut left = MultisetDigest::default();
        left.add(&owner_bytes(a));
        left.add(&owner_bytes(b));
        let mut right = MultisetDigest::default();
        right.add(&owner_bytes(b));
        right.add(&owner_bytes(a));
        assert!(left == right);
        right.add(&owner_bytes(a));
        assert!(left != right);
    }
}
