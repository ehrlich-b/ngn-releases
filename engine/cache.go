package engine

import (
	"sync"
	"unsafe"
)

type NodeType uint8

const (
	Exact      NodeType = 1
	UpperBound NodeType = 2
	LowerBound NodeType = 4
)

type TTMode uint8

const (
	TTDirect       TTMode = 0
	TTSynchronized TTMode = 1
)

type CachedEval struct {
	Key  uint64
	Data uint64
}

func (c *CachedEval) LoadKey() uint64 {
	return c.Key
}

func (c *CachedEval) LoadData() uint64 {
	return c.Data
}

func (c *CachedEval) Update(key uint64, data uint64) {
	c.Key = key
	c.Data = data
}

const (
	ttStripeCount             = 256
	OldAge             uint16 = 5
	DEFAULT_CACHE_SIZE        = 128
	MAX_CACHE_SIZE            = 120000

	MOVE_MASK  uint64 = (uint64(1) << 28) - 1
	EVAL_MASK  uint64 = (uint64(1) << 16) - 1
	DEPTH_MASK uint64 = (uint64(1) << 7) - 1
	TYPE_MASK  uint64 = (uint64(1) << 3) - 1
	AGE_MASK   uint64 = (uint64(1) << 9) - 1
	TTPV_MASK  uint64 = 1
)

var CACHE_ENTRY_SIZE int = int(unsafe.Sizeof(CachedEval{}))

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

func Pack(hashmove Move, eval int16, depth int8, nodeType NodeType, age uint16, ttPv bool) uint64 {
	var data uint64
	data |= uint64(hashmove) & MOVE_MASK
	data |= (uint64(uint16(eval)) & EVAL_MASK) << 28
	data |= (uint64(uint8(depth)) & DEPTH_MASK) << 44
	data |= (uint64(nodeType) & TYPE_MASK) << 51
	data |= (uint64(age) & AGE_MASK) << 54
	if ttPv {
		data |= TTPV_MASK << 63
	}
	return data
}

func Unpack(data uint64) (hashmove Move, eval int16, depth int8, nodeType NodeType, age uint16, ttPv bool) {
	hashmove = Move(data & MOVE_MASK)
	eval = int16(uint16((data >> 28) & EVAL_MASK))
	depth = int8((data >> 44) & DEPTH_MASK)
	nodeType = NodeType((data >> 51) & TYPE_MASK)
	age = uint16((data >> 54) & AGE_MASK)
	ttPv = ((data >> 63) & TTPV_MASK) != 0
	return
}

func RoundPowerOfTwo(size int) int {
	if size <= 0 {
		return 1
	}
	power := 1
	for power <= size/2 {
		power <<= 1
	}
	return power
}

func NewCache(megabytes int) *Cache {
	return newCacheWithMode(megabytes, TTDirect)
}

func newCacheWithMode(megabytes int, mode TTMode) *Cache {
	if megabytes <= 0 {
		return nil
	}
	records := RoundPowerOfTwo(megabytes * 1024 * 1024 / int(unsafe.Sizeof(CachedEval{})))
	return &Cache{
		items:  make([]CachedEval, records),
		size:   megabytes,
		length: uint64(records),
		mask:   uint(records - 1),
		mode:   mode,
	}
}

func (c *Cache) Size() int {
	return c.size
}

func (c *Cache) index(hash uint64) uint {
	return uint(hash) & c.mask
}

func (c *Cache) Clear() {
	for i := range c.items {
		c.items[i] = CachedEval{}
	}
	c.age = 0
}

func (c *Cache) AdvanceAge() {
	if c.mode == TTSynchronized {
		c.ageMu.Lock()
		c.age = (c.age + 1) & 1023
		c.ageMu.Unlock()
		return
	}
	c.age = (c.age + 1) & 1023
}

func (c *Cache) Consumed() int {
	limit := 1000
	if len(c.items) < limit {
		limit = len(c.items)
	}
	count := 0
	for i := 0; i < limit; i++ {
		if c.mode == TTSynchronized {
			stripe := &c.stripes[uint(i)%ttStripeCount]
			stripe.Lock()
			data := c.items[i].LoadData()
			stripe.Unlock()
			if data != 0 {
				count++
			}
			continue
		}
		if c.items[i].LoadData() != 0 {
			count++
		}
	}
	return count
}

func (c *Cache) Set(hash uint64, hashmove Move, eval int16, depth int8, nodeType NodeType, ttPv bool) {
	var currentAge uint16
	if c.mode == TTSynchronized {
		c.ageMu.Lock()
		currentAge = c.age
		c.ageMu.Unlock()
	} else {
		currentAge = c.age
	}

	index := c.index(hash)
	if c.mode == TTSynchronized {
		stripe := &c.stripes[index%ttStripeCount]
		stripe.Lock()
		c.setAt(index, hash, hashmove, eval, depth, nodeType, currentAge, ttPv)
		stripe.Unlock()
		return
	}
	c.setAt(index, hash, hashmove, eval, depth, nodeType, currentAge, ttPv)
}

func (c *Cache) setAt(index uint, hash uint64, hashmove Move, eval int16, depth int8, nodeType NodeType, currentAge uint16, ttPv bool) {
	oldKey := c.items[index].LoadKey()
	oldData := c.items[index].LoadData()
	replace := oldData == 0
	if !replace {
		oldHash := oldKey ^ oldData
		_, _, oldDepth, _, oldAge, _ := Unpack(oldData)
		if oldHash == hash {
			replace = int16(depth) >= int16(oldDepth)-3 || nodeType == Exact
		} else {
			replace = oldAge != currentAge || int16(depth) >= int16(oldDepth)
		}
	}
	if replace {
		data := Pack(hashmove, eval, depth, nodeType, currentAge, ttPv)
		c.items[index].Update(data^hash, data)
	}
}

func (c *Cache) Get(hash uint64) (Move, int16, int8, NodeType, bool, bool) {
	index := c.index(hash)
	var key, data uint64
	if c.mode == TTSynchronized {
		stripe := &c.stripes[index%ttStripeCount]
		stripe.Lock()
		key = c.items[index].LoadKey()
		data = c.items[index].LoadData()
		stripe.Unlock()
	} else {
		key = c.items[index].LoadKey()
		data = c.items[index].LoadData()
	}
	if key^data != hash {
		return 0, 0, 0, 0, false, false
	}
	hashmove, eval, depth, nodeType, _, ttPv := Unpack(data)
	return hashmove, eval, depth, nodeType, true, ttPv
}
