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

// TestSimple covers basic Get/Set/Count/Reset (plain single-writer variants).
func TestSimple(t *testing.T) {
	bm := NewBitMap()
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

// TestAtomicSet verifies that AtomicSet and AtomicGet work correctly
// (same semantics as Set/Get, just with CAS / atomic loads).
func TestAtomicSet(t *testing.T) {
	var bm NonBlockingBitMap
	bm.AtomicSet(5, true)
	bm.AtomicSet(127, true)
	if !bm.AtomicGet(5) || !bm.Get(5) {
		t.Fatalf("AtomicSet 5 not visible")
	}
	if !bm.AtomicGet(127) {
		t.Fatalf("AtomicSet 127 not visible")
	}
	bm.AtomicSet(5, false)
	if bm.AtomicGet(5) {
		t.Fatalf("AtomicSet clear 5 failed")
	}
	if !bm.AtomicGet(127) {
		t.Fatalf("AtomicSet 127 should remain set")
	}
}

// TODO: parallel concurrent test for Atomic* variants

// ---------------------------------------------------------------------------
// Bulk operations — plain variants
// ---------------------------------------------------------------------------

func TestOrFrom(t *testing.T) {
	// word-aligned offset
	var a, b NonBlockingBitMap
	b.Set(0, true)
	b.Set(3, true)
	b.Set(63, true)
	a.OrFrom(&b, 64) // shift b by 64 bits
	if !a.Get(64) || !a.Get(67) || !a.Get(127) {
		t.Fatalf("OrFrom aligned: expected bits at 64, 67, 127")
	}
	if a.Get(0) || a.Get(63) || a.Get(128) {
		t.Fatalf("OrFrom aligned: unexpected bits set")
	}

	// non-aligned offset: bit 0 → 4, bit 63 → 67
	var c, d NonBlockingBitMap
	d.Set(0, true)
	d.Set(63, true)
	c.OrFrom(&d, 4)
	if !c.Get(4) || !c.Get(67) {
		t.Fatalf("OrFrom non-aligned: expected bits at 4 and 67")
	}
	if c.Get(0) || c.Get(63) {
		t.Fatalf("OrFrom non-aligned: unexpected bits")
	}
}

func TestXorFrom(t *testing.T) {
	var a, b NonBlockingBitMap
	a.Set(68, true) // pre-set
	b.Set(4, true)  // offset=64 → flips bit 68 in a
	b.Set(5, true)  // offset=64 → flips bit 69 in a (was clear)
	a.XorFrom(&b, 64)
	if a.Get(68) {
		t.Fatalf("XorFrom: bit 68 should be cleared (was set, XOR flips it)")
	}
	if !a.Get(69) {
		t.Fatalf("XorFrom: bit 69 should be set (was clear, XOR flips it)")
	}
}

func TestAndNotFrom(t *testing.T) {
	var a, b NonBlockingBitMap
	a.Set(64, true)
	a.Set(65, true)
	a.Set(70, true)
	b.Set(0, true) // offset=64 → clears bit 64 in a
	b.Set(1, true) // offset=64 → clears bit 65 in a
	a.AndNotFrom(&b, 64)
	if a.Get(64) || a.Get(65) {
		t.Fatalf("AndNotFrom: bits 64 and 65 should be cleared")
	}
	if !a.Get(70) {
		t.Fatalf("AndNotFrom: bit 70 should remain set")
	}
}

// ---------------------------------------------------------------------------
// Bulk operations — Atomic* variants (same semantics, different impl)
// ---------------------------------------------------------------------------

func TestAtomicOrFrom(t *testing.T) {
	var a, b NonBlockingBitMap
	b.Set(0, true)
	b.Set(63, true)
	a.AtomicOrFrom(&b, 64)
	if !a.Get(64) || !a.Get(127) {
		t.Fatalf("AtomicOrFrom: expected bits at 64 and 127")
	}
	if a.Get(63) || a.Get(128) {
		t.Fatalf("AtomicOrFrom: unexpected bits")
	}
}

func TestAtomicXorFrom(t *testing.T) {
	var a, b NonBlockingBitMap
	a.Set(68, true)
	b.Set(4, true)
	b.Set(5, true)
	a.AtomicXorFrom(&b, 64)
	if a.Get(68) {
		t.Fatalf("AtomicXorFrom: bit 68 should be cleared")
	}
	if !a.Get(69) {
		t.Fatalf("AtomicXorFrom: bit 69 should be set")
	}
}

func TestAtomicAndNotFrom(t *testing.T) {
	var a, b NonBlockingBitMap
	a.Set(64, true)
	a.Set(65, true)
	a.Set(70, true)
	b.Set(0, true)
	b.Set(1, true)
	a.AtomicAndNotFrom(&b, 64)
	if a.Get(64) || a.Get(65) {
		t.Fatalf("AtomicAndNotFrom: bits 64 and 65 should be cleared")
	}
	if !a.Get(70) {
		t.Fatalf("AtomicAndNotFrom: bit 70 should remain set")
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestBitmapOpsNilOther(t *testing.T) {
	var a, empty NonBlockingBitMap
	a.Set(5, true)
	a.OrFrom(&empty, 0)
	a.XorFrom(&empty, 0)
	a.AndNotFrom(&empty, 0)
	a.AtomicOrFrom(&empty, 0)
	a.AtomicXorFrom(&empty, 0)
	a.AtomicAndNotFrom(&empty, 0)
	if !a.Get(5) {
		t.Fatalf("bit 5 should still be set after all no-op operations")
	}
}
