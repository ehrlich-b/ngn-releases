//! Archive-labeled K4 corpus: every eligible train-split position of the pinned
//! T80 archive, carrying the archive's own search score instead of a fresh
//! teacher label. `scatter` decodes the archive once into randomly assigned
//! bucket files of Bullet `ChessBoard` records with raw archive scores;
//! `gather` converts scores to NGN units, shuffles each bucket and concatenates
//! them into one training BF.

use std::{
    collections::{BTreeMap, HashSet},
    env,
    fs::{self, File, OpenOptions},
    io::{BufReader, BufWriter, Read, Write},
    path::{Path, PathBuf},
    sync::{Arc, mpsc},
    thread,
    time::Instant,
};

use anyhow::{Context, Result, bail};
use ngn_k4_sample::{
    CONFLICT_RECORD_BYTES, FileReceipt, SOURCE_BYTES, SOURCE_REVISION, SOURCE_SHA256, SPLIT_SEED,
    Split, chain_split, decode_hex_32, hex, k4_input_key, receipt, rejection_reasons, sha256_file,
};
use serde::Serialize;
use sfbinpack::{
    CompressedTrainingDataEntryReader, TrainingDataEntry,
    chess::{color::Color, piecetype::PieceType},
};
use sha2::{Digest, Sha256};

const SCATTER_SCHEMA: &str = "ngn-k4-archive-scatter-v1";
const CORPUS_SCHEMA: &str = "ngn-k4-archive-corpus-v1";
const RECORD_BYTES: usize = 32;
const BATCH_ENTRIES: usize = 65_536;
const BUCKET_WRITER_BYTES: usize = 1 << 20;
const MAX_ABS_NGN_SCORE: f64 = 10_000.0;
/// Stockfish `VALUE_NONE` is 32002; the archive uses it for unscored positions.
const SENTINEL_ABS_RAW: u16 = 32_000;
/// Additional archives from the same pinned `official-stockfish/master-binpacks`
/// revision as the T80 source, bound by exact byte count and Hugging Face LFS
/// SHA-256. Each uses its own SHA-256 as the chain-split key.
const EXTRA_SOURCES: [(u64, &str); 4] = [
    (6_141_321_549, "255956f2d28d78e5eb050aa542d768b0d8e0667ea09bb250effc83706f26da54"), // farseerT76
    (20_144_023_865, "cebf6e5aa62a0df447f3748c90a542ded13105c873a1a819d7f71bdf041ca8cb"), // farseerT74
    (32_981_564_118, "8c329b988c02bdf4ccc13b3cc627f69531b8b6bb1719f17f775f49476d045c70"), // T60T70wIsRightFarseer
    (46_983_203_020, "f1907be568d4a48297dd28c2060c53b07ee4170e4e9392faa525d6d697948441"), // farseerT75
];
/// Raw-score calibration bins (width 25, clamped to +/-2000) over all eligible
/// train records, counted by STM result index (loss, draw, win).
const SCORE_BIN_WIDTH: i32 = 25;
const SCORE_BIN_LIMIT: i32 = 2_000;

fn usage() -> ! {
    eprintln!(
        "usage: ngnk4archive scatter --input BINPACK --conflicts conflicts.bin --probe-keys KEYS.bin --output NEW_DIR --buckets N --workers N --seed N [--keep-per-mille K]\n       ngnk4archive gather --scatter DIR[@RAW_SCALE][,DIR[@RAW_SCALE]...] --map SCORE_MAP.json --output NEW.bf --manifest NEW.json --seed N"
    );
    std::process::exit(2);
}

fn flags(args: &[String]) -> BTreeMap<String, String> {
    let mut values = BTreeMap::new();
    let mut index = 0;
    while index < args.len() {
        if index + 1 >= args.len() || !args[index].starts_with("--") {
            usage();
        }
        if values.insert(args[index].clone(), args[index + 1].clone()).is_some() {
            usage();
        }
        index += 2;
    }
    values
}

fn required<'a>(values: &'a BTreeMap<String, String>, name: &str) -> &'a str {
    values.get(name).map(String::as_str).unwrap_or_else(|| usage())
}

fn splitmix64(state: &mut u64) -> u64 {
    *state = state.wrapping_add(0x9e37_79b9_7f4a_7c15);
    let mut value = *state;
    value = (value ^ (value >> 30)).wrapping_mul(0xbf58_476d_1ce4_e5b9);
    value = (value ^ (value >> 27)).wrapping_mul(0x94d0_49bb_1331_11eb);
    value ^ (value >> 31)
}

/// Mirrors Bullet 629ee50 `convert_to_bulletformat` followed by bulletformat
/// 1.8.0 `ChessBoard::from_raw`: the stored board, score and result are all
/// relative to the side to move, with the board flipped when Black moves.
fn pack(entry: &TrainingDataEntry) -> Result<[u8; RECORD_BYTES]> {
    let pieces = |piece_type| {
        entry.pos.pieces_bb_color(Color::Black, piece_type).bits()
            | entry.pos.pieces_bb_color(Color::White, piece_type).bits()
    };
    let mut bbs = [
        entry.pos.pieces_bb(Color::White).bits(),
        entry.pos.pieces_bb(Color::Black).bits(),
        pieces(PieceType::Pawn),
        pieces(PieceType::Knight),
        pieces(PieceType::Bishop),
        pieces(PieceType::Rook),
        pieces(PieceType::Queen),
        pieces(PieceType::King),
    ];
    if entry.pos.side_to_move() == Color::Black {
        for bb in &mut bbs {
            *bb = bb.swap_bytes();
        }
        bbs.swap(0, 1);
    }
    let occupied = bbs[0] | bbs[1];
    let mut record = [0u8; RECORD_BYTES];
    record[..8].copy_from_slice(&occupied.to_le_bytes());
    let mut remaining = occupied;
    let mut index = 0usize;
    while remaining != 0 {
        let bit = 1u64 << remaining.trailing_zeros();
        remaining &= remaining - 1;
        let colour = u8::from(bit & bbs[1] != 0) << 3;
        let piece = bbs[2..]
            .iter()
            .position(|bb| bit & bb != 0)
            .context("occupied square without a piece")?;
        if index >= 32 {
            bail!("more than 32 pieces");
        }
        record[8 + index / 2] |= (colour | piece as u8) << (4 * (index & 1));
        index += 1;
    }
    if !(-1..=1).contains(&entry.result) {
        bail!("archive result {} outside -1..1", entry.result);
    }
    record[24..26].copy_from_slice(&entry.score.to_le_bytes());
    record[26] = (1 + entry.result) as u8;
    record[27] = (bbs[0] & bbs[7]).trailing_zeros() as u8;
    record[28] = (bbs[1] & bbs[7]).trailing_zeros() as u8 ^ 56;
    Ok(record)
}

fn read_keys(path: &Path, record_bytes: usize) -> Result<HashSet<[u8; 32]>> {
    let data = fs::read(path).with_context(|| format!("read {}", path.display()))?;
    if data.len() % record_bytes != 0 {
        bail!("{} is not a whole number of {record_bytes}-byte records", path.display());
    }
    let mut keys = HashSet::with_capacity(data.len() / record_bytes);
    for record in data.chunks_exact(record_bytes) {
        keys.insert(record[..32].try_into().unwrap());
    }
    Ok(keys)
}

#[derive(Clone, Debug, Default, Serialize)]
struct Counts {
    decoded: u64,
    decoded_by_split: BTreeMap<String, u64>,
    rejections: BTreeMap<String, u64>,
    eligible_train: u64,
    excluded_conflict_key: u64,
    emitted: u64,
    emitted_result: BTreeMap<String, u64>,
    emitted_abs_raw_score_over: BTreeMap<String, u64>,
    probe_rows: u64,
    eligible_score_result_bins: BTreeMap<i32, [u64; 3]>,
}

impl Counts {
    fn merge(&mut self, other: Counts) {
        self.decoded += other.decoded;
        for (key, value) in other.decoded_by_split {
            *self.decoded_by_split.entry(key).or_default() += value;
        }
        for (key, value) in other.rejections {
            *self.rejections.entry(key).or_default() += value;
        }
        self.eligible_train += other.eligible_train;
        self.excluded_conflict_key += other.excluded_conflict_key;
        self.emitted += other.emitted;
        for (key, value) in other.emitted_result {
            *self.emitted_result.entry(key).or_default() += value;
        }
        for (key, value) in other.emitted_abs_raw_score_over {
            *self.emitted_abs_raw_score_over.entry(key).or_default() += value;
        }
        self.probe_rows += other.probe_rows;
        for (key, value) in other.eligible_score_result_bins {
            let bin = self.eligible_score_result_bins.entry(key).or_default();
            for (total, count) in bin.iter_mut().zip(value) {
                *total += count;
            }
        }
    }
}

struct Processed {
    index: u64,
    records: Vec<[u8; RECORD_BYTES]>,
    probe_lines: Vec<String>,
    counts: Counts,
}

fn process(
    index: u64,
    batch: Vec<(TrainingDataEntry, Split)>,
    conflicts: &HashSet<[u8; 32]>,
    probes: &HashSet<[u8; 32]>,
    keep_per_mille: u16,
) -> Result<Processed> {
    let mut counts = Counts::default();
    let mut records = Vec::with_capacity(batch.len());
    let mut probe_lines = Vec::new();
    for (entry, split) in batch {
        counts.decoded += 1;
        *counts.decoded_by_split.entry(split.as_str().to_string()).or_default() += 1;
        let reasons = rejection_reasons(&entry);
        if !reasons.is_empty() {
            for reason in reasons {
                *counts.rejections.entry(reason.to_string()).or_default() += 1;
            }
            continue;
        }
        if split != Split::Train && split != Split::Calibration {
            continue;
        }
        let key = k4_input_key(&entry.pos)?;
        if probes.contains(&key) {
            let side = if entry.pos.side_to_move() == Color::White { "w" } else { "b" };
            probe_lines.push(format!(
                "{},{},{},{},{},{},{}",
                hex(key),
                split.as_str(),
                entry.score,
                entry.result,
                entry.ply,
                side,
                hex(pack(&entry)?)
            ));
            counts.probe_rows += 1;
        }
        if split != Split::Train {
            continue;
        }
        counts.eligible_train += 1;
        if conflicts.contains(&key) {
            counts.excluded_conflict_key += 1;
            continue;
        }
        if entry.score.unsigned_abs() < SENTINEL_ABS_RAW {
            let bin = i32::from(entry.score).clamp(-SCORE_BIN_LIMIT, SCORE_BIN_LIMIT).div_euclid(SCORE_BIN_WIDTH);
            let result = usize::try_from(i32::from(entry.result) + 1)?;
            counts.eligible_score_result_bins.entry(bin).or_default()[result] += 1;
        }
        if u16::from_le_bytes([key[0], key[1]]) % 1000 >= keep_per_mille {
            continue;
        }
        records.push(pack(&entry)?);
        counts.emitted += 1;
        *counts.emitted_result.entry(entry.result.to_string()).or_default() += 1;
        let magnitude = entry.score.unsigned_abs();
        for threshold in [1_000u16, 2_000, 5_000, 10_000, 20_000] {
            if magnitude > threshold {
                *counts
                    .emitted_abs_raw_score_over
                    .entry(threshold.to_string())
                    .or_default() += 1;
            }
        }
    }
    Ok(Processed {
        index,
        records,
        probe_lines,
        counts,
    })
}

#[derive(Serialize)]
struct ScatterReceipt {
    schema: &'static str,
    state: &'static str,
    command: Vec<String>,
    source_revision: &'static str,
    source: FileReceipt,
    conflicts: FileReceipt,
    probe_keys: FileReceipt,
    split_seed: u64,
    bucket_seed: u64,
    buckets: usize,
    workers: usize,
    keep_per_mille: u16,
    record_contract: &'static str,
    eligibility: &'static str,
    counts: Counts,
    bucket_records: Vec<u64>,
    probe_csv: FileReceipt,
    elapsed_seconds: f64,
}

fn scatter(args: &[String]) -> Result<()> {
    let started = Instant::now();
    let values = flags(args);
    let input = PathBuf::from(required(&values, "--input"));
    let conflicts_path = PathBuf::from(required(&values, "--conflicts"));
    let probes_path = PathBuf::from(required(&values, "--probe-keys"));
    let output = PathBuf::from(required(&values, "--output"));
    let buckets: usize = required(&values, "--buckets").parse()?;
    let workers: usize = required(&values, "--workers").parse()?;
    let seed: u64 = required(&values, "--seed").parse()?;
    let keep_per_mille: u16 = values.get("--keep-per-mille").map_or(Ok(1000), |value| value.parse())?;
    if !(1..=4096).contains(&buckets) || !(1..=64).contains(&workers) || keep_per_mille > 1000 {
        bail!("buckets must be 1..4096, workers 1..64 and keep-per-mille 0..1000");
    }
    let source = receipt(&input)?;
    let pinned = (source.bytes == SOURCE_BYTES && source.sha256 == SOURCE_SHA256)
        || EXTRA_SOURCES.iter().any(|&(bytes, sha)| source.bytes == bytes && source.sha256 == sha);
    if !pinned {
        bail!("source identity mismatch: bytes={} sha256={}", source.bytes, source.sha256);
    }
    let source_sha = decode_hex_32(&source.sha256)?;
    let conflicts = Arc::new(read_keys(&conflicts_path, CONFLICT_RECORD_BYTES)?);
    let probes = Arc::new(read_keys(&probes_path, 32)?);
    fs::create_dir(&output).with_context(|| format!("create {}", output.display()))?;
    fs::write(output.join("STATE"), "SCATTERING\n")?;
    eprintln!(
        "ngnk4archive: conflicts={} probes={} buckets={buckets} workers={workers}",
        conflicts.len(),
        probes.len()
    );

    let (batch_sender, batch_receiver) =
        mpsc::sync_channel::<(u64, Vec<(TrainingDataEntry, Split)>)>(2 * workers);
    let batch_receiver = Arc::new(std::sync::Mutex::new(batch_receiver));
    let (done_sender, done_receiver) = mpsc::sync_channel::<Result<Processed>>(2 * workers);

    let reader_input = input.clone();
    let reader = thread::spawn(move || -> Result<u64> {
        let file = File::open(&reader_input)?;
        let mut reader = CompressedTrainingDataEntryReader::new(file)
            .map_err(|error| anyhow::anyhow!("open binpack reader: {error:?}"))?;
        let mut chain = 0u64;
        let mut split = chain_split(&source_sha, chain);
        let mut batch = Vec::with_capacity(BATCH_ENTRIES);
        let mut index = 0u64;
        while reader.has_next() {
            let entry = reader.next();
            let continuation = reader.is_next_entry_continuation();
            batch.push((entry, split));
            if !continuation {
                chain += 1;
                split = chain_split(&source_sha, chain);
            }
            if batch.len() == BATCH_ENTRIES {
                batch_sender.send((index, std::mem::take(&mut batch)))?;
                batch.reserve(BATCH_ENTRIES);
                index += 1;
            }
        }
        if !batch.is_empty() {
            batch_sender.send((index, batch))?;
        }
        Ok(chain)
    });

    let mut worker_handles = Vec::new();
    for _ in 0..workers {
        let receiver = batch_receiver.clone();
        let sender = done_sender.clone();
        let conflicts = conflicts.clone();
        let probes = probes.clone();
        worker_handles.push(thread::spawn(move || {
            loop {
                let next = receiver.lock().unwrap().recv();
                let Ok((index, batch)) = next else { break };
                let result = process(index, batch, &conflicts, &probes, keep_per_mille);
                let failed = result.is_err();
                if sender.send(result).is_err() || failed {
                    break;
                }
            }
        }));
    }
    drop(done_sender);

    let mut writers = Vec::with_capacity(buckets);
    for bucket in 0..buckets {
        let path = output.join(format!("bucket-{bucket:04}.bin"));
        let file = OpenOptions::new().create_new(true).write(true).open(&path)?;
        writers.push(BufWriter::with_capacity(BUCKET_WRITER_BYTES, file));
    }
    let probe_path = output.join("probe.csv");
    let mut probe_writer =
        BufWriter::new(OpenOptions::new().create_new(true).write(true).open(&probe_path)?);
    writeln!(probe_writer, "k4_input_sha256,split,raw_score,result,ply,side,board_hex")?;
    let mut bucket_records = vec![0u64; buckets];
    let mut rng = seed;
    let mut counts = Counts::default();
    let mut pending = BTreeMap::new();
    let mut next_index = 0u64;
    let mut last_report = Instant::now();
    for result in done_receiver {
        let processed = result?;
        pending.insert(processed.index, processed);
        while let Some(processed) = pending.remove(&next_index) {
            for record in &processed.records {
                let bucket = (splitmix64(&mut rng) % buckets as u64) as usize;
                writers[bucket].write_all(record)?;
                bucket_records[bucket] += 1;
            }
            for line in &processed.probe_lines {
                writeln!(probe_writer, "{line}")?;
            }
            counts.merge(processed.counts);
            next_index += 1;
        }
        if last_report.elapsed().as_secs() >= 60 {
            eprintln!(
                "ngnk4archive: decoded={} emitted={} elapsed={:.0}s",
                counts.decoded,
                counts.emitted,
                started.elapsed().as_secs_f64()
            );
            last_report = Instant::now();
        }
    }
    let chains = reader
        .join()
        .map_err(|_| anyhow::anyhow!("reader thread panicked"))??;
    for handle in worker_handles {
        handle
            .join()
            .map_err(|_| anyhow::anyhow!("worker thread panicked"))?;
    }
    if !pending.is_empty() {
        bail!("{} processed batches never became contiguous", pending.len());
    }
    for writer in &mut writers {
        writer.flush()?;
        writer.get_ref().sync_all()?;
    }
    probe_writer.flush()?;
    probe_writer.get_ref().sync_all()?;
    if bucket_records.iter().sum::<u64>() != counts.emitted {
        bail!("bucket record total differs from emitted count");
    }
    eprintln!("ngnk4archive: complete chains={chains} decoded={} emitted={}", counts.decoded, counts.emitted);
    let scatter_receipt = ScatterReceipt {
        schema: SCATTER_SCHEMA,
        state: "COMPLETE",
        command: env::args().collect(),
        source_revision: SOURCE_REVISION,
        source,
        conflicts: receipt(&conflicts_path)?,
        probe_keys: receipt(&probes_path)?,
        split_seed: SPLIT_SEED,
        bucket_seed: seed,
        buckets,
        workers,
        keep_per_mille,
        record_contract: "bulletformat ChessBoard 32B; STM-relative raw archive score i16 at 24; STM-relative result+1 at 26",
        eligibility: "ngn-k4-sampler-v2 rejection_reasons empty; chain_split == train; K4 key not in conflicts; u16le(K4 key[0..2]) % 1000 < keep_per_mille",
        counts,
        bucket_records,
        probe_csv: receipt(&probe_path)?,
        elapsed_seconds: started.elapsed().as_secs_f64(),
    };
    let mut bytes = serde_json::to_vec_pretty(&scatter_receipt)?;
    bytes.push(b'\n');
    fs::write(output.join("scatter.json"), bytes)?;
    fs::write(output.join("STATE"), "SCATTERED\n")?;
    Ok(())
}

#[derive(Serialize)]
struct CorpusManifest {
    schema: &'static str,
    state: &'static str,
    command: Vec<String>,
    scatter: FileReceipt,
    scatters: Vec<GatherInput>,
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

#[derive(Serialize)]
struct GatherInput {
    scatter: FileReceipt,
    raw_scale: f64,
}

fn piecewise(knots: &[[f64; 2]], raw: f64) -> f64 {
    let first = knots[0];
    let last = knots[knots.len() - 1];
    if raw <= first[0] {
        return first[1];
    }
    if raw >= last[0] {
        return last[1];
    }
    let index = knots.partition_point(|knot| knot[0] <= raw);
    let [x0, y0] = knots[index - 1];
    let [x1, y1] = knots[index];
    y0 + (y1 - y0) * (raw - x0) / (x1 - x0)
}

fn gather(args: &[String]) -> Result<()> {
    let started = Instant::now();
    let values = flags(args);
    // --scatter DIR[@RAW_SCALE][,DIR[@RAW_SCALE]...]: bucket i of every input
    // is merged, each raw score multiplied by its input's scale before the map.
    let mut inputs = Vec::new();
    for item in required(&values, "--scatter").split(',') {
        let (dir, scale) = match item.split_once('@') {
            Some((dir, scale)) => (PathBuf::from(dir), scale.parse::<f64>()?),
            None => (PathBuf::from(item), 1.0),
        };
        if !scale.is_finite() || scale <= 0.0 {
            bail!("raw scale must be finite and positive");
        }
        inputs.push((dir, scale));
    }
    let map_path = PathBuf::from(required(&values, "--map"));
    let output = PathBuf::from(required(&values, "--output"));
    let manifest_path = PathBuf::from(required(&values, "--manifest"));
    let seed: u64 = required(&values, "--seed").parse()?;
    let map: serde_json::Value = serde_json::from_slice(&fs::read(&map_path)?)?;
    let knots: Vec<[f64; 2]> = serde_json::from_value(map["knots"].clone())?;
    if knots.len() < 2
        || knots.iter().any(|[x, y]| !x.is_finite() || !y.is_finite())
        || knots.windows(2).any(|pair| pair[1][0] <= pair[0][0] || pair[1][1] < pair[0][1])
    {
        bail!("score map knots must be finite, strictly increasing in raw and non-decreasing in score");
    }
    let mut bucket_records: Vec<Vec<u64>> = Vec::new();
    let mut gather_inputs = Vec::new();
    for (dir, scale) in &inputs {
        if fs::read_to_string(dir.join("STATE"))? != "SCATTERED\n" {
            bail!("scatter directory {} is not complete", dir.display());
        }
        let scatter_path = dir.join("scatter.json");
        let scatter: serde_json::Value = serde_json::from_slice(&fs::read(&scatter_path)?)?;
        if scatter["schema"] != SCATTER_SCHEMA || scatter["state"] != "COMPLETE" {
            bail!("scatter receipt contract mismatch");
        }
        let records: Vec<u64> = serde_json::from_value(scatter["bucket_records"].clone())?;
        if !bucket_records.is_empty() && records.len() != bucket_records[0].len() {
            bail!("scatter inputs have different bucket counts");
        }
        bucket_records.push(records);
        gather_inputs.push(GatherInput { scatter: receipt(&scatter_path)?, raw_scale: *scale });
    }
    if manifest_path.exists() {
        bail!("refusing existing manifest {}", manifest_path.display());
    }
    let mut writer = BufWriter::with_capacity(
        8 << 20,
        OpenOptions::new().create_new(true).write(true).open(&output)?,
    );
    let mut digest = Sha256::new();
    let mut records = 0u64;
    let mut dropped = 0u64;
    let mut sentinels = 0u64;
    let mut rng = seed;
    for bucket in 0..bucket_records[0].len() {
        let mut kept: Vec<[u8; RECORD_BYTES]> = Vec::new();
        for ((dir, scale), records) in inputs.iter().zip(&bucket_records) {
            let expected = records[bucket];
            let path = dir.join(format!("bucket-{bucket:04}.bin"));
            let mut data = Vec::new();
            BufReader::new(File::open(&path)?).read_to_end(&mut data)?;
            if data.len() as u64 != expected * RECORD_BYTES as u64 {
                bail!("{} has {} bytes, expected {} records", path.display(), data.len(), expected);
            }
            kept.reserve(expected as usize);
            for chunk in data.chunks_exact(RECORD_BYTES) {
                let mut record: [u8; RECORD_BYTES] = chunk.try_into().unwrap();
                let raw = i16::from_le_bytes([record[24], record[25]]);
                if raw.unsigned_abs() >= SENTINEL_ABS_RAW {
                    sentinels += 1;
                    continue;
                }
                let converted = piecewise(&knots, f64::from(raw) * scale).round();
                if converted.abs() > MAX_ABS_NGN_SCORE {
                    dropped += 1;
                    continue;
                }
                record[24..26].copy_from_slice(&(converted as i16).to_le_bytes());
                kept.push(record);
            }
        }
        for index in (1..kept.len()).rev() {
            let other = (splitmix64(&mut rng) % (index as u64 + 1)) as usize;
            kept.swap(index, other);
        }
        for record in &kept {
            writer.write_all(record)?;
            digest.update(record);
        }
        records += kept.len() as u64;
        eprintln!("ngnk4archive: gathered bucket {bucket} records={records}");
    }
    writer.flush()?;
    writer.get_ref().sync_all()?;
    drop(writer);
    let (bytes, sha256) = sha256_file(&output)?;
    if bytes != records * RECORD_BYTES as u64 || sha256 != hex(digest.finalize()) {
        bail!("written training BF does not match streamed digest");
    }
    let manifest = CorpusManifest {
        schema: CORPUS_SCHEMA,
        state: "COMPLETE",
        command: env::args().collect(),
        scatter: gather_inputs[0].scatter.clone(),
        scatters: gather_inputs,
        score_map: receipt(&map_path)?,
        knots,
        score_contract: format!(
            "drop |raw| >= {SENTINEL_ABS_RAW} (VALUE_NONE); ngn_score = round(piecewise-linear knots(raw * input raw_scale)), clamped to outer knots; |ngn_score| <= {MAX_ABS_NGN_SCORE}; STM; target = 1/(1+exp(-ngn_score/400))"
        ),
        shuffle_seed: seed,
        records,
        record_bytes: RECORD_BYTES,
        dropped_sentinel: sentinels,
        dropped_abs_score: dropped,
        train: FileReceipt {
            path: output.display().to_string(),
            bytes,
            sha256,
        },
        elapsed_seconds: started.elapsed().as_secs_f64(),
    };
    let mut encoded = serde_json::to_vec_pretty(&manifest)?;
    encoded.push(b'\n');
    let mut file = OpenOptions::new().create_new(true).write(true).open(&manifest_path)?;
    file.write_all(&encoded)?;
    file.sync_all()?;
    Ok(())
}

fn main() -> Result<()> {
    let args: Vec<String> = env::args().collect();
    match args.get(1).map(String::as_str) {
        Some("scatter") => scatter(&args[2..]),
        Some("gather") => gather(&args[2..]),
        _ => usage(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use sfbinpack::chess::{coords::Square, r#move::Move, r#move::MoveType, piece::Piece, position::Position};

    fn entry(fen: &str, score: i16, result: i16) -> TrainingDataEntry {
        TrainingDataEntry {
            pos: Position::from_fen(fen).unwrap(),
            mv: Move::new(Square::new(12), Square::new(28), MoveType::Normal, Piece::none()),
            score,
            ply: 20,
            result,
        }
    }

    #[test]
    fn piecewise_interpolates_and_clamps() {
        let knots = [[-100.0, -50.0], [0.0, 4.0], [200.0, 104.0]];
        assert_eq!(piecewise(&knots, -500.0), -50.0);
        assert_eq!(piecewise(&knots, -50.0), -23.0);
        assert_eq!(piecewise(&knots, 0.0), 4.0);
        assert_eq!(piecewise(&knots, 100.0), 54.0);
        assert_eq!(piecewise(&knots, 30_000.0), 104.0);
    }

    #[test]
    fn pack_is_side_to_move_relative() {
        let white = pack(&entry("4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", 57, 1)).unwrap();
        // Occupancy e1, e2, e8; pieces: white king, white pawn, black king.
        assert_eq!(u64::from_le_bytes(white[..8].try_into().unwrap()), (1 << 4) | (1 << 12) | (1 << 60));
        assert_eq!(white[8], 0x05);
        assert_eq!(white[9], 0x0d);
        assert_eq!(i16::from_le_bytes([white[24], white[25]]), 57);
        assert_eq!(white[26], 2);
        assert_eq!((white[27], white[28]), (4, 60 ^ 56));

        // Same material with Black to move: board flips, colours swap, and the
        // stored score/result stay relative to the mover exactly as recorded.
        let black = pack(&entry("4k3/8/8/8/8/8/4P3/4K3 b - - 0 1", -57, -1)).unwrap();
        assert_eq!(
            u64::from_le_bytes(black[..8].try_into().unwrap()),
            (1 << 4) | (1 << 52) | (1 << 60)
        );
        // e1: black-to-move's own king (was e8), e7: opponent pawn, e8: opponent king.
        assert_eq!(black[8], 0x85);
        assert_eq!(black[9], 0x0d);
        assert_eq!(i16::from_le_bytes([black[24], black[25]]), -57);
        assert_eq!(black[26], 0);
        assert_eq!((black[27], black[28]), (4, 60 ^ 56));
    }
}
