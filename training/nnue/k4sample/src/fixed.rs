use std::{
    cmp::Ordering,
    collections::BinaryHeap,
    fs::{File, OpenOptions},
    io::{BufReader, BufWriter, Read, Write},
    marker::PhantomData,
    path::{Path, PathBuf},
};

use anyhow::{Context, Result, bail};

use crate::{
    CANDIDATE_RECORD_BYTES, CHAIN_RECORD_BYTES, CONFLICT_RECORD_BYTES, CandidateRecord,
    ChainRecord, ConflictRecord, KEY_RECORD_BYTES, KeyRecord, OWNER_RECORD_BYTES, OwnerRecord,
    SELECTED_CHAIN_BYTES, SelectedChain,
};

impl FixedRecord for ConflictRecord {
    const BYTES: usize = CONFLICT_RECORD_BYTES;

    fn encode_into(self, output: &mut [u8]) {
        output.copy_from_slice(&self.encode());
    }

    fn decode_from(bytes: &[u8]) -> Result<Self> {
        Self::decode(bytes.try_into().unwrap())
    }
}

pub trait FixedRecord: Copy {
    const BYTES: usize;
    fn encode_into(self, output: &mut [u8]);
    fn decode_from(bytes: &[u8]) -> Result<Self>;
}

impl FixedRecord for ChainRecord {
    const BYTES: usize = CHAIN_RECORD_BYTES;

    fn encode_into(self, output: &mut [u8]) {
        output.copy_from_slice(&self.encode());
    }

    fn decode_from(bytes: &[u8]) -> Result<Self> {
        Self::decode(bytes.try_into().unwrap())
    }
}

impl FixedRecord for KeyRecord {
    const BYTES: usize = KEY_RECORD_BYTES;

    fn encode_into(self, output: &mut [u8]) {
        output.copy_from_slice(&self.encode());
    }

    fn decode_from(bytes: &[u8]) -> Result<Self> {
        Self::decode(bytes.try_into().unwrap())
    }
}

impl FixedRecord for OwnerRecord {
    const BYTES: usize = OWNER_RECORD_BYTES;

    fn encode_into(self, output: &mut [u8]) {
        output.copy_from_slice(&self.encode());
    }

    fn decode_from(bytes: &[u8]) -> Result<Self> {
        Self::decode(bytes.try_into().unwrap())
    }
}

impl FixedRecord for CandidateRecord {
    const BYTES: usize = CANDIDATE_RECORD_BYTES;

    fn encode_into(self, output: &mut [u8]) {
        output.copy_from_slice(&self.encode());
    }

    fn decode_from(bytes: &[u8]) -> Result<Self> {
        Self::decode(bytes.try_into().unwrap())
    }
}

impl FixedRecord for SelectedChain {
    const BYTES: usize = SELECTED_CHAIN_BYTES;

    fn encode_into(self, output: &mut [u8]) {
        output.copy_from_slice(&self.encode());
    }

    fn decode_from(bytes: &[u8]) -> Result<Self> {
        Self::decode(bytes.try_into().unwrap())
    }
}

#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub struct ChainID(pub u64);

impl FixedRecord for ChainID {
    const BYTES: usize = 8;

    fn encode_into(self, output: &mut [u8]) {
        output.copy_from_slice(&self.0.to_le_bytes());
    }

    fn decode_from(bytes: &[u8]) -> Result<Self> {
        Ok(Self(u64::from_le_bytes(bytes.try_into().unwrap())))
    }
}

pub struct RecordReader<T: FixedRecord> {
    reader: BufReader<File>,
    path: PathBuf,
    _record: PhantomData<T>,
}

impl<T: FixedRecord> RecordReader<T> {
    pub fn open(path: impl AsRef<Path>) -> Result<Self> {
        let path = path.as_ref().to_path_buf();
        let file = File::open(&path).with_context(|| format!("open {}", path.display()))?;
        let bytes = file.metadata()?.len();
        if bytes % T::BYTES as u64 != 0 {
            bail!(
                "{} has {} trailing bytes for {}-byte records",
                path.display(),
                bytes % T::BYTES as u64,
                T::BYTES
            );
        }
        Ok(Self {
            reader: BufReader::new(file),
            path,
            _record: PhantomData,
        })
    }

    pub fn next_record(&mut self) -> Result<Option<T>> {
        let mut bytes = vec![0u8; T::BYTES];
        let mut consumed = 0usize;
        while consumed < bytes.len() {
            let count = self.reader.read(&mut bytes[consumed..])?;
            if count == 0 {
                if consumed == 0 {
                    return Ok(None);
                }
                bail!("truncated fixed record in {}", self.path.display());
            }
            consumed += count;
        }
        T::decode_from(&bytes).map(Some)
    }
}

struct HeapItem<T> {
    record: T,
    run: usize,
}

impl<T: Ord> Ord for HeapItem<T> {
    fn cmp(&self, other: &Self) -> Ordering {
        other
            .record
            .cmp(&self.record)
            .then_with(|| other.run.cmp(&self.run))
    }
}

impl<T: Ord> PartialOrd for HeapItem<T> {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl<T: Ord> PartialEq for HeapItem<T> {
    fn eq(&self, other: &Self) -> bool {
        self.record == other.record && self.run == other.run
    }
}

impl<T: Ord> Eq for HeapItem<T> {}

pub struct RecordMerger<T: FixedRecord + Ord> {
    readers: Vec<RecordReader<T>>,
    heap: BinaryHeap<HeapItem<T>>,
    previous: Option<T>,
    require_unique: bool,
}

impl<T: FixedRecord + Ord> RecordMerger<T> {
    pub fn open(paths: &[PathBuf], require_unique: bool) -> Result<Self> {
        if paths.is_empty() {
            bail!("cannot merge zero fixed-record runs");
        }
        let mut readers = paths
            .iter()
            .map(RecordReader::open)
            .collect::<Result<Vec<_>>>()?;
        let mut heap = BinaryHeap::new();
        for (run, reader) in readers.iter_mut().enumerate() {
            if let Some(record) = reader.next_record()? {
                heap.push(HeapItem { record, run });
            }
        }
        Ok(Self {
            readers,
            heap,
            previous: None,
            require_unique,
        })
    }

    pub fn next_record(&mut self) -> Result<Option<T>> {
        let Some(item) = self.heap.pop() else {
            return Ok(None);
        };
        if let Some(previous) = self.previous {
            if item.record < previous || (self.require_unique && item.record == previous) {
                bail!("fixed-record runs are not globally strict-sorted");
            }
        }
        if let Some(next) = self.readers[item.run].next_record()? {
            if next < item.record || (self.require_unique && next == item.record) {
                bail!("fixed-record run {} is not strict-sorted", item.run);
            }
            self.heap.push(HeapItem {
                record: next,
                run: item.run,
            });
        }
        self.previous = Some(item.record);
        Ok(Some(item.record))
    }
}

pub struct ExternalSorter<T: FixedRecord + Ord> {
    directory: PathBuf,
    prefix: String,
    capacity: usize,
    buffer: Vec<T>,
    runs: Vec<PathBuf>,
}

impl<T: FixedRecord + Ord> ExternalSorter<T> {
    pub fn new(
        directory: impl AsRef<Path>,
        prefix: impl Into<String>,
        capacity: usize,
    ) -> Result<Self> {
        if capacity == 0 {
            bail!("external-sort capacity must be positive");
        }
        Ok(Self {
            directory: directory.as_ref().to_path_buf(),
            prefix: prefix.into(),
            capacity,
            buffer: Vec::with_capacity(capacity),
            runs: Vec::new(),
        })
    }

    pub fn push(&mut self, record: T) -> Result<()> {
        self.buffer.push(record);
        if self.buffer.len() >= self.capacity {
            self.flush()?;
        }
        Ok(())
    }

    fn flush(&mut self) -> Result<()> {
        if self.buffer.is_empty() {
            return Ok(());
        }
        self.buffer.sort_unstable();
        self.buffer.dedup();
        let path = self
            .directory
            .join(format!(".{}-run-{:06}.bin", self.prefix, self.runs.len()));
        let mut writer = BufWriter::new(
            OpenOptions::new()
                .create_new(true)
                .write(true)
                .open(&path)?,
        );
        let mut bytes = vec![0u8; T::BYTES];
        for record in self.buffer.drain(..) {
            record.encode_into(&mut bytes);
            writer.write_all(&bytes)?;
        }
        writer.flush()?;
        writer.get_ref().sync_all()?;
        self.runs.push(path);
        Ok(())
    }

    pub fn finish(mut self, output: impl AsRef<Path>) -> Result<u64> {
        self.flush()?;
        if self.runs.is_empty() {
            let file = OpenOptions::new()
                .create_new(true)
                .write(true)
                .open(output.as_ref())?;
            file.sync_all()?;
            return Ok(0);
        }
        let mut merger = RecordMerger::<T>::open(&self.runs, false)?;
        let mut writer = BufWriter::new(
            OpenOptions::new()
                .create_new(true)
                .write(true)
                .open(output.as_ref())?,
        );
        let mut bytes = vec![0u8; T::BYTES];
        let mut previous = None;
        let mut records = 0u64;
        while let Some(record) = merger.next_record()? {
            if previous == Some(record) {
                continue;
            }
            record.encode_into(&mut bytes);
            writer.write_all(&bytes)?;
            previous = Some(record);
            records += 1;
        }
        writer.flush()?;
        writer.get_ref().sync_all()?;
        for path in self.runs {
            std::fs::remove_file(path)?;
        }
        Ok(records)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use sha2::Digest;

    #[test]
    fn external_sort_merges_and_deduplicates() {
        let root = std::env::temp_dir().join(format!(
            "ngn-k4-fixed-test-{}-{}",
            std::process::id(),
            crate::hex(sha2::Sha256::digest(b"external-sort-test"))
        ));
        std::fs::create_dir(&root).unwrap();
        let output = root.join("sorted.bin");
        let mut sorter = ExternalSorter::<ChainID>::new(&root, "ids", 3).unwrap();
        for value in [9, 1, 5, 1, 8, 2, 9, 0] {
            sorter.push(ChainID(value)).unwrap();
        }
        assert_eq!(sorter.finish(&output).unwrap(), 6);
        let mut reader = RecordReader::<ChainID>::open(&output).unwrap();
        let mut actual = Vec::new();
        while let Some(item) = reader.next_record().unwrap() {
            actual.push(item.0);
        }
        assert_eq!(actual, vec![0, 1, 2, 5, 8, 9]);
        std::fs::remove_dir_all(root).unwrap();
    }
}
