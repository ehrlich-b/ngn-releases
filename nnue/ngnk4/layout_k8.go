//go:build ngnk8

package ngnk4

// The ngnk8 build tag selects the eight-bucket research layout. It matches
// the trainer's K8_BUCKET_LAYOUT with files e-h mirrored onto d-a.
const (
	InputBuckets = 8
	bucketLabel  = "k8"
)

var kingBucketTable = [64]int{
	0, 1, 2, 3, 3, 2, 1, 0,
	4, 4, 5, 5, 5, 5, 4, 4,
	6, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6, 6,
	7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7,
}
