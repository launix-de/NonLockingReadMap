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

import "fmt"
import "testing"


func TestSimple(t *testing.T) {
	bm := NewBitMap()
	if bm.Size() != 48 {
		t.Fatalf("empty map size")
	}
	if bm.Count() != 0 {
		t.Fatalf("count = 0 .1")
	}
	if bm.Get(0) {
		t.Fatalf("read 0")
	}
	if bm.Get(1000) {
		t.Fatalf("read 1000")
	}
	if bm.Count() != 0 {
		t.Fatalf("count = 0 .2")
	}

	bm.Set(5, true)
	bm.Set(6, true)
	bm.Set(77, true)
	bm.Set(1000, true)

	if bm.Count() != 4 {
		t.Fatalf("count = 4")
	}

	if bm.CountUntil(6) != 1 {
		t.Fatalf("countuntil 5: " + fmt.Sprint(bm.CountUntil(6)))
	}

	if bm.CountUntil(7) != 2 {
		t.Fatalf("countuntil 6: " + fmt.Sprint(bm.CountUntil(7)))
	}

	if bm.CountUntil(100) != 3 {
		t.Fatalf("countuntil 100: " + fmt.Sprint(bm.CountUntil(100)))
	}

	if bm.CountUntil(10000) != 4 {
		t.Fatalf("countuntil 10000: " + fmt.Sprint(bm.CountUntil(10000)))
	}

	if bm.Get(0) {
		t.Fatalf("read 0 .2")
	}
	if bm.Get(4) {
		t.Fatalf("read 4")
	}
	if !bm.Get(5) {
		t.Fatalf("read 5")
	}
	if !bm.Get(6) {
		t.Fatalf("read 6")
	}
	if bm.Get(7) {
		t.Fatalf("read 7")
	}
	if bm.Get(63) {
		t.Fatalf("read 63")
	}
	if bm.Get(64) {
		t.Fatalf("read 64")
	}
	if bm.Get(71) {
		t.Fatalf("read 71")
	}
	if !bm.Get(77) {
		t.Fatalf("read 77")
	}
	if !bm.Get(1000) {
		t.Fatalf("read 1000 .2")
	}
	if bm.Get(3000) {
		t.Fatalf("read over size")
	}

	bm.Set(6, false)

	if bm.Count() != 3 {
		t.Fatalf("count = 3")
	}

	if !bm.Get(5) {
		t.Fatalf("read 5 .2")
	}
	if bm.Get(6) {
		t.Fatalf("read 6 .2")
	}

	bm.Reset()
	if bm.Get(0) {
		t.Fatalf("read 0 .3")
	}
	if bm.Get(1000) {
		t.Fatalf("read 1000 .3")
	}

}

// TODO: parallel concurrent test


