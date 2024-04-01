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
	if (i >> 6) > uint(len(data)) {
		return false
	} else {
		return ((data[i >> 6] >> (i & 0b111111)) & 1) != 0
	}
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


