//go:build !ngnk8

package ngnk4

// The default build is the frozen four-bucket K4 contract.
const (
	InputBuckets = 4
	bucketLabel  = "k4"
)

var kingBucketTable = [64]int{
	1, 1, 0, 0, 0, 0, 1, 1,
	2, 2, 2, 2, 2, 2, 2, 2,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
}
