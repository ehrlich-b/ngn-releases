use std::{
    cmp::Ordering,
    fs::File,
    io::{BufReader, Read},
    path::Path,
};

use anyhow::{Context, Result, bail};
use serde::{Deserialize, Serialize};
use sfbinpack::{
    TrainingDataEntry,
    chess::{
        color::Color, coords::Square, r#move::MoveType, piece::Piece, piecetype::PieceType,
        position::Position,
    },
};
use sha2::{Digest, Sha256};

pub const CONTRACT_VERSION: &str = "ngn-k4-sampler-v2";
pub const SOURCE_REVISION: &str = "1e095a758c630bc58d0b6dac4da44fcd38ac89c2";
pub const SOURCE_BYTES: u64 = 10_809_713_086;
pub const SOURCE_SHA256: &str = "0d22957b8d4f0f8e6f2913be7b744b2dab5178c3563e0f916312d0d94c28b92b";
pub const SPLIT_SEED: u64 = 26_092_001;
pub const SPLIT_DOMAIN: &[u8] = b"ngn-k4-chain-split-v1\0";
pub const CHAIN_PRIORITY_DOMAIN: &[u8] = b"ngn-k4-chain-priority-v1\0";
pub const POSITION_PRIORITY_DOMAIN: &[u8] = b"ngn-k4-position-priority-v1\0";
pub const POSITION_ID_DOMAIN: &[u8] = b"ngn-k4-position-id-v1\0";
pub const K4_INPUT_DOMAIN: &[u8] = b"ngn-k4-input-v1\0";
pub const MAX_OBSERVED_CHAIN: usize = 65_536;
pub const DEFAULT_RUN_RECORDS: usize = 1_000_000;
pub const MAX_SCAN_SECONDS: u64 = 14_400;
pub const KEY_RECORD_BYTES: usize = 48;
pub const CHAIN_RECORD_BYTES: usize = 64;
pub const CONFLICT_RECORD_BYTES: usize = 40;
pub const OWNER_RECORD_BYTES: usize = 48;
pub const CANDIDATE_RECORD_BYTES: usize = 48;
pub const SELECTED_CHAIN_BYTES: usize = 16;

pub mod fixed;

const KING_BUCKETS: [u16; 64] = [
    1, 1, 0, 0, 0, 0, 1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
    3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
];

#[derive(Clone, Copy, Debug, Deserialize, Eq, Hash, Ord, PartialEq, PartialOrd, Serialize)]
#[repr(u8)]
#[serde(rename_all = "kebab-case")]
pub enum Split {
    Train = 0,
    Validation = 1,
    Calibration = 2,
    ReservedTest = 3,
}

impl Split {
    pub const ALL: [Self; 4] = [
        Self::Train,
        Self::Validation,
        Self::Calibration,
        Self::ReservedTest,
    ];

    pub fn as_str(self) -> &'static str {
        match self {
            Self::Train => "train",
            Self::Validation => "validation",
            Self::Calibration => "calibration",
            Self::ReservedTest => "reserved-test",
        }
    }

    pub fn from_byte(value: u8) -> Result<Self> {
        match value {
            0 => Ok(Self::Train),
            1 => Ok(Self::Validation),
            2 => Ok(Self::Calibration),
            3 => Ok(Self::ReservedTest),
            _ => bail!("invalid split byte {value}"),
        }
    }
}

pub fn hex(bytes: impl AsRef<[u8]>) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let bytes = bytes.as_ref();
    let mut output = String::with_capacity(bytes.len() * 2);
    for &byte in bytes {
        output.push(DIGITS[(byte >> 4) as usize] as char);
        output.push(DIGITS[(byte & 15) as usize] as char);
    }
    output
}

pub fn decode_hex_32(value: &str) -> Result<[u8; 32]> {
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

pub fn sha256_file(path: &Path) -> Result<(u64, String)> {
    let mut reader =
        BufReader::new(File::open(path).with_context(|| format!("open {}", path.display()))?);
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

fn identity_digest(domain: &[u8], source_sha: &[u8; 32], chain: u64) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(domain);
    digest.update(SPLIT_SEED.to_le_bytes());
    digest.update(source_sha);
    digest.update(chain.to_le_bytes());
    digest.finalize().into()
}

pub fn chain_split(source_sha: &[u8; 32], chain: u64) -> Split {
    let digest = identity_digest(SPLIT_DOMAIN, source_sha, chain);
    match u64::from_le_bytes(digest[..8].try_into().unwrap()) % 100 {
        0..=96 => Split::Train,
        97 => Split::Validation,
        98 => Split::Calibration,
        _ => Split::ReservedTest,
    }
}

pub fn chain_priority(source_sha: &[u8; 32], chain: u64) -> [u8; 32] {
    identity_digest(CHAIN_PRIORITY_DOMAIN, source_sha, chain)
}

pub fn position_priority(source_sha: &[u8; 32], chain: u64, entry: u32) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(POSITION_PRIORITY_DOMAIN);
    digest.update(SPLIT_SEED.to_le_bytes());
    digest.update(source_sha);
    digest.update(chain.to_le_bytes());
    digest.update(entry.to_le_bytes());
    digest.finalize().into()
}

pub fn position_id(source_sha: &[u8; 32], chain: u64, entry: u32, k4_key: &[u8; 32]) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(POSITION_ID_DOMAIN);
    digest.update(source_sha);
    digest.update(chain.to_le_bytes());
    digest.update(entry.to_le_bytes());
    digest.update(k4_key);
    digest.finalize().into()
}

fn k4_feature(
    color: usize,
    piece_type: usize,
    square: usize,
    king_square: usize,
    perspective: usize,
) -> u16 {
    let mut oriented_square = square;
    let mut oriented_king = king_square;
    if perspective == 1 {
        oriented_square ^= 56;
        oriented_king ^= 56;
    }
    if king_square % 8 > 3 {
        oriented_square ^= 7;
    }
    KING_BUCKETS[oriented_king] * 768
        + ((color ^ perspective) * 384 + piece_type * 64 + oriented_square) as u16
}

pub fn k4_input_components(position: &Position) -> Result<([Vec<u16>; 2], u8)> {
    let white_kings = position.pieces_bb_color(Color::White, PieceType::King);
    let black_kings = position.pieces_bb_color(Color::Black, PieceType::King);
    if white_kings.count() != 1 || black_kings.count() != 1 {
        bail!("K4 input requires exactly one king per color");
    }
    let king_squares = [
        white_kings.bits().trailing_zeros() as usize,
        black_kings.bits().trailing_zeros() as usize,
    ];
    let mut features = [Vec::new(), Vec::new()];
    let piece_types = [
        PieceType::Pawn,
        PieceType::Knight,
        PieceType::Bishop,
        PieceType::Rook,
        PieceType::Queen,
        PieceType::King,
    ];
    let colors = [Color::White, Color::Black];
    let mut piece_count = 0usize;
    for (color_index, color) in colors.into_iter().enumerate() {
        for (piece_index, piece_type) in piece_types.into_iter().enumerate() {
            let mut pieces = position.pieces_bb_color(color, piece_type).bits();
            piece_count += pieces.count_ones() as usize;
            while pieces != 0 {
                let square = pieces.trailing_zeros() as usize;
                pieces &= pieces - 1;
                features[0].push(k4_feature(
                    color_index,
                    piece_index,
                    square,
                    king_squares[0],
                    0,
                ));
                features[1].push(k4_feature(
                    color_index,
                    piece_index,
                    square,
                    king_squares[1],
                    1,
                ));
            }
        }
    }
    if piece_count > 32 {
        bail!("K4 input has {piece_count} pieces");
    }
    features[0].sort_unstable();
    features[1].sort_unstable();
    if position.side_to_move() == Color::Black {
        features.swap(0, 1);
    }
    let head = ((piece_count.saturating_sub(2)) / 4).min(7) as u8;
    Ok((features, head))
}

pub fn k4_input_key(position: &Position) -> Result<[u8; 32]> {
    let (features, head) = k4_input_components(position)?;
    let mut digest = Sha256::new();
    digest.update(K4_INPUT_DOMAIN);
    for perspective in features {
        digest.update((perspective.len() as u16).to_le_bytes());
        for feature in perspective {
            digest.update(feature.to_le_bytes());
        }
    }
    digest.update([head]);
    Ok(digest.finalize().into())
}

fn sign(value: i32) -> i32 {
    value.signum()
}

fn path_is_clear(position: &Position, from: i32, to: i32, step: i32) -> bool {
    let mut square = from + step;
    while square != to {
        if position.piece_at(Square::new(square as u32)) != Piece::none() {
            return false;
        }
        square += step;
    }
    true
}

fn quiet_move_is_legal(entry: &TrainingDataEntry) -> bool {
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
    let from_file = from % 8;
    let from_rank = from / 8;
    let to_file = to % 8;
    let to_rank = to / 8;
    let df = to_file - from_file;
    let dr = to_rank - from_rank;
    let pseudo_legal = match piece.piece_type() {
        PieceType::Pawn => {
            let (direction, home_rank) = if piece.color() == Color::White {
                (1, 1)
            } else {
                (-1, 6)
            };
            df == 0
                && (dr == direction
                    || (from_rank == home_rank
                        && dr == 2 * direction
                        && entry
                            .pos
                            .piece_at(Square::new((from + 8 * direction) as u32))
                            == Piece::none()))
                && to_rank != 0
                && to_rank != 7
        }
        PieceType::Knight => matches!((df.abs(), dr.abs()), (1, 2) | (2, 1)),
        PieceType::Bishop => {
            df.abs() == dr.abs() && path_is_clear(&entry.pos, from, to, sign(df) + 8 * sign(dr))
        }
        PieceType::Rook => {
            (df == 0 || dr == 0)
                && path_is_clear(
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
            step != 0 && path_is_clear(&entry.pos, from, to, step)
        }
        PieceType::King => df.abs() <= 1 && dr.abs() <= 1,
        PieceType::None => false,
    };
    if !pseudo_legal {
        return false;
    }
    let mover = entry.pos.side_to_move();
    !entry.pos.after_move(entry.mv).is_checked(mover)
}

pub fn rejection_reasons(entry: &TrainingDataEntry) -> Vec<&'static str> {
    let mut reasons = Vec::new();
    if entry.ply < 8 {
        reasons.push("ply-before-8");
    }
    let occupied = entry.pos.occupied().count();
    if occupied > 32
        || entry
            .pos
            .pieces_bb_color(Color::White, PieceType::King)
            .count()
            != 1
        || entry
            .pos
            .pieces_bb_color(Color::Black, PieceType::King)
            .count()
            != 1
    {
        reasons.push("invalid-structure");
        return reasons;
    }
    if entry.pos.is_checked(entry.pos.side_to_move()) {
        reasons.push("stm-in-check");
    }
    if !quiet_move_is_legal(entry) {
        reasons.push("source-move-not-legal-quiet");
    }
    reasons
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct KeyRecord {
    pub key: [u8; 32],
    pub chain: u64,
    pub entry: u32,
    pub split: Split,
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

impl KeyRecord {
    pub fn encode(self) -> [u8; KEY_RECORD_BYTES] {
        let mut bytes = [0u8; KEY_RECORD_BYTES];
        bytes[..32].copy_from_slice(&self.key);
        bytes[32..40].copy_from_slice(&self.chain.to_le_bytes());
        bytes[40..44].copy_from_slice(&self.entry.to_le_bytes());
        bytes[44] = self.split as u8;
        bytes
    }

    pub fn decode(bytes: [u8; KEY_RECORD_BYTES]) -> Result<Self> {
        if bytes[45..].iter().any(|byte| *byte != 0) {
            bail!("nonzero key-record padding");
        }
        Ok(Self {
            key: bytes[..32].try_into().unwrap(),
            chain: u64::from_le_bytes(bytes[32..40].try_into().unwrap()),
            entry: u32::from_le_bytes(bytes[40..44].try_into().unwrap()),
            split: Split::from_byte(bytes[44])?,
        })
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ChainRecord {
    pub chain: u64,
    pub source_start: u64,
    pub decoded: u32,
    pub eligible: u32,
    pub split: Split,
    pub priority: [u8; 32],
}

impl ChainRecord {
    pub fn encode(self) -> [u8; CHAIN_RECORD_BYTES] {
        let mut bytes = [0u8; CHAIN_RECORD_BYTES];
        bytes[..8].copy_from_slice(&self.chain.to_le_bytes());
        bytes[8..16].copy_from_slice(&self.source_start.to_le_bytes());
        bytes[16..20].copy_from_slice(&self.decoded.to_le_bytes());
        bytes[20..24].copy_from_slice(&self.eligible.to_le_bytes());
        bytes[24] = self.split as u8;
        bytes[32..].copy_from_slice(&self.priority);
        bytes
    }

    pub fn decode(bytes: [u8; CHAIN_RECORD_BYTES]) -> Result<Self> {
        if bytes[25..32].iter().any(|byte| *byte != 0) {
            bail!("nonzero chain-record padding");
        }
        Ok(Self {
            chain: u64::from_le_bytes(bytes[..8].try_into().unwrap()),
            source_start: u64::from_le_bytes(bytes[8..16].try_into().unwrap()),
            decoded: u32::from_le_bytes(bytes[16..20].try_into().unwrap()),
            eligible: u32::from_le_bytes(bytes[20..24].try_into().unwrap()),
            split: Split::from_byte(bytes[24])?,
            priority: bytes[32..].try_into().unwrap(),
        })
    }
}

#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub struct ConflictRecord {
    pub key: [u8; 32],
    pub cross_split: bool,
    pub historical_holdout: bool,
}

impl ConflictRecord {
    pub fn encode(self) -> [u8; CONFLICT_RECORD_BYTES] {
        let mut bytes = [0u8; CONFLICT_RECORD_BYTES];
        bytes[..32].copy_from_slice(&self.key);
        bytes[32] = u8::from(self.cross_split);
        bytes[33] = u8::from(self.historical_holdout);
        bytes
    }

    pub fn decode(bytes: [u8; CONFLICT_RECORD_BYTES]) -> Result<Self> {
        if bytes[32] > 1 || bytes[33] > 1 || bytes[34..].iter().any(|byte| *byte != 0) {
            bail!("invalid conflict record flags/padding");
        }
        Ok(Self {
            key: bytes[..32].try_into().unwrap(),
            cross_split: bytes[32] != 0,
            historical_holdout: bytes[33] != 0,
        })
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct OwnerRecord {
    pub chain: u64,
    pub entry: u32,
    pub split: Split,
    pub key: [u8; 32],
}

impl Ord for OwnerRecord {
    fn cmp(&self, other: &Self) -> Ordering {
        (self.chain, self.entry, self.key).cmp(&(other.chain, other.entry, other.key))
    }
}

impl PartialOrd for OwnerRecord {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl OwnerRecord {
    pub fn encode(self) -> [u8; OWNER_RECORD_BYTES] {
        let mut bytes = [0u8; OWNER_RECORD_BYTES];
        bytes[..8].copy_from_slice(&self.chain.to_le_bytes());
        bytes[8..12].copy_from_slice(&self.entry.to_le_bytes());
        bytes[12] = self.split as u8;
        bytes[16..].copy_from_slice(&self.key);
        bytes
    }

    pub fn decode(bytes: [u8; OWNER_RECORD_BYTES]) -> Result<Self> {
        if bytes[13..16].iter().any(|byte| *byte != 0) {
            bail!("nonzero owner-record padding");
        }
        Ok(Self {
            chain: u64::from_le_bytes(bytes[..8].try_into().unwrap()),
            entry: u32::from_le_bytes(bytes[8..12].try_into().unwrap()),
            split: Split::from_byte(bytes[12])?,
            key: bytes[16..].try_into().unwrap(),
        })
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct CandidateRecord {
    pub priority: [u8; 32],
    pub chain: u64,
    pub unique_positions: u32,
    pub split: Split,
}

impl Ord for CandidateRecord {
    fn cmp(&self, other: &Self) -> Ordering {
        (self.priority, self.chain).cmp(&(other.priority, other.chain))
    }
}

impl PartialOrd for CandidateRecord {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl CandidateRecord {
    pub fn encode(self) -> [u8; CANDIDATE_RECORD_BYTES] {
        let mut bytes = [0u8; CANDIDATE_RECORD_BYTES];
        bytes[..32].copy_from_slice(&self.priority);
        bytes[32..40].copy_from_slice(&self.chain.to_le_bytes());
        bytes[40..44].copy_from_slice(&self.unique_positions.to_le_bytes());
        bytes[44] = self.split as u8;
        bytes
    }

    pub fn decode(bytes: [u8; CANDIDATE_RECORD_BYTES]) -> Result<Self> {
        if bytes[45..].iter().any(|byte| *byte != 0) {
            bail!("nonzero candidate-record padding");
        }
        Ok(Self {
            priority: bytes[..32].try_into().unwrap(),
            chain: u64::from_le_bytes(bytes[32..40].try_into().unwrap()),
            unique_positions: u32::from_le_bytes(bytes[40..44].try_into().unwrap()),
            split: Split::from_byte(bytes[44])?,
        })
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct SelectedChain {
    pub chain: u64,
    pub split: Split,
    pub pilot: bool,
    pub unique_positions: u32,
}

impl Ord for SelectedChain {
    fn cmp(&self, other: &Self) -> Ordering {
        self.chain.cmp(&other.chain)
    }
}

impl PartialOrd for SelectedChain {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl SelectedChain {
    pub fn encode(self) -> [u8; SELECTED_CHAIN_BYTES] {
        let mut bytes = [0u8; SELECTED_CHAIN_BYTES];
        bytes[..8].copy_from_slice(&self.chain.to_le_bytes());
        bytes[8] = self.split as u8;
        bytes[9] = u8::from(self.pilot);
        bytes[12..16].copy_from_slice(&self.unique_positions.to_le_bytes());
        bytes
    }

    pub fn decode(bytes: [u8; SELECTED_CHAIN_BYTES]) -> Result<Self> {
        if bytes[9] > 1 || bytes[10..12].iter().any(|byte| *byte != 0) {
            bail!("invalid selected-chain flags/padding");
        }
        Ok(Self {
            chain: u64::from_le_bytes(bytes[..8].try_into().unwrap()),
            split: Split::from_byte(bytes[8])?,
            pilot: bytes[9] != 0,
            unique_positions: u32::from_le_bytes(bytes[12..16].try_into().unwrap()),
        })
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct FileReceipt {
    pub path: String,
    pub bytes: u64,
    pub sha256: String,
}

pub fn receipt(path: &Path) -> Result<FileReceipt> {
    let (bytes, sha256) = sha256_file(path)?;
    Ok(FileReceipt {
        path: path.display().to_string(),
        bytes,
        sha256,
    })
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ScanChunkReceipt {
    pub schema: String,
    pub chunk: u32,
    pub source_position_start: u64,
    pub source_position_end: u64,
    pub chain_start: u64,
    pub chain_end: u64,
    pub decoded_positions: u64,
    pub eligible_positions: u64,
    pub rejection_counts: std::collections::BTreeMap<String, u64>,
    pub keys: FileReceipt,
    pub chains: FileReceipt,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ScanManifest {
    pub schema: String,
    pub contract_version: String,
    pub state: String,
    pub source_revision: String,
    pub source: FileReceipt,
    pub split_seed: u64,
    pub split_domain_hex: String,
    pub chain_priority_domain_hex: String,
    pub position_priority_domain_hex: String,
    pub k4_input_domain_hex: String,
    pub sfbinpack_version: String,
    pub exact_argv: Vec<String>,
    pub run_records: usize,
    pub max_scan_seconds: u64,
    pub chunks: Vec<ScanChunkReceipt>,
    pub decoded_positions: u64,
    pub complete_chains: u64,
    pub eligible_positions: u64,
    pub rejection_counts: std::collections::BTreeMap<String, u64>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use sfbinpack::chess::{coords::Square, r#move::Move, position::Position};

    fn entry(fen: &str, from: u32, to: u32, ply: u16) -> TrainingDataEntry {
        TrainingDataEntry {
            pos: Position::from_fen(fen).unwrap(),
            mv: Move::new(
                Square::new(from),
                Square::new(to),
                MoveType::Normal,
                Piece::none(),
            ),
            score: 0,
            ply,
            result: 0,
        }
    }

    #[test]
    fn k4_key_matches_go_goldens_and_state_contract() {
        let start =
            Position::from_fen("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1").unwrap();
        assert_eq!(
            hex(k4_input_key(&start).unwrap()),
            "754c73a16b5b3d57e57872163f8df31d7f867efe6b6dd6d3590edb2e2dae1b16"
        );
        let black =
            Position::from_fen("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b - - 0 1").unwrap();
        assert_eq!(
            hex(k4_input_key(&black).unwrap()),
            "e8dc98ae95a55187e5498a2ee1678a7877a5bcf2fd154756f8ea46a80a0827df"
        );
        let state =
            Position::from_fen("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 44 99")
                .unwrap();
        assert_eq!(k4_input_key(&black).unwrap(), k4_input_key(&state).unwrap());
    }

    #[test]
    fn legality_filter_handles_paths_pins_promotions_and_ep() {
        assert!(rejection_reasons(&entry("7k/8/8/8/8/8/2P5/K7 w - - 0 8", 10, 26, 8)).is_empty());
        assert_eq!(
            rejection_reasons(&entry("7k/8/8/8/8/8/2P5/K7 w - - 0 8", 10, 18, 7)),
            vec!["ply-before-8"]
        );
        assert!(
            rejection_reasons(&entry("4r2k/8/8/8/8/8/4R3/4K3 w - - 0 8", 12, 13, 8))
                .contains(&"source-move-not-legal-quiet")
        );
        assert!(
            rejection_reasons(&entry("7k/P7/8/8/8/8/8/K7 w - - 0 8", 48, 56, 8))
                .contains(&"source-move-not-legal-quiet")
        );
    }

    #[test]
    fn fixed_records_round_trip_and_reject_padding() {
        let key = KeyRecord {
            key: [7; 32],
            chain: 55,
            entry: 3,
            split: Split::Calibration,
        };
        assert_eq!(KeyRecord::decode(key.encode()).unwrap(), key);
        let chain = ChainRecord {
            chain: 8,
            source_start: 99,
            decoded: 12,
            eligible: 7,
            split: Split::ReservedTest,
            priority: [9; 32],
        };
        assert_eq!(ChainRecord::decode(chain.encode()).unwrap(), chain);
        let mut corrupt = key.encode();
        corrupt[47] = 1;
        assert!(KeyRecord::decode(corrupt).is_err());
        let owner = OwnerRecord {
            chain: 2,
            entry: 7,
            split: Split::Validation,
            key: [3; 32],
        };
        assert_eq!(OwnerRecord::decode(owner.encode()).unwrap(), owner);
        let candidate = CandidateRecord {
            priority: [4; 32],
            chain: 5,
            unique_positions: 13,
            split: Split::Train,
        };
        assert_eq!(
            CandidateRecord::decode(candidate.encode()).unwrap(),
            candidate
        );
        let selected = SelectedChain {
            chain: 6,
            split: Split::Calibration,
            pilot: false,
            unique_positions: 21,
        };
        assert_eq!(SelectedChain::decode(selected.encode()).unwrap(), selected);
    }

    #[test]
    fn split_is_stable_and_covers_four_partitions() {
        let source = [0x42; 32];
        let observed: std::collections::BTreeSet<_> = (0..10_000)
            .map(|chain| chain_split(&source, chain))
            .collect();
        assert_eq!(observed, Split::ALL.into_iter().collect());
        assert_eq!(chain_split(&source, 0), chain_split(&source, 0));
        assert_ne!(chain_priority(&source, 0), position_priority(&source, 0, 0));
    }
}
