use std::{
    collections::BTreeMap,
    env,
    fs::{self, File, OpenOptions},
    io::{BufWriter, Write},
    panic::{AssertUnwindSafe, catch_unwind},
    path::{Path, PathBuf},
    time::Instant,
};

use anyhow::{Context, Result, bail};
use ngn_k4_sample::{
    CHAIN_RECORD_BYTES, CONTRACT_VERSION, ChainRecord, DEFAULT_RUN_RECORDS, K4_INPUT_DOMAIN,
    KEY_RECORD_BYTES, KeyRecord, MAX_OBSERVED_CHAIN, MAX_SCAN_SECONDS, POSITION_PRIORITY_DOMAIN,
    SOURCE_BYTES, SOURCE_REVISION, SOURCE_SHA256, SPLIT_DOMAIN, SPLIT_SEED, ScanChunkReceipt,
    ScanManifest, chain_priority, chain_split, decode_hex_32, hex, k4_input_key, receipt,
    rejection_reasons, sha256_file,
};
use sfbinpack::CompressedTrainingDataEntryReader;

const CHUNK_SCHEMA: &str = "ngn-k4-scan-chunk-v1";
const MANIFEST_SCHEMA: &str = "ngn-k4-scan-manifest-v1";

struct Args {
    input: PathBuf,
    output: PathBuf,
    run_records: usize,
    max_seconds: u64,
    exact: Vec<String>,
}

fn usage() -> ! {
    eprintln!(
        "usage: ngnk4scan --input SOURCE.binpack --output NEW_OR_RESUMABLE_DIR --run-records 1000000 --max-seconds 14400"
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
    let input = PathBuf::from(values.remove("--input").context("missing --input")?);
    let output = PathBuf::from(values.remove("--output").context("missing --output")?);
    let run_records = values
        .remove("--run-records")
        .context("missing --run-records")?
        .parse::<usize>()?;
    if run_records != DEFAULT_RUN_RECORDS {
        bail!("--run-records is frozen at {DEFAULT_RUN_RECORDS}, got {run_records}");
    }
    let max_seconds = values
        .remove("--max-seconds")
        .context("missing --max-seconds")?
        .parse::<u64>()?;
    if max_seconds != MAX_SCAN_SECONDS {
        bail!("--max-seconds is frozen at {MAX_SCAN_SECONDS}, got {max_seconds}");
    }
    if !values.is_empty() {
        bail!("unknown arguments: {:?}", values.keys().collect::<Vec<_>>());
    }
    Ok(Args {
        input,
        output,
        run_records,
        max_seconds,
        exact,
    })
}

fn panic_text(payload: Box<dyn std::any::Any + Send>) -> String {
    if let Some(message) = payload.downcast_ref::<&str>() {
        (*message).to_string()
    } else if let Some(message) = payload.downcast_ref::<String>() {
        message.clone()
    } else {
        "non-string panic payload".to_string()
    }
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

fn write_json_new(path: &Path, value: &impl serde::Serialize) -> Result<()> {
    let mut data = serde_json::to_vec_pretty(value)?;
    data.push(b'\n');
    write_new(path, &data)
}

fn flush_chunk(
    output: &Path,
    chunk_index: u32,
    source_start: u64,
    chain_start: u64,
    keys: &mut Vec<KeyRecord>,
    chains: &mut Vec<ChainRecord>,
    rejection_counts: &mut BTreeMap<String, u64>,
) -> Result<ScanChunkReceipt> {
    if chains.is_empty() {
        bail!("cannot flush an empty scan chunk");
    }
    keys.sort_unstable();
    let stem = format!("chunk-{chunk_index:06}");
    let keys_path = output.join(format!("{stem}.keys.bin"));
    let chains_path = output.join(format!("{stem}.chains.bin"));
    let chunk_path = output.join(format!("{stem}.json"));

    let mut key_writer = BufWriter::new(
        OpenOptions::new()
            .create_new(true)
            .write(true)
            .open(&keys_path)?,
    );
    for record in keys.iter().copied() {
        key_writer.write_all(&record.encode())?;
    }
    key_writer.flush()?;
    key_writer.get_ref().sync_all()?;

    let mut chain_writer = BufWriter::new(
        OpenOptions::new()
            .create_new(true)
            .write(true)
            .open(&chains_path)?,
    );
    for record in chains.iter().copied() {
        chain_writer.write_all(&record.encode())?;
    }
    chain_writer.flush()?;
    chain_writer.get_ref().sync_all()?;

    let source_end = chains
        .last()
        .map(|chain| chain.source_start + u64::from(chain.decoded))
        .unwrap();
    let chain_end = chains.last().unwrap().chain + 1;
    let receipt = ScanChunkReceipt {
        schema: CHUNK_SCHEMA.to_string(),
        chunk: chunk_index,
        source_position_start: source_start,
        source_position_end: source_end,
        chain_start,
        chain_end,
        decoded_positions: source_end - source_start,
        eligible_positions: keys.len() as u64,
        rejection_counts: std::mem::take(rejection_counts),
        keys: receipt(&keys_path)?,
        chains: receipt(&chains_path)?,
    };
    if receipt.keys.bytes % KEY_RECORD_BYTES as u64 != 0
        || receipt.chains.bytes % CHAIN_RECORD_BYTES as u64 != 0
    {
        bail!("internal fixed-record byte-count mismatch");
    }
    write_json_new(&chunk_path, &receipt)?;
    keys.clear();
    chains.clear();
    Ok(receipt)
}

fn load_chunks(output: &Path) -> Result<Vec<ScanChunkReceipt>> {
    let mut chunks = Vec::new();
    for index in 0u32.. {
        let path = output.join(format!("chunk-{index:06}.json"));
        if !path.exists() {
            break;
        }
        let chunk: ScanChunkReceipt = serde_json::from_reader(File::open(&path)?)?;
        if chunk.schema != CHUNK_SCHEMA || chunk.chunk != index {
            bail!("invalid chunk receipt {}", path.display());
        }
        if chunk.chain_start
            != chunks
                .last()
                .map_or(0, |prior: &ScanChunkReceipt| prior.chain_end)
            || chunk.source_position_start
                != chunks.last().map_or(0, |prior| prior.source_position_end)
        {
            bail!("noncontiguous chunk receipt {}", path.display());
        }
        for item in [&chunk.keys, &chunk.chains] {
            let (bytes, sha) = sha256_file(Path::new(&item.path))?;
            if bytes != item.bytes || sha != item.sha256 {
                bail!("chunk artifact receipt mismatch {}", item.path);
            }
        }
        if chunk.keys.bytes / KEY_RECORD_BYTES as u64 != chunk.eligible_positions
            || chunk.chains.bytes / CHAIN_RECORD_BYTES as u64 != chunk.chain_end - chunk.chain_start
        {
            bail!("chunk fixed-record count mismatch {}", path.display());
        }
        chunks.push(chunk);
    }
    Ok(chunks)
}

fn open_output(output: &Path) -> Result<Vec<ScanChunkReceipt>> {
    if !output.exists() {
        fs::create_dir_all(output)?;
        write_new(&output.join("STATE"), b"SCANNING\n")?;
        return Ok(Vec::new());
    }
    let state = fs::read_to_string(output.join("STATE"))?;
    if state != "SCANNING\n" && state != "FAILED\n" {
        bail!("output state is not resumable: {state:?}");
    }
    if output.join("manifest.json").exists() {
        bail!("refusing to resume output with a published manifest");
    }
    fs::write(output.join("STATE"), "SCANNING\n")?;
    load_chunks(output)
}

fn main() -> Result<()> {
    let args = arguments()?;
    let output = args.output.clone();
    let chunks = open_output(&args.output)?;
    let result = catch_unwind(AssertUnwindSafe(|| run(args, chunks)))
        .map_err(|panic| anyhow::anyhow!("scanner panic: {}", panic_text(panic)))
        .and_then(|result| result);
    if let Err(error) = &result {
        let _ = fs::write(output.join("STATE"), "FAILED\n");
        eprintln!("ngnk4scan: {error:#}");
    }
    result
}

fn run(args: Args, mut chunks: Vec<ScanChunkReceipt>) -> Result<()> {
    let started = Instant::now();
    let (source_bytes, source_sha) = sha256_file(&args.input)?;
    if source_bytes != SOURCE_BYTES || source_sha != SOURCE_SHA256 {
        bail!("source identity mismatch: bytes={source_bytes} sha256={source_sha}");
    }
    let source_sha_bytes = decode_hex_32(&source_sha)?;
    let resume_source = chunks.last().map_or(0, |chunk| chunk.source_position_end);
    let resume_chain = chunks.last().map_or(0, |chunk| chunk.chain_end);
    let file = File::open(&args.input)?;
    let mut reader = CompressedTrainingDataEntryReader::new(file)
        .map_err(|error| anyhow::anyhow!("open binpack reader: {error:?}"))?;
    let mut keys = Vec::with_capacity(args.run_records + MAX_OBSERVED_CHAIN);
    let mut chains = Vec::new();
    let mut chain_keys = Vec::new();
    let mut source_position = 0u64;
    let mut chain = 0u64;
    let mut chain_start = 0u64;
    let mut rejections: BTreeMap<String, u64> = BTreeMap::new();
    let mut total_eligible = chunks.iter().map(|chunk| chunk.eligible_positions).sum();
    let mut chunk_source_start = resume_source;
    let mut chunk_chain_start = resume_chain;

    loop {
        let has_next = catch_unwind(AssertUnwindSafe(|| reader.has_next()))
            .map_err(|panic| anyhow::anyhow!("sfbinpack has_next panic: {}", panic_text(panic)))?;
        if !has_next {
            break;
        }
        let (entry, continuation) = catch_unwind(AssertUnwindSafe(|| {
            let entry = reader.next();
            let continuation = reader.is_next_entry_continuation();
            (entry, continuation)
        }))
        .map_err(|panic| {
            anyhow::anyhow!(
                "sfbinpack next panic at source position {source_position}: {}",
                panic_text(panic)
            )
        })?;
        let entry_ordinal: u32 = (source_position - chain_start)
            .try_into()
            .context("chain entry ordinal exceeds u32")?;
        let reasons = rejection_reasons(&entry);
        if reasons.is_empty() && chain >= resume_chain {
            chain_keys.push(KeyRecord {
                key: k4_input_key(&entry.pos)?,
                chain,
                entry: entry_ordinal,
                split: chain_split(&source_sha_bytes, chain),
            });
        } else if chain >= resume_chain {
            for reason in reasons {
                *rejections.entry(reason.to_string()).or_default() += 1;
            }
        }
        source_position += 1;
        if continuation {
            continue;
        }
        let decoded: u32 = (source_position - chain_start)
            .try_into()
            .context("chain length exceeds u32")?;
        if decoded as usize > MAX_OBSERVED_CHAIN {
            bail!("chain {chain} length {decoded} exceeds {MAX_OBSERVED_CHAIN}");
        }
        if chain < resume_chain {
            if source_position > resume_source {
                bail!("resume checkpoint ends inside chain {chain}");
            }
        } else {
            let split = chain_split(&source_sha_bytes, chain);
            let eligible: u32 = chain_keys
                .len()
                .try_into()
                .context("eligible chain length exceeds u32")?;
            total_eligible += u64::from(eligible);
            keys.append(&mut chain_keys);
            chains.push(ChainRecord {
                chain,
                source_start: chain_start,
                decoded,
                eligible,
                split,
                priority: chain_priority(&source_sha_bytes, chain),
            });
            if keys.len() >= args.run_records {
                let chunk = flush_chunk(
                    &args.output,
                    chunks.len().try_into()?,
                    chunk_source_start,
                    chunk_chain_start,
                    &mut keys,
                    &mut chains,
                    &mut rejections,
                )?;
                chunk_source_start = chunk.source_position_end;
                chunk_chain_start = chunk.chain_end;
                chunks.push(chunk);
            }
        }
        chain += 1;
        chain_start = source_position;
        if started.elapsed().as_secs() >= args.max_seconds {
            if !chains.is_empty() {
                let chunk = flush_chunk(
                    &args.output,
                    chunks.len().try_into()?,
                    chunk_source_start,
                    chunk_chain_start,
                    &mut keys,
                    &mut chains,
                    &mut rejections,
                )?;
                chunks.push(chunk);
            }
            bail!(
                "four-hour scan cap reached at complete chain {chain}; finished chunks remain resumable"
            );
        }
    }
    if !chain_keys.is_empty() {
        bail!("source ended inside an encoded chain");
    }
    if source_position != resume_source && chain < resume_chain {
        bail!("source ended before resume checkpoint");
    }
    if !chains.is_empty() {
        let chunk = flush_chunk(
            &args.output,
            chunks.len().try_into()?,
            chunk_source_start,
            chunk_chain_start,
            &mut keys,
            &mut chains,
            &mut rejections,
        )?;
        chunks.push(chunk);
    }
    if chunks.last().map_or(0, |chunk| chunk.source_position_end) != source_position
        || chunks.last().map_or(0, |chunk| chunk.chain_end) != chain
    {
        bail!("final scan extent differs from chunk receipts");
    }
    let source = ngn_k4_sample::FileReceipt {
        path: args.input.display().to_string(),
        bytes: source_bytes,
        sha256: source_sha,
    };
    let mut all_rejections = BTreeMap::new();
    for chunk in &chunks {
        for (reason, count) in &chunk.rejection_counts {
            *all_rejections.entry(reason.clone()).or_default() += count;
        }
    }
    let manifest = ScanManifest {
        schema: MANIFEST_SCHEMA.to_string(),
        contract_version: CONTRACT_VERSION.to_string(),
        state: "COMPLETE".to_string(),
        source_revision: SOURCE_REVISION.to_string(),
        source,
        split_seed: SPLIT_SEED,
        split_domain_hex: hex(SPLIT_DOMAIN),
        chain_priority_domain_hex: hex(ngn_k4_sample::CHAIN_PRIORITY_DOMAIN),
        position_priority_domain_hex: hex(POSITION_PRIORITY_DOMAIN),
        k4_input_domain_hex: hex(K4_INPUT_DOMAIN),
        sfbinpack_version: "0.6.4".to_string(),
        exact_argv: args.exact,
        run_records: args.run_records,
        max_scan_seconds: args.max_seconds,
        decoded_positions: source_position,
        complete_chains: chain,
        eligible_positions: total_eligible,
        rejection_counts: all_rejections,
        chunks,
    };
    write_json_new(&args.output.join("manifest.json"), &manifest)?;
    fs::write(args.output.join("STATE"), "COMPLETE\n")?;
    Ok(())
}
