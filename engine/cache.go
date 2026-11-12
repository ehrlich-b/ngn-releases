package engine

import (
	"math/bits"
	"sync"
)

type NodeType uint8

const (
	Exact      NodeType = 1
	UpperBound NodeType = 2
	LowerBound NodeType = 3
)

type TTMode uint8

const (
	TTDirect TTMode = iota
	TTSynchronized
)

const (
	DEFAULT_CACHE_SIZE        = 128
	ttWays                    = 4
	ttRecordBytes             = 16
	ttStripeCount             = 256
	ttMoveMask         uint64 = 1<<27 - 1
	ttOccupied         uint64 = 1 << 61
)

// A cluster contains four full-key records in 64 bytes. The engine's valid
// moves use 27 bits. Full keys avoid introducing truncated-key false hits.
// Payload: score[0:16], depth[16:23], generation[23:31], PV[31],
// move[32:59], bound[59:61], occupied[61]. The last two bits are unused.
// This layout and retention rule were designed for NGN, without source code
// from another engine. Clusters and depth/recency preference are public ideas.
type ttRecord struct{ hash, payload uint64 }

type Cache struct {
	items   []ttRecord
	size    int
	length  uint64
	mask    uint
	age     uint16
	mode    TTMode
	ageMu   sync.Mutex
	stripes [ttStripeCount]sync.Mutex
}

func Pack(move Move, score int16, depth int8, bound NodeType, age uint16, pv bool) uint64 {
	if depth < 0 {
		depth = 0
	}
	data := uint64(uint16(score)) | uint64(depth)<<16 | uint64(age&255)<<23 |
		(uint64(move)&ttMoveMask)<<32 | uint64(bound&3)<<59 | ttOccupied
	if pv {
		data |= 1 << 31
	}
	return data
}

func Unpack(data uint64) (Move, int16, int8, NodeType, uint16, bool) {
	return Move(data >> 32 & ttMoveMask), int16(data), int8(data >> 16 & 127),
		NodeType(data >> 59 & 3), uint16(data >> 23 & 255), data&(1<<31) != 0
}

func RoundPowerOfTwo(n int) int {
	if n <= 1 {
		return 1
	}
	return 1 << (bits.Len(uint(n)) - 1)
}

func NewCache(megabytes int) *Cache { return newCacheWithMode(megabytes, TTDirect) }
func newCacheWithMode(megabytes int, mode TTMode) *Cache {
	if megabytes <= 0 {
		return nil
	}
	records := RoundPowerOfTwo(megabytes * 1024 * 1024 / ttRecordBytes)
	return &Cache{items: allocateTTRecords(records), size: megabytes, length: uint64(records), mask: uint(records/ttWays - 1), mode: mode}
}
func (c *Cache) Size() int              { return c.size }
func (c *Cache) index(hash uint64) uint { return (uint(hash) & c.mask) * ttWays }

// Clear is called only with the receiver's search session quiescent.
func (c *Cache) Clear() { clear(c.items); c.age = 0 }
func (c *Cache) AdvanceAge() {
	if c.mode == TTSynchronized {
		c.ageMu.Lock()
		defer c.ageMu.Unlock()
	}
	c.age = (c.age + 1) & 255
}
func (c *Cache) generation() uint16 {
	if c.mode == TTSynchronized {
		c.ageMu.Lock()
		defer c.ageMu.Unlock()
	}
	return c.age
}

// Retention is measured in quarter-plies. Each elapsed search ages a record
// by two plies; an exact bound and PV add half/quarter ply respectively.
// The capped modular distance makes wraparound deterministic.
func ttRetention(data uint64, current uint16) int {
	_, _, depth, bound, age, pv := Unpack(data)
	distance := int((current - age) & 255)
	if distance > 31 {
		distance = 31
	}
	value := 4*int(depth) - 8*distance
	if bound == Exact {
		value += 2
	}
	if pv {
		value++
	}
	return value
}

func (c *Cache) Set(hash uint64, move Move, score int16, depth int8, bound NodeType, pv bool) {
	current := c.generation()
	first := c.index(hash)
	if c.mode == TTSynchronized {
		lock := &c.stripes[(first/ttWays)%ttStripeCount]
		lock.Lock()
		defer lock.Unlock()
	}
	incoming := Pack(move, score, depth, bound, current, pv)
	victim := first
	weakest := int(^uint(0) >> 1)
	for i := first; i < first+ttWays; i++ {
		old := c.items[i]
		if old.payload&ttOccupied == 0 {
			victim = i
			weakest = -1 << 30
			continue
		}
		quality := ttRetention(old.payload, current)
		if old.hash == hash {
			// A matching position gets a two-ply refresh window: its latest
			// completed search can change the useful bound and move without
			// needing to exceed the retained search depth. Collision victims
			// still use the strict retention comparison below.
			if ttRetention(incoming, current)+8 >= quality {
				if move == EmptyMove {
					incoming |= old.payload & (ttMoveMask << 32)
				}
				c.items[i] = ttRecord{hash, incoming}
			} else {
				// A rejected shallow update still proves the entry is current.
				c.items[i].payload = (old.payload &^ (uint64(255) << 23)) | uint64(current)<<23
			}
			return
		}
		if quality < weakest {
			victim = i
			weakest = quality
		}
	}
	if ttRetention(incoming, current) >= weakest {
		c.items[victim] = ttRecord{hash, incoming}
	}
}

func (c *Cache) Get(hash uint64) (Move, int16, int8, NodeType, bool, bool) {
	first := c.index(hash)
	if c.mode == TTSynchronized {
		lock := &c.stripes[(first/ttWays)%ttStripeCount]
		lock.Lock()
		defer lock.Unlock()
	}
	for i := first; i < first+ttWays; i++ {
		record := c.items[i]
		if record.payload&ttOccupied != 0 && record.hash == hash {
			move, score, depth, bound, _, pv := Unpack(record.payload)
			return move, score, depth, bound, true, pv
		}
	}
	return EmptyMove, 0, 0, 0, false, false
}

// UCI hashfull samples 256 evenly spaced clusters across the allocation,
// reporting the per-mille fraction carrying this search's generation.
func (c *Cache) Consumed() int {
	current := c.generation()
	clusters := len(c.items) / ttWays
	samples := min(256, clusters)
	stride := clusters / samples
	occupied := 0
	for bucket := 0; bucket < clusters; bucket += stride {
		first := bucket * ttWays
		if c.mode == TTSynchronized {
			c.stripes[bucket%ttStripeCount].Lock()
		}
		for i := first; i < first+ttWays; i++ {
			data := c.items[i].payload
			if data&ttOccupied != 0 && uint16(data>>23&255) == current {
				occupied++
			}
		}
		if c.mode == TTSynchronized {
			c.stripes[bucket%ttStripeCount].Unlock()
		}
	}
	return occupied * 1000 / (samples * ttWays)
}
