//go:build !linux

package engine

func allocateTTRecords(records int) []ttRecord {
	return make([]ttRecord, records)
}
