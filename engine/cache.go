/*
https://github.com/amanjpro/zahak/?tab=MIT-1-ov-file#readme
MIT License

Copyright (c) 2021 Amanj Sherwany

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package engine

import (
	"sync"
	"unsafe"
	// 	"fmt"
)

type CachedEval struct {
	Key  uint64 // 8
	Data uint64 // 8
	// Plain uint64 fields preserve the 16-byte record and historical direct-mode
	// cost. TTSynchronized protects the whole key/data record with one stripe;
	// callers must never read or update its fields separately while shared.
	// The XOR relation (Key=Data^hash) remains the collision checksum.
}

type NodeType uint8

const (
	Exact      NodeType = 1 << iota // PV-Node
	UpperBound                      // All-Node
	LowerBound                      // Cut-Node
)

type TTMode uint8

const (
	TTDirect TTMode = iota
	TTSynchronized
)

const ttStripeCount = 256

// Cache must not be copied after first use because synchronized mode owns mutexes.
type Cache struct {
	items   []CachedEval
	size    int
	length  uint64
	mask    uint
	age     uint16
	mode    TTMode
	ageMu   sync.Mutex
	stripes [ttStripeCount]sync.Mutex
}

const OldAge = uint16(5)
const DEFAULT_CACHE_SIZE = 128
const MAX_CACHE_SIZE = 120_000

var CACHE_ENTRY_SIZE = int(unsafe.Sizeof(CachedEval{}))

func (c *CachedEval) LoadKey() uint64 {
	return c.Key
}

func (c *CachedEval) LoadData() uint64 {
	return c.Data
}

const MOVE_MASK uint64 = 0b1111111111111111111111111111 // move << 0, 28 bits
const EVAL_MASK uint64 = 0b1111111111111111             // eval << 28, 16 bits
const DEPTH_MASK uint64 = 0b1111111                     // depth << 44, 7 bits
const TYPE_MASK uint64 = 0b111                          // type << 51, 3 bits
const AGE_MASK uint64 = 0b111111111                     // age << 54, 9 bits (T19: 1 bit ceded to ttPv)
const TTPV_MASK uint64 = 0b1                            // ttPv << 63, 1 bit

func Pack(hashmove Move, eval int16, depth int8, nodeType NodeType, age uint16, ttPv bool) uint64 {
	var pv uint64
	if ttPv {
		pv = 1
	}
	return (uint64(hashmove) & MOVE_MASK) |
		((uint64(eval) & EVAL_MASK) << 28) |
		((uint64(depth) & DEPTH_MASK) << 44) |
		((uint64(nodeType) & TYPE_MASK) << 51) |
		((uint64(age) & AGE_MASK) << 54) |
		((pv & TTPV_MASK) << 63)
}

func Unpack(data uint64) (hashmove Move, eval int16, depth int8, nodeType NodeType, age uint16, ttPv bool) {
	hashmove = Move(data & MOVE_MASK)
	eval = int16((data >> 28) & EVAL_MASK)
	depth = int8((data >> 44) & DEPTH_MASK)
	nodeType = NodeType((data >> 51) & TYPE_MASK)
	age = uint16((data >> 54) & AGE_MASK)
	ttPv = ((data >> 63) & TTPV_MASK) != 0
	return
}

func (c *CachedEval) Update(key uint64, data uint64) {
	c.Key = key
	c.Data = data
}

func (c *Cache) AdvanceAge() {
	if c.mode == TTSynchronized {
		c.ageMu.Lock()
		defer c.ageMu.Unlock()
	}
	c.age += 1
	// Deliberately preserve the historical 10-bit wrap even though Pack stores
	// only nine age bits. That mismatch is a separate behavior change.
	if c.age > uint16(0b1111111111) {
		c.age = 0
	}
}

func (c *Cache) Consumed() int {
	used := 0
	samples := 1000

	for i := 0; i < samples; i++ {
		if c.mode == TTSynchronized {
			stripe := &c.stripes[uint(i)%ttStripeCount]
			stripe.Lock()
			data := c.items[i].LoadData()
			stripe.Unlock()
			if data != 0 {
				used++
			}
		} else if c.items[i].LoadData() != 0 {
			used++
		}
	}

	return used / (samples / 1000)
}

func (c *Cache) index(hash uint64) uint {
	return uint(hash) & c.mask
}

func (c *Cache) Set(hash uint64, hashmove Move, eval int16, depth int8, nodeType NodeType, ttPv bool) {
	index := c.index(hash)
	if c.mode == TTSynchronized {
		c.ageMu.Lock()
		age := c.age
		c.ageMu.Unlock()
		stripe := &c.stripes[index%ttStripeCount]
		stripe.Lock()
		defer stripe.Unlock()
		c.setAt(index, age, hash, hashmove, eval, depth, nodeType, ttPv)
		return
	}
	c.setAt(index, c.age, hash, hashmove, eval, depth, nodeType, ttPv)
}

func (c *Cache) setAt(index uint, age uint16, hash uint64, hashmove Move, eval int16, depth int8, nodeType NodeType, ttPv bool) {
	oldValue := &c.items[index]
	oldKey := oldValue.LoadKey()
	oldData := oldValue.LoadData()
	oldHash := oldKey ^ oldData

	// very good for debugging hash issues
	// newHashmove, newEval, newDepth, newNodeType, newAge := Unpack(newData)
	// if hashmove != newHashmove || eval != newEval || depth != newDepth || nodeType != newNodeType || age != newAge {
	// 	panic(fmt.Sprintf(
	// 		"Culprits are: %d %d %d %d %d\nSomehow became: %d %d %d %d %d\n", hashmove, eval, depth, nodeType, age, newHashmove, newEval, newDepth, newNodeType, newAge))
	// }

	_, _, oldDepth, _, oldAge, _ := Unpack(oldData)
	var replace bool
	if oldData == 0 {
		replace = true
	} else if oldHash == hash {
		replace = depth >= oldDepth-3 || nodeType == Exact
	} else {
		replace = oldAge != age || depth >= oldDepth
	}
	if replace {

		newData := Pack(hashmove, eval, depth, nodeType, age, ttPv)
		newKey := newData ^ hash
		c.items[index].Update(newKey, newData)
	}
}

func (c *Cache) Size() int {
	return c.size
}

// Clear invalidates every entry while preserving the configured allocation.
// Callers must hold exclusive idle ownership in both direct and synchronized modes.
func (c *Cache) Clear() {
	clear(c.items)
	c.age = 0
}

func (c *Cache) Get(hash uint64) (Move, int16, int8, NodeType, bool, bool) {
	index := c.index(hash)
	if c.mode == TTSynchronized {
		stripe := &c.stripes[index%ttStripeCount]
		stripe.Lock()
		defer stripe.Unlock()
	}
	value := &c.items[index]
	data := value.LoadData()
	key := value.LoadKey()
	ok := hash == (key ^ data)
	if ok {
		move, eval, depth, nType, _, ttPv := Unpack(data)
		return move, eval, depth, nType, true, ttPv
	}
	return 0, 0, 0, 0, false, false
}

func NewCache(megabytes int) *Cache {
	return newCacheWithMode(megabytes, TTDirect)
}

func newCacheWithMode(megabytes int, mode TTMode) *Cache {
	if megabytes < 1 {
		return nil
	}
	size := int((megabytes * 1024 * 1024) / CACHE_ENTRY_SIZE)
	length := RoundPowerOfTwo(size)
	return &Cache{
		items: make([]CachedEval, length), size: megabytes, length: uint64(length),
		mask: uint(length - 1), mode: mode,
	}
}

func RoundPowerOfTwo(size int) int {
	var x = 1
	for (x << 1) <= size {
		x <<= 1
	}
	return x
}
