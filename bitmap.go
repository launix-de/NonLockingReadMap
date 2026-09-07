/*
Copyright (C) 2024-2026  Carl-Philip Hänsch

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package NonLockingReadMap

import "math/bits"
import "sync"
import "sync/atomic"

/*
NonBlockingBitMap is a size-flexible, lazily allocated bitmap.

Concurrency model
-----------------
There are two families of operations, distinguished by their name prefix:

	Atomic* operations (AtomicSet, AtomicGet, AtomicOrFrom, …)
	  Each word-level read-modify-write is an atomic CAS loop, so multiple
	  goroutines may call these concurrently without external locking.
	  Use these when the bitmap is shared between goroutines.

	Plain operations (Set, Get, OrFrom, …)
	  Perform direct (non-CAS) word reads/writes. They are NOT safe for
	  concurrent use — the caller must guarantee exclusive access (e.g.
	  under an external mutex). They are faster because they avoid the
	  CAS retry overhead.

Slice-pointer growth

	Atomic operations hold a shared resize lock while updating a word.
	The rare growth path takes the exclusive lock before copying and replacing
	the backing slice, so a completed word update can never be lost during the
	copy. Word updates themselves remain atomic CAS loops and operations on an
	already-sized bitmap do not serialize with each other.

Lazy allocation

	The backing slice is nil until the first write. A zero-value
	NonBlockingBitMap is ready to use and occupies only one pointer word.
*/
type NonBlockingBitMap struct {
	data   atomic.Pointer[[]uint64]
	resize sync.RWMutex
}

func NewBitMap() (result NonBlockingBitMap) {
	return
}

func (b *NonBlockingBitMap) ComputeSize() uint {
	dataptr := b.data.Load()
	var sz uint = 8 /* atomic pointer */ + 24 /* resize lock */ + 16 /* allocation of slice */ + 24 /* slice */
	if dataptr != nil {
		sz += 8 * uint(len(*dataptr)) /* slice storage */
	}
	return sz
}

func (b *NonBlockingBitMap) Reset() {
	dataptr := b.data.Load()
	for {
		if b.data.CompareAndSwap(dataptr, nil) {
			break
		}
	}
}

func (b *NonBlockingBitMap) Copy() (result NonBlockingBitMap) {
	dataptr := b.data.Load()
	if dataptr == nil {
		return
	}
	data2 := make([]uint64, len(*dataptr))
	copy(data2, *dataptr)
	result.data.Store(&data2)
	return
}

// ensureWord grows the backing slice to include wordIdx if necessary. Plain
// callers still require exclusive ownership after this function returns.
func (b *NonBlockingBitMap) ensureWord(wordIdx uint) []uint64 {
	b.resize.RLock()
	dataptr := b.data.Load()
	if dataptr != nil && wordIdx < uint(len(*dataptr)) {
		data := *dataptr
		b.resize.RUnlock()
		return data
	}
	b.resize.RUnlock()

	b.resize.Lock()
	defer b.resize.Unlock()
	dataptr = b.data.Load()
	var data []uint64
	if dataptr != nil {
		data = *dataptr
	}
	if wordIdx < uint(len(data)) {
		return data
	}
	newdata := make([]uint64, wordIdx+1)
	copy(newdata, data)
	b.data.Store(&newdata)
	return newdata
}

// ---------------------------------------------------------------------------
// Read operations
// ---------------------------------------------------------------------------

// Get returns the bit at position i using a plain (non-atomic) word read.
// Safe only when no concurrent writes are happening — e.g. while holding
// an external read lock that excludes all writers, or in exclusive single-
// goroutine use. If concurrent Atomic* writes are possible, use AtomicGet.
func (b *NonBlockingBitMap) Get(i uint) bool {
	ptr := b.data.Load()
	if ptr == nil {
		return false
	}
	data := *ptr
	if (i >> 6) >= uint(len(data)) {
		return false
	}
	return ((data[i>>6] >> (i & 63)) & 1) != 0
}

// AtomicGet returns the bit at position i using an atomic word load.
// Provides strict Go memory-model happens-before guarantees with AtomicSet.
func (b *NonBlockingBitMap) AtomicGet(i uint) bool {
	ptr := b.data.Load()
	if ptr == nil {
		return false
	}
	data := *ptr
	if (i >> 6) >= uint(len(data)) {
		return false
	}
	return (atomic.LoadUint64(&data[i>>6])>>(i&63))&1 != 0
}

// DataPtr returns the raw pointer to the underlying []uint64 slice.
// Useful for JIT compilation where the pointer can be embedded as an
// immediate value. The returned pointer may be nil if no bits have been
// set yet. The slice data is safe to read concurrently.
func (b *NonBlockingBitMap) DataPtr() *[]uint64 {
	return b.data.Load()
}

// ---------------------------------------------------------------------------
// Single-bit write — plain (single-writer, no CAS on the word)
// ---------------------------------------------------------------------------

// Set writes bit i to val.
// Requires exclusive access: must not be called concurrently with any
// other write on this bitmap. Use AtomicSet when concurrent writes are
// possible.
func (b *NonBlockingBitMap) Set(i uint, val bool) {
	data := b.ensureWord(i >> 6)
	bit := uint64(1) << (i & 63)
	if val {
		data[i>>6] |= bit
	} else {
		data[i>>6] &^= bit
	}
}

// ---------------------------------------------------------------------------
// Single-bit write — atomic (CAS loop, concurrent-safe)
// ---------------------------------------------------------------------------

// AtomicSet writes bit i to val using a CAS loop.
// The read-modify-write is atomic: concurrent AtomicSet calls on the same
// or different bits are safe without external locking.
func (b *NonBlockingBitMap) AtomicSet(i uint, val bool) {
	wordIdx := i >> 6
	b.lockExistingWord(wordIdx)
	defer b.resize.RUnlock()
	data := *b.data.Load()
	bit := uint64(1) << (i & 63)
	for {
		old := atomic.LoadUint64(&data[wordIdx])
		var ncell uint64
		if val {
			ncell = old | bit
		} else {
			ncell = old &^ bit
		}
		if atomic.CompareAndSwapUint64(&data[wordIdx], old, ncell) {
			return
		}
	}
}

// lockExistingWord returns with resize.RLock held and the requested word
// present. Holding the lock prevents a concurrent grow-copy from missing the
// caller's subsequent atomic word update.
func (b *NonBlockingBitMap) lockExistingWord(wordIdx uint) {
	for {
		b.resize.RLock()
		dataptr := b.data.Load()
		if dataptr != nil && wordIdx < uint(len(*dataptr)) {
			return
		}
		b.resize.RUnlock()
		b.ensureWord(wordIdx)
	}
}

// ---------------------------------------------------------------------------
// Metrics
// ---------------------------------------------------------------------------

func (b *NonBlockingBitMap) Size() uint {
	dataptr := b.data.Load()
	if dataptr == nil {
		return 48
	}
	return 8*8 + uint(len(*dataptr))
}

func (b *NonBlockingBitMap) Count() (result uint) {
	dataptr := b.data.Load()
	if dataptr == nil {
		return 0
	}
	for _, v := range *dataptr {
		result += uint(bits.OnesCount64(v))
	}
	return
}

// Iterate calls fn for each bit index that is set, in ascending order.
func (b *NonBlockingBitMap) Iterate(fn func(uint)) {
	dataptr := b.data.Load()
	if dataptr == nil {
		return
	}
	for wi, word := range *dataptr {
		for word != 0 {
			bit := uint(bits.TrailingZeros64(word))
			fn(uint(wi)*64 + bit)
			word &^= 1 << bit
		}
	}
}

func (b *NonBlockingBitMap) CountUntil(idx uint) (result uint) {
	dataptr := b.data.Load()
	if dataptr == nil {
		return 0
	}
	for i := uint(0); i < (idx >> 6); i++ {
		if i >= uint(len(*dataptr)) {
			return
		}
		result += uint(bits.OnesCount64((*dataptr)[i]))
	}
	if (idx >> 6) >= uint(len(*dataptr)) {
		return
	}
	currentCell := (*dataptr)[idx>>6]
	for i := uint(0); i < (idx & 63); i++ {
		if ((currentCell >> i) & 1) != 0 {
			result++
		}
	}
	return
}

// ---------------------------------------------------------------------------
// Bulk operations — plain (single-writer, direct word writes)
//
// Each function maps bits from other (at position i) into b (at position
// i+offset). The offset is in bits; non-word-aligned offsets are handled by
// splitting each source word across two destination words.
//
// Requires exclusive write access on b. Safe to use when b is private to
// the calling goroutine (e.g. under an external mutex).
// ---------------------------------------------------------------------------

// OrFrom sets bits in b at positions i+offset for each bit i set in other.
// Equivalent to: b |= (other << offset) in bitmap terms.
// Requires exclusive access to b.
func (b *NonBlockingBitMap) OrFrom(other *NonBlockingBitMap, offset uint) {
	otherPtr := other.data.Load()
	if otherPtr == nil {
		return
	}
	wordOffset := offset >> 6
	bitOffset := offset & 63
	for i, v := range *otherPtr {
		if v == 0 {
			continue
		}
		lo := v << bitOffset
		if lo != 0 {
			data := b.ensureWord(uint(i) + wordOffset)
			data[uint(i)+wordOffset] |= lo
		}
		if bitOffset > 0 {
			hi := v >> (64 - bitOffset)
			if hi != 0 {
				data := b.ensureWord(uint(i) + wordOffset + 1)
				data[uint(i)+wordOffset+1] |= hi
			}
		}
	}
}

// XorFrom flips bits in b at positions i+offset for each bit i set in other.
// Equivalent to: b ^= (other << offset) in bitmap terms.
// Requires exclusive access to b.
func (b *NonBlockingBitMap) XorFrom(other *NonBlockingBitMap, offset uint) {
	otherPtr := other.data.Load()
	if otherPtr == nil {
		return
	}
	wordOffset := offset >> 6
	bitOffset := offset & 63
	for i, v := range *otherPtr {
		if v == 0 {
			continue
		}
		lo := v << bitOffset
		if lo != 0 {
			data := b.ensureWord(uint(i) + wordOffset)
			data[uint(i)+wordOffset] ^= lo
		}
		if bitOffset > 0 {
			hi := v >> (64 - bitOffset)
			if hi != 0 {
				data := b.ensureWord(uint(i) + wordOffset + 1)
				data[uint(i)+wordOffset+1] ^= hi
			}
		}
	}
}

// AndNotFrom clears bits in b at positions i+offset for each bit i set in other.
// Equivalent to: b &= ^(other << offset) in bitmap terms.
// Bits beyond b's current size are unaffected (already zero).
// Requires exclusive access to b.
func (b *NonBlockingBitMap) AndNotFrom(other *NonBlockingBitMap, offset uint) {
	otherPtr := other.data.Load()
	if otherPtr == nil {
		return
	}
	bPtr := b.data.Load()
	if bPtr == nil {
		return
	}
	bData := *bPtr
	wordOffset := offset >> 6
	bitOffset := offset & 63
	for i, v := range *otherPtr {
		if v == 0 {
			continue
		}
		lo := v << bitOffset
		if lo != 0 {
			dstIdx := uint(i) + wordOffset
			if dstIdx < uint(len(bData)) {
				bData[dstIdx] &^= lo
			}
		}
		if bitOffset > 0 {
			hi := v >> (64 - bitOffset)
			if hi != 0 {
				dstIdx := uint(i) + wordOffset + 1
				if dstIdx < uint(len(bData)) {
					bData[dstIdx] &^= hi
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Bulk operations — atomic (CAS loops, concurrent-safe)
//
// Same semantics as the plain variants above, but each word update uses a
// CAS loop so multiple goroutines may call these concurrently.
// ---------------------------------------------------------------------------

func (b *NonBlockingBitMap) atomicOrWord(wordIdx uint, mask uint64) {
	if mask == 0 {
		return
	}
	b.lockExistingWord(wordIdx)
	defer b.resize.RUnlock()
	data := *b.data.Load()
	for {
		old := atomic.LoadUint64(&data[wordIdx])
		if atomic.CompareAndSwapUint64(&data[wordIdx], old, old|mask) {
			return
		}
	}
}

func (b *NonBlockingBitMap) atomicXorWord(wordIdx uint, mask uint64) {
	if mask == 0 {
		return
	}
	b.lockExistingWord(wordIdx)
	defer b.resize.RUnlock()
	data := *b.data.Load()
	for {
		old := atomic.LoadUint64(&data[wordIdx])
		if atomic.CompareAndSwapUint64(&data[wordIdx], old, old^mask) {
			return
		}
	}
}

func (b *NonBlockingBitMap) atomicAndNotWord(wordIdx uint, mask uint64) {
	if mask == 0 {
		return
	}
	b.resize.RLock()
	defer b.resize.RUnlock()
	dataptr := b.data.Load()
	if dataptr == nil {
		return
	}
	data := *dataptr
	if wordIdx >= uint(len(data)) {
		return
	}
	for {
		old := atomic.LoadUint64(&data[wordIdx])
		if atomic.CompareAndSwapUint64(&data[wordIdx], old, old&^mask) {
			return
		}
	}
}

// AtomicOrFrom sets bits in b at positions i+offset for each bit i set in other.
// Equivalent to: b |= (other << offset) in bitmap terms.
// Concurrent-safe: each word update is a CAS loop.
func (b *NonBlockingBitMap) AtomicOrFrom(other *NonBlockingBitMap, offset uint) {
	otherPtr := other.data.Load()
	if otherPtr == nil {
		return
	}
	wordOffset := offset >> 6
	bitOffset := offset & 63
	for i, v := range *otherPtr {
		if v == 0 {
			continue
		}
		b.atomicOrWord(uint(i)+wordOffset, v<<bitOffset)
		if bitOffset > 0 {
			b.atomicOrWord(uint(i)+wordOffset+1, v>>(64-bitOffset))
		}
	}
}

// AtomicXorFrom flips bits in b at positions i+offset for each bit i set in other.
// Equivalent to: b ^= (other << offset) in bitmap terms.
// Concurrent-safe: each word update is a CAS loop.
func (b *NonBlockingBitMap) AtomicXorFrom(other *NonBlockingBitMap, offset uint) {
	otherPtr := other.data.Load()
	if otherPtr == nil {
		return
	}
	wordOffset := offset >> 6
	bitOffset := offset & 63
	for i, v := range *otherPtr {
		if v == 0 {
			continue
		}
		b.atomicXorWord(uint(i)+wordOffset, v<<bitOffset)
		if bitOffset > 0 {
			b.atomicXorWord(uint(i)+wordOffset+1, v>>(64-bitOffset))
		}
	}
}

// AtomicAndNotFrom clears bits in b at positions i+offset for each bit i set in other.
// Equivalent to: b &= ^(other << offset) in bitmap terms.
// Concurrent-safe: each word update is a CAS loop.
func (b *NonBlockingBitMap) AtomicAndNotFrom(other *NonBlockingBitMap, offset uint) {
	otherPtr := other.data.Load()
	if otherPtr == nil {
		return
	}
	wordOffset := offset >> 6
	bitOffset := offset & 63
	for i, v := range *otherPtr {
		if v == 0 {
			continue
		}
		b.atomicAndNotWord(uint(i)+wordOffset, v<<bitOffset)
		if bitOffset > 0 {
			b.atomicAndNotWord(uint(i)+wordOffset+1, v>>(64-bitOffset))
		}
	}
}
