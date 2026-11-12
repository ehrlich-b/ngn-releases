//go:build linux

package engine

import (
	"syscall"
	"unsafe"
)

const ttHugePageBytes = 2 * 1024 * 1024

// Linux MADV_COLLAPSE (since 6.1) promotes already populated heap pages now,
// rather than waiting for khugepaged. Older kernels may reject this hint.
const ttMadvCollapse = 25

// allocateTTRecords keeps the table on the Go heap, so resizing and releasing a
// cache retain their ordinary GC lifetime. An aligned interior slice keeps the
// complete pointer-free backing allocation live; no mmap finalizer is needed.
func allocateTTRecords(records int) []ttRecord {
	bytes := records * ttRecordBytes
	if bytes < ttHugePageBytes {
		return make([]ttRecord, records)
	}
	storage := make([]byte, bytes+ttHugePageBytes-1)
	address := uintptr(unsafe.Pointer(&storage[0]))
	offset := int(-address & (ttHugePageBytes - 1))
	region := storage[offset : offset+bytes : offset+bytes]
	// Huge pages are a best-effort hint. Failure leaves the same zeroed table
	// and never changes capacity, indexing, replacement, or search behavior.
	_ = syscall.Madvise(region, syscall.MADV_HUGEPAGE)
	// Fresh Go heap spans can still be entirely unpopulated. Collapse requires
	// a present page in each huge-page range; touching one byte also lets the
	// ordinary first-write fault allocate a huge page directly.
	for index := 0; index < len(region); index += ttHugePageBytes {
		region[index] = 0
	}
	_ = syscall.Madvise(region, ttMadvCollapse)
	return unsafe.Slice((*ttRecord)(unsafe.Pointer(&region[0])), records)
}
