use std::{
    collections::{BTreeMap, BTreeSet},
    env,
    fs::{self, File, OpenOptions},
    io::{BufRead, BufReader, BufWriter, Write},
    panic::{AssertUnwindSafe, catch_unwind},
    path::{Path, PathBuf},
};

use anyhow::{Context, Result, bail};
use ngn_k4_sample::fixed::{ChainID, ExternalSorter, RecordMerger, RecordReader};
use ngn_k4_sample::{
    CHAIN_PRIORITY_DOMAIN, CHAIN_RECORD_BYTES, CONTRACT_VERSION, CandidateRecord, ChainRecord,
    ConflictRecord, DEFAULT_RUN_RECORDS, FileReceipt, K4_INPUT_DOMAIN, KEY_RECORD_BYTES,
    MAX_SCAN_SECONDS, OwnerRecord, POSITION_ID_DOMAIN, POSITION_PRIORITY_DOMAIN, SOURCE_BYTES,
    SOURCE_SHA256, SPLIT_DOMAIN, SPLIT_SEED, ScanManifest, SelectedChain, Split, chain_priority,
    decode_hex_32, hex, k4_input_key, position_id, position_priority, receipt, rejection_reasons,
    sha256_file,
};
use serde::{Deserialize, Serialize};
use sfbinpack::CompressedTrainingDataEntryReader;

const SELECTION_SCHEMA: &str = "ngn-k4-selection-v3";
const OUTPUT_SCHEMA: &str = "ngn-k4-sampler-output-v2";
const INPUT_SCHEMA: &str = "ngn-k4-label-input-v2";
const PILOT_ACCEPTED: u64 = 1_000_000;
const MAIN_ACCEPTED: u64 = 20_000_000;
const HOLDOUT_ACCEPTED: u64 = 100_000;
const PILOT_CANDIDATES: u64 = 1_250_000;
const MAIN_CANDIDATES: u64 = 25_000_000;
const HOLDOUT_CANDIDATES: u64 = 125_000;
const MAX_QUARANTINE_CHAINS: usize = 50_000_000;
const MAX_SHARD_RECORDS: usize = 100_000;

struct Args {
    scan_manifest: PathBuf,
    historical_positions: PathBuf,
    source: PathBuf,
    output: PathBuf,
    sort_records: usize,
    exact: Vec<String>,
}

fn usage() -> ! {
    eprintln!(
        "usage: ngnk4select --scan-manifest SCAN/manifest.json --historical-positions positions.jsonl --source SOURCE.binpack --output NEW_DIR --pilot-candidates 1250000 --main-candidates 25000000 --holdout-candidates 125000 --sort-records 1000000"
    );
    std::process::exit(2)
}

fn arguments() -> Result<Args> {
    let exact: Vec<String> = env::args().collect();
    let mut values = BTreeMap::new();
    let mut index = 1;
    while index < exact.len() {
        if index + 1 >= exact.len() || !exact[index].starts_with("--") {
            usage();
        }
        if values
            .insert(exact[index].clone(), exact[index + 1].clone())
            .is_some()
        {
            bail!("duplicate argument {}", exact[index]);
        }
        index += 2;
    }
    for (name, frozen) in [
        ("--pilot-candidates", PILOT_CANDIDATES),
        ("--main-candidates", MAIN_CANDIDATES),
        ("--holdout-candidates", HOLDOUT_CANDIDATES),
        ("--sort-records", DEFAULT_RUN_RECORDS as u64),
    ] {
        let actual = values
            .remove(name)
            .with_context(|| format!("missing {name}"))?
            .parse::<u64>()?;
        if actual != frozen {
            bail!("{name} is frozen at {frozen}, got {actual}");
        }
    }
    let args = Args {
        scan_manifest: PathBuf::from(
            values
                .remove("--scan-manifest")
                .context("missing --scan-manifest")?,
        ),
        historical_positions: PathBuf::from(
            values
                .remove("--historical-positions")
                .context("missing --historical-positions")?,
        ),
        source: PathBuf::from(values.remove("--source").context("missing --source")?),
        output: PathBuf::from(values.remove("--output").context("missing --output")?),
        sort_records: DEFAULT_RUN_RECORDS,
        exact,
    };
    if !values.is_empty() {
        bail!("unknown arguments: {:?}", values.keys().collect::<Vec<_>>());
    }
    Ok(args)
}

fn write_new(path: &Path, bytes: &[u8]) -> Result<()> {
    let mut file = OpenOptions::new()
        .create_new(true)
        .write(true)
        .open(path)
        .with_context(|| format!("create new {}", path.display()))?;
    file.write_all(bytes)?;
    file.sync_all()?;
    Ok(())
}

fn write_json_new(path: &Path, value: &impl Serialize) -> Result<()> {
    let mut data = serde_json::to_vec_pretty(value)?;
    data.push(b'\n');
    write_new(path, &data)
}

fn validate_receipt(item: &FileReceipt) -> Result<()> {
    let (bytes, sha) = sha256_file(Path::new(&item.path))?;
    if bytes != item.bytes || sha != item.sha256 {
        bail!("file receipt mismatch {}", item.path);
    }
    Ok(())
}

fn load_scan(path: &Path) -> Result<(ScanManifest, FileReceipt)> {
    let scan_receipt = receipt(path)?;
    let scan: ScanManifest = serde_json::from_reader(File::open(path)?)?;
    if scan.schema != "ngn-k4-scan-manifest-v1"
        || scan.contract_version != CONTRACT_VERSION
        || scan.state != "COMPLETE"
        || scan.source.bytes != SOURCE_BYTES
        || scan.source.sha256 != SOURCE_SHA256
        || scan.split_seed != SPLIT_SEED
        || scan.split_domain_hex != hex(SPLIT_DOMAIN)
        || scan.chain_priority_domain_hex != hex(CHAIN_PRIORITY_DOMAIN)
        || scan.position_priority_domain_hex != hex(POSITION_PRIORITY_DOMAIN)
        || scan.k4_input_domain_hex != hex(K4_INPUT_DOMAIN)
        || scan.sfbinpack_version != "0.6.4"
        || scan.run_records != DEFAULT_RUN_RECORDS
        || scan.max_scan_seconds != MAX_SCAN_SECONDS
    {
        bail!("scan manifest contract mismatch");
    }
    validate_receipt(&scan.source)?;
    let mut source_end = 0u64;
    let mut chain_end = 0u64;
    let mut eligible = 0u64;
    for (index, chunk) in scan.chunks.iter().enumerate() {
        if chunk.schema != "ngn-k4-scan-chunk-v1"
            || chunk.chunk != index as u32
            || chunk.source_position_start != source_end
            || chunk.chain_start != chain_end
        {
            bail!("noncontiguous scan chunk {index}");
        }
        validate_receipt(&chunk.keys)?;
        validate_receipt(&chunk.chains)?;
        if chunk.keys.bytes % KEY_RECORD_BYTES as u64 != 0
            || chunk.chains.bytes % CHAIN_RECORD_BYTES as u64 != 0
            || chunk.keys.bytes / KEY_RECORD_BYTES as u64 != chunk.eligible_positions
            || chunk.chains.bytes / CHAIN_RECORD_BYTES as u64 != chunk.chain_end - chunk.chain_start
        {
            bail!("scan chunk {index} fixed-record mismatch");
        }
        source_end = chunk.source_position_end;
        chain_end = chunk.chain_end;
        eligible += chunk.eligible_positions;
    }
    if source_end != scan.decoded_positions
        || chain_end != scan.complete_chains
        || eligible != scan.eligible_positions
    {
        bail!("scan manifest totals mismatch");
    }
    Ok((scan, scan_receipt))
}

#[derive(Deserialize)]
struct HistoricalPosition {
    source_fen: String,
    tentative_split: String,
    eligible: bool,
}

fn historical_keys(path: &Path, output: &Path) -> Result<(BTreeSet<[u8; 32]>, FileReceipt)> {
    let reader = BufReader::new(File::open(path)?);
    let mut keys = BTreeSet::new();
    for (index, line) in reader.lines().enumerate() {
        let line = line.with_context(|| format!("historical line {}", index + 1))?;
        let item: HistoricalPosition = serde_json::from_str(&line)
            .with_context(|| format!("historical JSON line {}", index + 1))?;
        if !item.eligible
            || (item.tentative_split != "validation" && item.tentative_split != "sealed-test")
        {
            continue;
        }
        let position = catch_unwind(AssertUnwindSafe(|| {
            sfbinpack::chess::position::Position::from_fen(&item.source_fen)
        }))
        .map_err(|_| anyhow::anyhow!("historical FEN panic at line {}", index + 1))?
        .map_err(|error| anyhow::anyhow!("historical FEN line {}: {error:?}", index + 1))?;
        keys.insert(k4_input_key(&position)?);
    }
    let mut writer = BufWriter::new(
        OpenOptions::new()
            .create_new(true)
            .write(true)
            .open(output)?,
    );
    for key in &keys {
        writer.write_all(key)?;
    }
    writer.flush()?;
    writer.get_ref().sync_all()?;
    Ok((keys, receipt(output)?))
}

fn key_paths(scan: &ScanManifest) -> Vec<PathBuf> {
    scan.chunks
        .iter()
        .map(|chunk| PathBuf::from(&chunk.keys.path))
        .collect()
}

fn write_conflicts(
    scan: &ScanManifest,
    historical: &BTreeSet<[u8; 32]>,
    output: &Path,
) -> Result<(u64, u64)> {
    let mut merger = RecordMerger::<ngn_k4_sample::KeyRecord>::open(&key_paths(scan), true)?;
    let mut writer = BufWriter::new(
        OpenOptions::new()
            .create_new(true)
            .write(true)
            .open(output)?,
    );
    let mut current_key = None;
    let mut split_mask = 0u8;
    let mut cross = 0u64;
    let mut historical_count = 0u64;
    let mut write_group = |key: [u8; 32], mask: u8| -> Result<()> {
        let cross_split = mask.count_ones() > 1;
        let historical_holdout = historical.contains(&key);
        if cross_split || historical_holdout {
            let item = ConflictRecord {
                key,
                cross_split,
                historical_holdout,
            };
            writer.write_all(&item.encode())?;
            cross += u64::from(cross_split);
            historical_count += u64::from(historical_holdout);
        }
        Ok(())
    };
    while let Some(record) = merger.next_record()? {
        if current_key.is_some_and(|key| key != record.key) {
            write_group(current_key.unwrap(), split_mask)?;
            split_mask = 0;
        }
        current_key = Some(record.key);
        split_mask |= 1 << record.split as u8;
    }
    if let Some(key) = current_key {
        write_group(key, split_mask)?;
    }
    writer.flush()?;
    writer.get_ref().sync_all()?;
    Ok((cross, historical_count))
}

fn quarantine_chains(
    scan: &ScanManifest,
    conflicts_path: &Path,
    output: &Path,
    sort_records: usize,
    scratch: &Path,
) -> Result<(Vec<u64>, u64)> {
    let mut keys = RecordMerger::<ngn_k4_sample::KeyRecord>::open(&key_paths(scan), true)?;
    let mut conflicts = RecordReader::<ConflictRecord>::open(conflicts_path)?;
    let mut conflict = conflicts.next_record()?;
    let mut sorter = ExternalSorter::<ChainID>::new(scratch, "quarantine", sort_records)?;
    while let Some(record) = keys.next_record()? {
        while conflict.is_some_and(|item| item.key < record.key) {
            conflict = conflicts.next_record()?;
        }
        if let Some(item) = conflict {
            if item.key == record.key
                && (item.cross_split || (item.historical_holdout && record.split == Split::Train))
            {
                sorter.push(ChainID(record.chain))?;
            }
        }
    }
    let records = sorter.finish(output)?;
    if records as usize > MAX_QUARANTINE_CHAINS {
        bail!(
            "quarantine set has {records} chains, exceeds bounded-memory cap {MAX_QUARANTINE_CHAINS}"
        );
    }
    let mut reader = RecordReader::<ChainID>::open(output)?;
    let mut result = Vec::with_capacity(records as usize);
    while let Some(item) = reader.next_record()? {
        result.push(item.0);
    }
    Ok((result, records))
}

fn for_each_owner<F>(
    scan: &ScanManifest,
    source_sha: &[u8; 32],
    quarantined: &[u64],
    mut visit: F,
) -> Result<u64>
where
    F: FnMut(OwnerRecord) -> Result<()>,
{
    let mut keys = RecordMerger::<ngn_k4_sample::KeyRecord>::open(&key_paths(scan), true)?;
    let mut current_key = None;
    let mut owner: Option<(ngn_k4_sample::KeyRecord, [u8; 32], [u8; 32])> = None;
    let mut count = 0u64;
    let mut finish_group =
        |owner: &mut Option<(ngn_k4_sample::KeyRecord, [u8; 32], [u8; 32])>| -> Result<()> {
            if let Some((record, _, _)) = owner.take() {
                visit(OwnerRecord {
                    chain: record.chain,
                    entry: record.entry,
                    split: record.split,
                    key: record.key,
                })?;
                count += 1;
            }
            Ok(())
        };
    while let Some(record) = keys.next_record()? {
        if current_key.is_some_and(|key| key != record.key) {
            finish_group(&mut owner)?;
        }
        current_key = Some(record.key);
        if quarantined.binary_search(&record.chain).is_ok() {
            continue;
        }
        if owner
            .as_ref()
            .is_some_and(|(prior, _, _)| prior.split != record.split)
        {
            bail!(
                "nonquarantined K4 key crosses splits at chains {} and {}",
                owner.unwrap().0.chain,
                record.chain
            );
        }
        let chain_order = chain_priority(source_sha, record.chain);
        let position_order = position_priority(source_sha, record.chain, record.entry);
        let replace = owner
            .as_ref()
            .is_none_or(|(prior, prior_chain, prior_position)| {
                (chain_order, position_order, record.chain, record.entry)
                    < (*prior_chain, *prior_position, prior.chain, prior.entry)
            });
        if replace {
            owner = Some((record, chain_order, position_order));
        }
    }
    finish_group(&mut owner)?;
    Ok(count)
}

fn count_owners(
    scan: &ScanManifest,
    source_sha: &[u8; 32],
    quarantined: &[u64],
) -> Result<(Vec<u32>, u64)> {
    let chain_count: usize = scan
        .complete_chains
        .try_into()
        .context("chain count exceeds address space")?;
    let mut counts = vec![0u32; chain_count];
    let total = for_each_owner(scan, source_sha, quarantined, |owner| {
        let index: usize = owner.chain.try_into()?;
        let count = counts
            .get_mut(index)
            .context("owner references missing chain")?;
        *count = count.checked_add(1).context("owner count overflow")?;
        Ok(())
    })?;
    Ok((counts, total))
}

fn selected_chain_mask(path: &Path, chain_count: usize) -> Result<(Vec<bool>, u64, u64)> {
    let mut reader = RecordReader::<SelectedChain>::open(path)?;
    let mut mask = vec![false; chain_count];
    let mut chains = 0u64;
    let mut positions = 0u64;
    while let Some(selected) = reader.next_record()? {
        let index: usize = selected.chain.try_into()?;
        let bit = mask
            .get_mut(index)
            .context("selected owner chain exceeds scan chain count")?;
        if *bit {
            bail!("duplicate selected chain {}", selected.chain);
        }
        *bit = true;
        chains += 1;
        positions += u64::from(selected.unique_positions);
    }
    Ok((mask, chains, positions))
}

struct OwnerOutputs<'a> {
    by_chain: &'a Path,
    by_key: &'a Path,
    scratch: &'a Path,
    sort_records: usize,
}

fn write_selected_owners(
    scan: &ScanManifest,
    source_sha: &[u8; 32],
    quarantined: &[u64],
    selected: &[bool],
    outputs: OwnerOutputs<'_>,
) -> Result<(u64, u64)> {
    let mut sorter = ExternalSorter::new(outputs.scratch, "selected-owners", outputs.sort_records)?;
    let mut key_writer = BufWriter::new(
        OpenOptions::new()
            .create_new(true)
            .write(true)
            .open(outputs.by_key)?,
    );
    let mut selected_count = 0u64;
    let total = for_each_owner(scan, source_sha, quarantined, |owner| {
        let index: usize = owner.chain.try_into()?;
        if selected
            .get(index)
            .copied()
            .context("owner references missing selected-mask chain")?
        {
            key_writer.write_all(&owner.encode())?;
            sorter.push(owner)?;
            selected_count += 1;
        }
        Ok(())
    })?;
    key_writer.flush()?;
    key_writer.get_ref().sync_all()?;
    let by_key_count =
        key_writer.get_ref().metadata()?.len() / ngn_k4_sample::OWNER_RECORD_BYTES as u64;
    let by_chain_count = sorter.finish(outputs.by_chain)?;
    if by_key_count != selected_count || by_chain_count != selected_count {
        bail!(
            "selected owner key/chain order counts differ: expected={selected_count} key={by_key_count} chain={by_chain_count}"
        );
    }
    Ok((total, selected_count))
}

fn candidate_paths(output: &Path) -> [PathBuf; 4] {
    [
        output.join("candidates-train.bin"),
        output.join("candidates-validation.bin"),
        output.join("candidates-calibration.bin"),
        output.join("candidates-reserved-test.bin"),
    ]
}

fn build_candidates(
    scan: &ScanManifest,
    owner_counts: &[u32],
    output: &Path,
    sort_records: usize,
    scratch: &Path,
) -> Result<[u64; 4]> {
    let mut sorters = Split::ALL
        .into_iter()
        .map(|split| {
            ExternalSorter::new(
                scratch,
                format!("candidate-{}", split.as_str()),
                sort_records,
            )
        })
        .collect::<Result<Vec<_>>>()?;
    let mut expected_chain = 0u64;
    for chunk in &scan.chunks {
        let mut chains = RecordReader::<ChainRecord>::open(&chunk.chains.path)?;
        while let Some(chain) = chains.next_record()? {
            if chain.chain != expected_chain {
                bail!("chain metadata ordinal mismatch at {expected_chain}");
            }
            let index: usize = chain.chain.try_into()?;
            let count = *owner_counts
                .get(index)
                .context("owner-count vector ended before chain metadata")?;
            if count > 0 {
                sorters[chain.split as usize].push(CandidateRecord {
                    priority: chain.priority,
                    chain: chain.chain,
                    unique_positions: count,
                    split: chain.split,
                })?;
            }
            expected_chain += 1;
        }
    }
    if expected_chain != scan.complete_chains || owner_counts.len() != expected_chain as usize {
        bail!("owner-count or chain stream has unexpected tail");
    }
    let paths = candidate_paths(output);
    let mut counts = [0u64; 4];
    for (index, sorter) in sorters.into_iter().enumerate() {
        counts[index] = sorter.finish(&paths[index])?;
    }
    Ok(counts)
}

#[derive(Clone, Debug, Serialize)]
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

fn select_chains(
    candidate_paths: &[PathBuf; 4],
    output: &Path,
    sort_records: usize,
    scratch: &Path,
    pilot_target: u64,
    main_target: u64,
    holdout_target: u64,
) -> Result<([u64; 5], u64)> {
    let mut selected = ExternalSorter::<SelectedChain>::new(scratch, "selected", sort_records)?;
    let mut counts = [0u64; 5];
    let mut chain_count = 0u64;
    for split in Split::ALL {
        let mut reader = RecordReader::<CandidateRecord>::open(&candidate_paths[split as usize])?;
        while let Some(candidate) = reader.next_record()? {
            if candidate.split != split {
                bail!(
                    "candidate split mismatch in {}",
                    candidate_paths[split as usize].display()
                );
            }
            let index = match split {
                Split::Train => 1,
                Split::Validation => 2,
                Split::Calibration => 3,
                Split::ReservedTest => 4,
            };
            let target = if split == Split::Train {
                main_target
            } else {
                holdout_target
            };
            if counts[index] >= target {
                break;
            }
            let pilot = split == Split::Train && counts[0] < pilot_target;
            let positions = u64::from(candidate.unique_positions);
            if pilot {
                counts[0] += positions;
            }
            counts[index] += positions;
            selected.push(SelectedChain {
                chain: candidate.chain,
                split,
                pilot,
                unique_positions: candidate.unique_positions,
            })?;
            chain_count += 1;
        }
    }
    if counts[0] < pilot_target
        || counts[1] < main_target
        || counts[2..].iter().any(|count| *count < holdout_target)
    {
        bail!("archive has insufficient unique candidates for frozen targets: {counts:?}");
    }
    let written = selected.finish(output)?;
    if written != chain_count {
        bail!("selected-chain dedup changed count {chain_count} to {written}");
    }
    Ok((counts, chain_count))
}

#[derive(Clone, Debug, Serialize)]
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

#[derive(Clone, Debug, Serialize)]
struct InputHeader<'a> {
    r#type: &'static str,
    schema: &'static str,
    shard_id: &'a str,
    split: &'a str,
    source_manifest_sha256: &'a str,
    record_count: usize,
}

#[derive(Clone, Debug, Serialize)]
struct InputPosition {
    r#type: &'static str,
    id: String,
    fen: String,
    source_move: String,
    encoded_chain: u64,
    chain_entry: u32,
    source_position: u64,
    k4_input_sha256: String,
    source_archive_sha256: &'static str,
    source_manifest_ref: String,
}

#[derive(Clone, Debug, Serialize)]
struct ShardReceipt {
    stage: String,
    split: String,
    shard_id: String,
    records: usize,
    file: FileReceipt,
}

struct ShardSeries {
    stage: String,
    split: Split,
    selection_sha: String,
    directory: PathBuf,
    buffer: Vec<InputPosition>,
    receipts: Vec<ShardReceipt>,
}

impl ShardSeries {
    fn new(stage: &str, split: Split, selection_sha: &str, directory: &Path) -> Self {
        Self {
            stage: stage.to_string(),
            split,
            selection_sha: selection_sha.to_string(),
            directory: directory.to_path_buf(),
            buffer: Vec::with_capacity(MAX_SHARD_RECORDS),
            receipts: Vec::new(),
        }
    }

    fn push(&mut self, position: InputPosition) -> Result<()> {
        self.buffer.push(position);
        if self.buffer.len() == MAX_SHARD_RECORDS {
            self.flush()?;
        }
        Ok(())
    }

    fn flush(&mut self) -> Result<()> {
        if self.buffer.is_empty() {
            return Ok(());
        }
        let shard_id = format!(
            "{}-{}-{:06}",
            self.stage,
            self.split.as_str(),
            self.receipts.len()
        );
        let path = self.directory.join(format!("{shard_id}.jsonl"));
        let mut writer = BufWriter::new(
            OpenOptions::new()
                .create_new(true)
                .write(true)
                .open(&path)?,
        );
        serde_json::to_writer(
            &mut writer,
            &InputHeader {
                r#type: "header",
                schema: INPUT_SCHEMA,
                shard_id: &shard_id,
                split: self.split.as_str(),
                source_manifest_sha256: &self.selection_sha,
                record_count: self.buffer.len(),
            },
        )?;
        writer.write_all(b"\n")?;
        let records = self.buffer.len();
        for position in self.buffer.drain(..) {
            serde_json::to_writer(&mut writer, &position)?;
            writer.write_all(b"\n")?;
        }
        writer.flush()?;
        writer.get_ref().sync_all()?;
        self.receipts.push(ShardReceipt {
            stage: self.stage.clone(),
            split: self.split.as_str().to_string(),
            shard_id,
            records,
            file: receipt(&path)?,
        });
        Ok(())
    }
}

#[derive(Clone, Debug, Serialize)]
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

fn emit_shards(
    args: &Args,
    scan: &ScanManifest,
    selection_path: &Path,
    selected_path: &Path,
    owners_path: &Path,
    counts: &[u64; 5],
) -> Result<Vec<ShardReceipt>> {
    let selection = receipt(selection_path)?;
    let shard_dir = args.output.join("shards");
    fs::create_dir(&shard_dir)?;
    let mut series = vec![
        ShardSeries::new("pilot", Split::Train, &selection.sha256, &shard_dir),
        ShardSeries::new(
            "main-expansion",
            Split::Train,
            &selection.sha256,
            &shard_dir,
        ),
        ShardSeries::new("fixed", Split::Validation, &selection.sha256, &shard_dir),
        ShardSeries::new("fixed", Split::Calibration, &selection.sha256, &shard_dir),
        ShardSeries::new("fixed", Split::ReservedTest, &selection.sha256, &shard_dir),
    ];
    let mut selected_reader = RecordReader::<SelectedChain>::open(selected_path)?;
    let mut selected = selected_reader.next_record()?;
    let mut owners = RecordReader::<ngn_k4_sample::OwnerRecord>::open(owners_path)?;
    let mut owner = owners.next_record()?;
    let source_sha = decode_hex_32(SOURCE_SHA256)?;
    let file = File::open(&args.source)?;
    let mut reader = CompressedTrainingDataEntryReader::new(file)
        .map_err(|error| anyhow::anyhow!("open source reader: {error:?}"))?;
    let mut source_position = 0u64;
    let mut chain = 0u64;
    let mut chain_start = 0u64;
    let mut emitted = [0u64; 5];
    let mut emitted_in_chain = 0u32;
    while reader.has_next() {
        let entry = reader.next();
        let continuation = reader.is_next_entry_continuation();
        let entry_ordinal: u32 = (source_position - chain_start).try_into()?;
        if selected.is_some_and(|item| item.chain < chain) {
            bail!(
                "selected chain {} was not observed",
                selected.unwrap().chain
            );
        }
        while owner.is_some_and(|item| item.chain < chain) {
            owner = owners.next_record()?;
        }
        if let (Some(selected_chain), Some(owner_record)) = (selected, owner) {
            if selected_chain.chain == chain
                && owner_record.chain == chain
                && owner_record.entry == entry_ordinal
            {
                let reasons = rejection_reasons(&entry);
                if !reasons.is_empty() {
                    bail!(
                        "selected owner became ineligible at chain {chain} entry {entry_ordinal}: {reasons:?}"
                    );
                }
                let key = k4_input_key(&entry.pos)?;
                if key != owner_record.key || selected_chain.split != owner_record.split {
                    bail!(
                        "selected owner identity mismatch at chain {chain} entry {entry_ordinal}"
                    );
                }
                let base = InputPosition {
                    r#type: "position",
                    id: hex(position_id(&source_sha, chain, entry_ordinal, &key)),
                    fen: entry
                        .pos
                        .fen()
                        .map_err(|error| anyhow::anyhow!("source FEN: {error:?}"))?,
                    source_move: entry.mv.as_uci(),
                    encoded_chain: chain,
                    chain_entry: entry_ordinal,
                    source_position,
                    k4_input_sha256: hex(key),
                    source_archive_sha256: SOURCE_SHA256,
                    source_manifest_ref: selection_path.display().to_string(),
                };
                let series_index = match selected_chain.split {
                    Split::Train if selected_chain.pilot => 0,
                    Split::Train => 1,
                    Split::Validation => 2,
                    Split::Calibration => 3,
                    Split::ReservedTest => 4,
                };
                series[series_index].push(base)?;
                emitted[series_index] += 1;
                emitted_in_chain += 1;
                owner = owners.next_record()?;
            }
        }
        source_position += 1;
        if !continuation {
            if let Some(item) = selected {
                if item.chain == chain {
                    if emitted_in_chain != item.unique_positions {
                        bail!(
                            "selected chain {chain} emitted {emitted_in_chain}, expected {}",
                            item.unique_positions
                        );
                    }
                    selected = selected_reader.next_record()?;
                }
            }
            emitted_in_chain = 0;
            chain += 1;
            chain_start = source_position;
        }
    }
    if selected.is_some()
        || source_position != scan.decoded_positions
        || chain != scan.complete_chains
        || emitted != *counts
    {
        bail!(
            "emission totals mismatch: source={source_position}/{} chains={chain}/{} emitted={emitted:?}/{counts:?}",
            scan.decoded_positions,
            scan.complete_chains
        );
    }
    for item in &mut series {
        item.flush()?;
    }
    Ok(series.into_iter().flat_map(|item| item.receipts).collect())
}

fn main() -> Result<()> {
    let args = arguments()?;
    if args.output.exists() {
        bail!("refusing existing output {}", args.output.display());
    }
    fs::create_dir(&args.output)?;
    write_new(&args.output.join("STATE"), b"SELECTING\n")?;
    let result = run(&args);
    if let Err(error) = &result {
        let _ = fs::write(args.output.join("STATE"), "FAILED\n");
        eprintln!("ngnk4select: {error:#}");
    }
    result
}

fn run(args: &Args) -> Result<()> {
    let (scan, scan_receipt) = load_scan(&args.scan_manifest)?;
    let source_receipt = receipt(&args.source)?;
    if source_receipt.bytes != SOURCE_BYTES || source_receipt.sha256 != SOURCE_SHA256 {
        bail!("source archive identity mismatch");
    }
    if source_receipt.sha256 != scan.source.sha256 {
        bail!("source archive differs from scan");
    }
    let historical_positions_receipt = receipt(&args.historical_positions)?;
    let historical_path = args.output.join("historical-holdout-keys.bin");
    let (historical, historical_receipt) =
        historical_keys(&args.historical_positions, &historical_path)?;
    let conflicts_path = args.output.join("conflicts.bin");
    let (cross_keys, historical_conflicts) = write_conflicts(&scan, &historical, &conflicts_path)?;
    let conflicts_receipt = receipt(&conflicts_path)?;
    let quarantine_path = args.output.join("quarantine-chains.bin");
    let (quarantined, quarantine_count) = quarantine_chains(
        &scan,
        &conflicts_path,
        &quarantine_path,
        args.sort_records,
        &args.output,
    )?;
    let source_sha = decode_hex_32(SOURCE_SHA256)?;
    let (owner_counts, owner_count) = count_owners(&scan, &source_sha, &quarantined)?;
    let candidate_counts = build_candidates(
        &scan,
        &owner_counts,
        &args.output,
        args.sort_records,
        &args.output,
    )?;
    let candidates = candidate_paths(&args.output);
    let selected_path = args.output.join("selected-chains.bin");
    let (selected_counts, selected_chain_count) = select_chains(
        &candidates,
        &selected_path,
        args.sort_records,
        &args.output,
        PILOT_CANDIDATES,
        MAIN_CANDIDATES,
        HOLDOUT_CANDIDATES,
    )?;
    let (selected_mask, selected_file_chains, selected_file_positions) =
        selected_chain_mask(&selected_path, owner_counts.len())?;
    if selected_file_chains != selected_chain_count {
        bail!(
            "selected-chain count changed while loading mask: {selected_file_chains}/{selected_chain_count}"
        );
    }
    let expected_selected_positions = selected_counts[1] + selected_counts[2..].iter().sum::<u64>();
    if selected_file_positions != expected_selected_positions {
        bail!(
            "selected-chain position total differs: {selected_file_positions}/{expected_selected_positions}"
        );
    }
    let owners_path = args.output.join("owners-by-chain.bin");
    let owners_by_key_path = args.output.join("owners-by-key.bin");
    let (second_owner_count, selected_owner_count) = write_selected_owners(
        &scan,
        &source_sha,
        &quarantined,
        &selected_mask,
        OwnerOutputs {
            by_chain: &owners_path,
            by_key: &owners_by_key_path,
            scratch: &args.output,
            sort_records: args.sort_records,
        },
    )?;
    if second_owner_count != owner_count || selected_owner_count != expected_selected_positions {
        bail!(
            "owner replay differs: total={second_owner_count}/{owner_count} selected={selected_owner_count}/{expected_selected_positions}"
        );
    }
    let counts = SelectionCounts {
        historical_holdout_keys: historical.len() as u64,
        cross_split_keys: cross_keys,
        historical_conflict_keys: historical_conflicts,
        quarantined_chains: quarantine_count,
        unique_owner_positions: owner_count,
        selected_owner_positions: selected_owner_count,
        candidate_chains: candidate_counts,
        pilot_candidate_positions: selected_counts[0],
        main_candidate_positions: selected_counts[1],
        validation_candidate_positions: selected_counts[2],
        calibration_candidate_positions: selected_counts[3],
        reserved_test_candidate_positions: selected_counts[4],
        selected_chains: selected_chain_count,
    };
    let selection_path = args.output.join("selection.json");
    let selection = SelectionManifest {
        schema: SELECTION_SCHEMA.to_string(),
        contract_version: CONTRACT_VERSION.to_string(),
        state: "SELECTED".to_string(),
        exact_argv: args.exact.clone(),
        source: source_receipt,
        scan_manifest: scan_receipt,
        historical_positions: historical_positions_receipt,
        historical_keys: historical_receipt,
        conflicts: conflicts_receipt,
        quarantine_chains: receipt(&quarantine_path)?,
        owners_by_chain: receipt(&owners_path)?,
        owners_by_key: receipt(&owners_by_key_path)?,
        selected_chains: receipt(&selected_path)?,
        candidate_files: candidates
            .iter()
            .map(|path| receipt(path))
            .collect::<Result<_>>()?,
        split_seed: SPLIT_SEED,
        split_domain_hex: hex(SPLIT_DOMAIN),
        chain_priority_domain_hex: hex(CHAIN_PRIORITY_DOMAIN),
        position_priority_domain_hex: hex(POSITION_PRIORITY_DOMAIN),
        position_id_domain_hex: hex(POSITION_ID_DOMAIN),
        k4_input_domain_hex: hex(K4_INPUT_DOMAIN),
        pilot_candidate_target: PILOT_CANDIDATES,
        main_candidate_target: MAIN_CANDIDATES,
        holdout_candidate_target: HOLDOUT_CANDIDATES,
        pilot_accepted_target: PILOT_ACCEPTED,
        main_accepted_target: MAIN_ACCEPTED,
        holdout_accepted_target: HOLDOUT_ACCEPTED,
        counts,
    };
    write_json_new(&selection_path, &selection)?;
    fs::write(args.output.join("STATE"), "EMITTING\n")?;
    let expected = [
        selection.counts.pilot_candidate_positions,
        selection.counts.main_candidate_positions - selection.counts.pilot_candidate_positions,
        selection.counts.validation_candidate_positions,
        selection.counts.calibration_candidate_positions,
        selection.counts.reserved_test_candidate_positions,
    ];
    let shards = emit_shards(
        args,
        &scan,
        &selection_path,
        &selected_path,
        &owners_path,
        &expected,
    )?;
    let output = OutputManifest {
        schema: OUTPUT_SCHEMA.to_string(),
        contract_version: CONTRACT_VERSION.to_string(),
        state: "COMPLETE".to_string(),
        selection: receipt(&selection_path)?,
        shards,
        pilot_candidate_positions: expected[0],
        main_expansion_candidate_positions: expected[1],
        total_train_candidate_positions: expected[0] + expected[1],
        validation_candidate_positions: expected[2],
        calibration_candidate_positions: expected[3],
        reserved_test_candidate_positions: expected[4],
        pilot_accepted_target: PILOT_ACCEPTED,
        main_accepted_target: MAIN_ACCEPTED,
        holdout_accepted_target: HOLDOUT_ACCEPTED,
    };
    write_json_new(&args.output.join("manifest.json"), &output)?;
    fs::write(args.output.join("STATE"), "COMPLETE\n")?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use ngn_k4_sample::fixed::FixedRecord;
    use ngn_k4_sample::{KeyRecord, ScanChunkReceipt};
    use std::time::{SystemTime, UNIX_EPOCH};

    fn test_root(label: &str) -> PathBuf {
        let nonce = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let path = std::env::temp_dir().join(format!(
            "ngn-k4-select-{label}-{}-{nonce}",
            std::process::id()
        ));
        fs::create_dir(&path).unwrap();
        path
    }

    fn write_records<T: FixedRecord>(path: &Path, records: &[T]) {
        let mut writer = BufWriter::new(File::create(path).unwrap());
        let mut bytes = vec![0u8; T::BYTES];
        for &record in records {
            record.encode_into(&mut bytes);
            writer.write_all(&bytes).unwrap();
        }
        writer.flush().unwrap();
    }

    fn scan_with_keys(path: &Path, records: &[KeyRecord]) -> ScanManifest {
        let mut sorted = records.to_vec();
        sorted.sort_unstable();
        write_records(path, &sorted);
        let key_receipt = receipt(path).unwrap();
        ScanManifest {
            schema: "ngn-k4-scan-manifest-v1".to_string(),
            contract_version: CONTRACT_VERSION.to_string(),
            state: "COMPLETE".to_string(),
            source_revision: String::new(),
            source: FileReceipt {
                path: String::new(),
                bytes: SOURCE_BYTES,
                sha256: SOURCE_SHA256.to_string(),
            },
            split_seed: SPLIT_SEED,
            split_domain_hex: hex(SPLIT_DOMAIN),
            chain_priority_domain_hex: hex(CHAIN_PRIORITY_DOMAIN),
            position_priority_domain_hex: hex(POSITION_PRIORITY_DOMAIN),
            k4_input_domain_hex: hex(K4_INPUT_DOMAIN),
            sfbinpack_version: "0.6.4".to_string(),
            exact_argv: Vec::new(),
            run_records: DEFAULT_RUN_RECORDS,
            max_scan_seconds: MAX_SCAN_SECONDS,
            chunks: vec![ScanChunkReceipt {
                schema: "ngn-k4-scan-chunk-v1".to_string(),
                chunk: 0,
                source_position_start: 0,
                source_position_end: records.len() as u64,
                chain_start: 0,
                chain_end: 5,
                decoded_positions: records.len() as u64,
                eligible_positions: records.len() as u64,
                rejection_counts: BTreeMap::new(),
                keys: key_receipt,
                chains: FileReceipt {
                    path: String::new(),
                    bytes: 5 * CHAIN_RECORD_BYTES as u64,
                    sha256: String::new(),
                },
            }],
            decoded_positions: records.len() as u64,
            complete_chains: 5,
            eligible_positions: records.len() as u64,
            rejection_counts: BTreeMap::new(),
        }
    }

    #[test]
    fn conflict_quarantine_and_owner_transfer_are_exact() {
        let root = test_root("conflicts");
        let keys_path = root.join("keys.bin");
        let source = [0x55; 32];
        let cross = [0x11; 32];
        let historical = [0x22; 32];
        let shared_train = [0x33; 32];
        let unique_three = [0x44; 32];
        let unique_four = [0x55; 32];
        let scan = scan_with_keys(
            &keys_path,
            &[
                KeyRecord {
                    key: cross,
                    chain: 0,
                    entry: 0,
                    split: Split::Train,
                },
                KeyRecord {
                    key: cross,
                    chain: 1,
                    entry: 0,
                    split: Split::Validation,
                },
                KeyRecord {
                    key: historical,
                    chain: 2,
                    entry: 0,
                    split: Split::Train,
                },
                KeyRecord {
                    key: shared_train,
                    chain: 3,
                    entry: 0,
                    split: Split::Train,
                },
                KeyRecord {
                    key: shared_train,
                    chain: 4,
                    entry: 0,
                    split: Split::Train,
                },
                KeyRecord {
                    key: unique_three,
                    chain: 3,
                    entry: 1,
                    split: Split::Train,
                },
                KeyRecord {
                    key: unique_four,
                    chain: 4,
                    entry: 1,
                    split: Split::Train,
                },
            ],
        );
        let conflicts = root.join("conflicts.bin");
        assert_eq!(
            write_conflicts(&scan, &BTreeSet::from([historical]), &conflicts).unwrap(),
            (1, 1)
        );
        let quarantine = root.join("quarantine.bin");
        let (quarantined, count) =
            quarantine_chains(&scan, &conflicts, &quarantine, 2, &root).unwrap();
        assert_eq!(count, 3);
        assert_eq!(quarantined, vec![0, 1, 2]);
        let owners = root.join("owners.bin");
        let owners_by_key = root.join("owners-key.bin");
        let want_chain = if chain_priority(&source, 3) < chain_priority(&source, 4) {
            3
        } else {
            4
        };
        let (counts, total) = count_owners(&scan, &source, &quarantined).unwrap();
        assert_eq!(total, 3);
        assert_eq!(counts.iter().sum::<u32>(), 3);
        assert_eq!(counts[want_chain as usize], 2);
        let mut selected = vec![false; counts.len()];
        selected[want_chain as usize] = true;
        assert_eq!(
            write_selected_owners(
                &scan,
                &source,
                &quarantined,
                &selected,
                OwnerOutputs {
                    by_chain: &owners,
                    by_key: &owners_by_key,
                    scratch: &root,
                    sort_records: 2,
                },
            )
            .unwrap(),
            (3, 2)
        );
        let mut reader = RecordReader::<OwnerRecord>::open(&owners).unwrap();
        let owner = reader.next_record().unwrap().unwrap();
        assert_eq!(owner.chain, want_chain);
        assert_eq!(owner.key, shared_train);
        let owner = reader.next_record().unwrap().unwrap();
        assert_eq!(owner.chain, want_chain);
        assert_eq!(owner.entry, 1);
        assert!(reader.next_record().unwrap().is_none());
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn pilot_is_whole_chain_prefix_of_main_selection() {
        let root = test_root("selection");
        let paths = candidate_paths(&root);
        for split in Split::ALL {
            let records = if split == Split::Train {
                vec![
                    CandidateRecord {
                        priority: [1; 32],
                        chain: 9,
                        unique_positions: 2,
                        split,
                    },
                    CandidateRecord {
                        priority: [2; 32],
                        chain: 8,
                        unique_positions: 2,
                        split,
                    },
                    CandidateRecord {
                        priority: [3; 32],
                        chain: 7,
                        unique_positions: 2,
                        split,
                    },
                ]
            } else {
                vec![CandidateRecord {
                    priority: [split as u8; 32],
                    chain: 10 + split as u64,
                    unique_positions: 3,
                    split,
                }]
            };
            write_records(&paths[split as usize], &records);
        }
        let selected_path = root.join("selected.bin");
        let (counts, chains) = select_chains(&paths, &selected_path, 2, &root, 3, 5, 2).unwrap();
        assert_eq!(counts, [4, 6, 3, 3, 3]);
        assert_eq!(chains, 6);
        let mut reader = RecordReader::<SelectedChain>::open(&selected_path).unwrap();
        let mut train = Vec::new();
        while let Some(item) = reader.next_record().unwrap() {
            if item.split == Split::Train {
                train.push((item.chain, item.pilot));
            }
        }
        assert_eq!(train, vec![(7, false), (8, true), (9, true)]);
        fs::remove_dir_all(root).unwrap();
    }
}
