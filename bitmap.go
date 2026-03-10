/*
Copyright (C) 2024  Carl-Philip Hänsch

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
import "sync/atomic"

/*
 this is a size-flexible threadsafe bitmap. It grows on write.

 properties of this map:
  - non-blocking read
  - non-blocking write

*/
type NonBlockingBitMap struct {
	data atomic.Pointer[[]uint64]
}

func NewBitMap() (result NonBlockingBitMap) {
	return
}

func (b NonBlockingBitMap) ComputeSize() uint {
	dataptr := b.data.Load()
	var sz uint = 8 /* atomic pointer */ + 16 /* allocation of slice */ + 24 /* slice */
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

func (b *NonBlockingBitMap) Get(i uint) bool {
	ptr := b.data.Load()
	if ptr == nil {
		return false
	}
	data := *ptr
	if (i >> 6) >= uint(len(data)) {
		return false
	} else {
		return ((data[i >> 6] >> (i & 0b111111)) & 1) != 0
	}
}

// DataPtr returns the raw pointer to the underlying []uint64 slice.
// This is useful for JIT compilation where the pointer can be embedded
// as an immediate value. The returned pointer may be nil if no bits
// have been set yet. The slice data is safe to read concurrently.
func (b *NonBlockingBitMap) DataPtr() *[]uint64 {
	return b.data.Load()
}

func (b *NonBlockingBitMap) Set(i uint, val bool) {
	// first step: load array and ensure it is big enough
	var data []uint64
	for {
		dataptr := b.data.Load()
		if dataptr == nil {
			data = []uint64{}
		} else {
			data = *dataptr
		}
		if (i >> 6) >= uint(len(data)) {
			// first step: increase data size
			newdata := append(data, 0) // allocate new element
			if b.data.CompareAndSwap(dataptr, &newdata) {
				continue
			}
		} else {
			// finished: our data is now big enough
			break
		}
	}
	// second step: set & replace
	bit := uint64(1 << (uint64(i) & 0b111111))
	for {
		cell := data[i >> 6]
		var ncell uint64
		if val {
			ncell = cell | bit
		} else {
			ncell = cell & ^bit
		}
		if atomic.CompareAndSwapUint64(&data[i >> 6], cell, ncell) {
			break
		}
	}
}

func (b *NonBlockingBitMap) Size() uint {
	dataptr := b.data.Load()
	if dataptr == nil {
		return 48
	}
	return 8 * 8 + uint(len(*dataptr))
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
	currentCell := (*dataptr)[idx >> 6]
	for i := uint(0); i < (idx & 0b111111); i++ {
		if ((currentCell >> i) & 1) != 0 {
			result++
		}
	}
	return
}

// setOrWord atomically ORs mask into b's word at wordIdx, growing the backing slice if needed.
func (b *NonBlockingBitMap) setOrWord(wordIdx uint, mask uint64) {
	if mask == 0 {
		return
	}
	for {
		dataptr := b.data.Load()
		var data []uint64
		if dataptr != nil {
			data = *dataptr
		}
		if wordIdx >= uint(len(data)) {
			newdata := make([]uint64, wordIdx+1)
			copy(newdata, data)
			if !b.data.CompareAndSwap(dataptr, &newdata) {
				continue
			}
			data = newdata
		}
		for {
			old := atomic.LoadUint64(&data[wordIdx])
			if atomic.CompareAndSwapUint64(&data[wordIdx], old, old|mask) {
				return
			}
		}
	}
}

// setXorWord atomically XORs mask into b's word at wordIdx, growing the backing slice if needed.
func (b *NonBlockingBitMap) setXorWord(wordIdx uint, mask uint64) {
	if mask == 0 {
		return
	}
	for {
		dataptr := b.data.Load()
		var data []uint64
		if dataptr != nil {
			data = *dataptr
		}
		if wordIdx >= uint(len(data)) {
			newdata := make([]uint64, wordIdx+1)
			copy(newdata, data)
			if !b.data.CompareAndSwap(dataptr, &newdata) {
				continue
			}
			data = newdata
		}
		for {
			old := atomic.LoadUint64(&data[wordIdx])
			if atomic.CompareAndSwapUint64(&data[wordIdx], old, old^mask) {
				return
			}
		}
	}
}

// setAndNotWord atomically clears bits given by mask in b's word at wordIdx.
// If wordIdx is out of range the operation is a no-op (nothing to clear).
func (b *NonBlockingBitMap) setAndNotWord(wordIdx uint, mask uint64) {
	if mask == 0 {
		return
	}
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

// OrFrom sets bits in b at positions i+offset for each bit i set in other.
// Equivalent to: b |= (other << offset) in bitmap terms.
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
		b.setOrWord(uint(i)+wordOffset, v<<bitOffset)
		if bitOffset > 0 {
			b.setOrWord(uint(i)+wordOffset+1, v>>(64-bitOffset))
		}
	}
}

// XorFrom flips bits in b at positions i+offset for each bit i set in other.
// Equivalent to: b ^= (other << offset) in bitmap terms.
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
		b.setXorWord(uint(i)+wordOffset, v<<bitOffset)
		if bitOffset > 0 {
			b.setXorWord(uint(i)+wordOffset+1, v>>(64-bitOffset))
		}
	}
}

// AndNotFrom clears bits in b at positions i+offset for each bit i set in other.
// Equivalent to: b &= ^(other << offset) in bitmap terms.
// Bits in b beyond its current size are unaffected (already zero).
func (b *NonBlockingBitMap) AndNotFrom(other *NonBlockingBitMap, offset uint) {
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
		b.setAndNotWord(uint(i)+wordOffset, v<<bitOffset)
		if bitOffset > 0 {
			b.setAndNotWord(uint(i)+wordOffset+1, v>>(64-bitOffset))
		}
	}
}

