package engine

import (
	"bytes"
	"encoding/binary"
	"os"
	"sync"
	"testing"
)

func TestPrivateCacheCompatibilityDump(t *testing.T) {
	path := os.Getenv("NGN_PRIVATE_CACHE_DUMP")
	if path == "" {
		t.Skip("private baseline comparison only")
	}
	var output bytes.Buffer
	put := func(value interface{}) {
		if err := binary.Write(&output, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	nextState := uint64(0xbfc831e97d2465a3)
	next := func() uint64 {
		nextState ^= nextState << 13
		nextState ^= nextState >> 7
		nextState ^= nextState << 17
		return nextState
	}
	for i := 0; i < 100000; i++ {
		bits := next()
		move, score, depth, bound, age, pv := Unpack(bits)
		put(uint32(move))
		put(score)
		put(depth)
		put(uint8(bound))
		put(age)
		put(pv)
		put(Pack(move, score, depth, bound, age, pv))
		put(Pack(Move(next()), int16(next()), int8(next()), NodeType(next()), uint16(next()), next()&1 != 0))
	}
	for _, mode := range []TTMode{TTDirect, TTSynchronized} {
		c := newCacheWithMode(1, mode)
		put(int32(c.Size()))
		put(c.length)
		put(uint64(c.mask))
		put(uint8(c.mode))
		get := func(hash uint64) {
			move, score, depth, bound, hit, pv := c.Get(hash)
			put(uint32(move))
			put(score)
			put(depth)
			put(uint8(bound))
			put(hit)
			put(pv)
		}
		get(0)
		get(1)
		for i := 0; i < 100000; i++ {
			random := next()
			hash := (random & ^uint64(c.mask)) | uint64(i%1300)
			if i%17 == 0 {
				hash = 0
			}
			if i%4 == 0 {
				c.AdvanceAge()
			}
			if i%23003 == 23002 {
				c.Clear()
			}
			if i%19 == 0 {
				get(hash)
			}
			c.Set(hash, Move(next()), int16(next()), int8(next()), NodeType(next()), next()&1 != 0)
			get(hash)
			if i%31 == 0 {
				get(hash ^ (uint64(c.mask) + 1))
			}
			if i%97 == 0 {
				put(c.age)
				put(int32(c.Consumed()))
			}
		}
		for _, value := range c.items {
			put(value.LoadKey())
			put(value.LoadData())
		}
		c.Clear()
		put(c.age)
		put(int32(c.Consumed()))
		get(0)
		get(1)
	}
	for _, size := range []int{-20, -1, 0, 1, 2, 3, 7, 8, 9, 1023, 1024, 1025, 120000 * 1024 * 1024 / 16} {
		put(uint64(RoundPowerOfTwo(size)))
	}
	for _, megabytes := range []int{-1, 0, 1, 2, 3, 5} {
		c := NewCache(megabytes)
		put(c != nil)
		if c != nil {
			put(int32(c.Size()))
			put(c.length)
			put(uint64(c.mask))
		}
	}
	if err := os.WriteFile(path, output.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestIndependentCacheSynchronizedCoherence(t *testing.T) {
	c := newCacheWithMode(1, TTSynchronized)
	var workers sync.WaitGroup
	workers.Add(10)
	for worker := 0; worker < 8; worker++ {
		go func(worker int) {
			defer workers.Done()
			for i := 0; i < 1200; i++ {
				hash := uint64(worker+1)<<32 | uint64(i%64+1)
				move := Move(hash >> 2)
				score, depth, pv := int16(hash>>8), int8(hash%100), hash&1 != 0
				c.Set(hash, move, score, depth, Exact, pv)
				gotMove, gotScore, gotDepth, gotBound, hit, gotPV := c.Get(hash)
				if hit && (gotMove != move&Move(MOVE_MASK) || gotScore != score || gotDepth != depth || gotBound != Exact || gotPV != pv) {
					t.Errorf("incoherent cache record for %x", hash)
					return
				}
			}
		}(worker)
	}
	go func() {
		defer workers.Done()
		for i := 0; i < 2049; i++ {
			c.AdvanceAge()
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < 200; i++ {
			if got := c.Consumed(); got < 0 || got > 1000 {
				t.Errorf("occupancy %d", got)
			}
		}
	}()
	workers.Wait()
	if c.age != 1 {
		t.Fatalf("age wrap = %d, want 1", c.age)
	}
}
