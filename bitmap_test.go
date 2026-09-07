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

import (
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Basic single-threaded correctness (plain Set/Get)
// ---------------------------------------------------------------------------

// TestSimple covers basic Get/Set/Count/Reset using the plain (single-writer) variants.
func TestSimple(t *testing.T) {
	bm := NewBitMap()
	if bm.Count() != 0 {
		t.Fatal("initial count should be 0")
	}
	if bm.Get(0) || bm.Get(1000) {
		t.Fatal("fresh bitmap must return false for all bits")
	}

	bm.Set(5, true)
	bm.Set(6, true)
	bm.Set(77, true)
	bm.Set(1000, true)

	if bm.Count() != 4 {
		t.Fatalf("expected count 4, got %d", bm.Count())
	}
	if bm.CountUntil(6) != 1 {
		t.Fatalf("CountUntil(6) = %d, want 1", bm.CountUntil(6))
	}
	if bm.CountUntil(7) != 2 {
		t.Fatalf("CountUntil(7) = %d, want 2", bm.CountUntil(7))
	}
	if bm.CountUntil(100) != 3 {
		t.Fatalf("CountUntil(100) = %d, want 3", bm.CountUntil(100))
	}
	if bm.CountUntil(10000) != 4 {
		t.Fatalf("CountUntil(10000) = %d, want 4", bm.CountUntil(10000))
	}

	for _, i := range []uint{0, 4, 7, 63, 64, 71} {
		if bm.Get(i) {
			t.Fatalf("bit %d should be clear", i)
		}
	}
	for _, i := range []uint{5, 6, 77, 1000} {
		if !bm.Get(i) {
			t.Fatalf("bit %d should be set", i)
		}
	}

	bm.Set(6, false)
	if bm.Count() != 3 {
		t.Fatalf("expected count 3 after clear, got %d", bm.Count())
	}
	if bm.Get(6) {
		t.Fatal("bit 6 should be clear after Set(6, false)")
	}
	if !bm.Get(5) {
		t.Fatal("bit 5 should still be set")
	}

	bm.Reset()
	if bm.Get(0) || bm.Get(1000) {
		t.Fatal("all bits should be clear after Reset")
	}
}

// TestAtomicSetGet verifies AtomicSet / AtomicGet correctness.
func TestAtomicSetGet(t *testing.T) {
	var bm NonBlockingBitMap
	bm.AtomicSet(5, true)
	bm.AtomicSet(127, true)

	if !bm.AtomicGet(5) {
		t.Fatal("AtomicGet(5) should be true")
	}
	if !bm.Get(5) {
		t.Fatal("Get(5) should see AtomicSet write")
	}
	if !bm.AtomicGet(127) {
		t.Fatal("AtomicGet(127) should be true")
	}
	if bm.AtomicGet(6) {
		t.Fatal("AtomicGet(6) should be false")
	}

	bm.AtomicSet(5, false)
	if bm.AtomicGet(5) {
		t.Fatal("AtomicGet(5) should be false after AtomicSet(5, false)")
	}
	if !bm.AtomicGet(127) {
		t.Fatal("AtomicGet(127) should still be true")
	}
}

// ---------------------------------------------------------------------------
// Bulk operations — plain variants (single-writer, word-aligned and non-aligned)
// ---------------------------------------------------------------------------

func TestOrFrom(t *testing.T) {
	// word-aligned offset (offset divisible by 64)
	var a, b NonBlockingBitMap
	b.Set(0, true)
	b.Set(3, true)
	b.Set(63, true)
	a.OrFrom(&b, 64) // bit 0→64, bit 3→67, bit 63→127
	for _, tc := range []struct {
		i    uint
		want bool
	}{
		{64, true}, {67, true}, {127, true},
		{0, false}, {63, false}, {128, false},
	} {
		if a.Get(tc.i) != tc.want {
			t.Fatalf("OrFrom aligned: Get(%d) = %v, want %v", tc.i, a.Get(tc.i), tc.want)
		}
	}

	// non-aligned offset: bit 0→4, bit 63→67 (crosses word boundary)
	var c, d NonBlockingBitMap
	d.Set(0, true)
	d.Set(63, true)
	c.OrFrom(&d, 4)
	if !c.Get(4) || !c.Get(67) {
		t.Fatal("OrFrom non-aligned: expected bits at 4 and 67")
	}
	if c.Get(0) || c.Get(63) {
		t.Fatal("OrFrom non-aligned: unexpected bits set")
	}
}

func TestXorFrom(t *testing.T) {
	var a, b NonBlockingBitMap
	a.Set(68, true) // pre-set
	b.Set(4, true)  // offset=64 → flips bit 68 (was set  → cleared)
	b.Set(5, true)  // offset=64 → flips bit 69 (was clear → set)
	a.XorFrom(&b, 64)
	if a.Get(68) {
		t.Fatal("XorFrom: bit 68 should be cleared")
	}
	if !a.Get(69) {
		t.Fatal("XorFrom: bit 69 should be set")
	}
}

func TestAndNotFrom(t *testing.T) {
	var a, b NonBlockingBitMap
	a.Set(64, true)
	a.Set(65, true)
	a.Set(70, true)
	b.Set(0, true) // offset=64 → clears bit 64
	b.Set(1, true) // offset=64 → clears bit 65
	a.AndNotFrom(&b, 64)
	if a.Get(64) || a.Get(65) {
		t.Fatal("AndNotFrom: bits 64 and 65 should be cleared")
	}
	if !a.Get(70) {
		t.Fatal("AndNotFrom: bit 70 should remain set")
	}
}

// ---------------------------------------------------------------------------
// Bulk operations — Atomic* variants (same semantics as plain, different impl)
// ---------------------------------------------------------------------------

func TestAtomicOrFrom(t *testing.T) {
	var a, b NonBlockingBitMap
	b.Set(0, true)
	b.Set(63, true)
	a.AtomicOrFrom(&b, 64)
	if !a.Get(64) || !a.Get(127) {
		t.Fatal("AtomicOrFrom: expected bits at 64 and 127")
	}
	if a.Get(63) || a.Get(128) {
		t.Fatal("AtomicOrFrom: unexpected bits")
	}

	// non-aligned
	var c, d NonBlockingBitMap
	d.Set(0, true)
	d.Set(63, true)
	c.AtomicOrFrom(&d, 4)
	if !c.Get(4) || !c.Get(67) {
		t.Fatal("AtomicOrFrom non-aligned: expected bits at 4 and 67")
	}
}

func TestAtomicXorFrom(t *testing.T) {
	var a, b NonBlockingBitMap
	a.AtomicSet(68, true)
	b.Set(4, true)
	b.Set(5, true)
	a.AtomicXorFrom(&b, 64)
	if a.Get(68) {
		t.Fatal("AtomicXorFrom: bit 68 should be cleared")
	}
	if !a.Get(69) {
		t.Fatal("AtomicXorFrom: bit 69 should be set")
	}
}

func TestAtomicAndNotFrom(t *testing.T) {
	var a, b NonBlockingBitMap
	a.AtomicSet(64, true)
	a.AtomicSet(65, true)
	a.AtomicSet(70, true)
	b.Set(0, true)
	b.Set(1, true)
	a.AtomicAndNotFrom(&b, 64)
	if a.Get(64) || a.Get(65) {
		t.Fatal("AtomicAndNotFrom: bits 64 and 65 should be cleared")
	}
	if !a.Get(70) {
		t.Fatal("AtomicAndNotFrom: bit 70 should remain set")
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
		t.Fatal("bit 5 must remain set after all no-op bulk operations")
	}
}

func TestAndNotFromEmptyTarget(t *testing.T) {
	// AndNotFrom on a nil/empty target must not panic.
	var a, b NonBlockingBitMap
	b.Set(10, true)
	a.AndNotFrom(&b, 0)       // a is empty — no-op
	a.AtomicAndNotFrom(&b, 0) // same
}

func TestLargeOffset(t *testing.T) {
	// Offset that jumps many words.
	var a, b NonBlockingBitMap
	b.Set(0, true)
	b.Set(1, true)
	a.AtomicOrFrom(&b, 10000)
	if !a.Get(10000) || !a.Get(10001) {
		t.Fatal("AtomicOrFrom with large offset: expected bits at 10000 and 10001")
	}
	if a.Get(9999) || a.Get(10002) {
		t.Fatal("AtomicOrFrom with large offset: unexpected bits")
	}
}

// ---------------------------------------------------------------------------
// Parallel / race-detector tests
//
// Run with: go test -race ./...
//
// Each test below fires N goroutines that call Atomic* operations concurrently.
// Correctness is verified after all goroutines have finished.
// ---------------------------------------------------------------------------

const parallelWorkers = 64

// TestAtomicSetParallel: N goroutines each set their own unique bit.
// After all complete every bit must be set (tests concurrent growth + CAS).
func TestAtomicSetParallel(t *testing.T) {
	var bm NonBlockingBitMap
	var wg sync.WaitGroup
	for i := 0; i < parallelWorkers; i++ {
		wg.Add(1)
		go func(bit uint) {
			defer wg.Done()
			bm.AtomicSet(bit*7, true) // spread across multiple words
		}(uint(i))
	}
	wg.Wait()
	for i := 0; i < parallelWorkers; i++ {
		if !bm.Get(uint(i) * 7) {
			t.Fatalf("bit %d should be set after parallel AtomicSet", i*7)
		}
	}
}

// TestAtomicSetSameBitParallel: N goroutines set and clear the same bit
// simultaneously. No data race must occur; the final state must be consistent.
func TestAtomicSetSameBitParallel(t *testing.T) {
	var bm NonBlockingBitMap
	var wg sync.WaitGroup
	// Half set, half clear — races on the CAS are expected and handled.
	for i := 0; i < parallelWorkers; i++ {
		wg.Add(1)
		go func(val bool) {
			defer wg.Done()
			bm.AtomicSet(42, val)
		}(i%2 == 0)
	}
	wg.Wait()
	// Result is non-deterministic (last CAS wins), but must not be corrupted.
	_ = bm.Get(42) // just verify no panic / corruption
}

// TestAtomicOrFromParallel: N goroutines each AtomicOrFrom a different source
// bitmap into a shared target. All contributed bits must be visible at the end.
func TestAtomicOrFromParallel(t *testing.T) {
	var target NonBlockingBitMap
	sources := make([]NonBlockingBitMap, parallelWorkers)
	for i := range sources {
		sources[i].Set(uint(i), true)
	}

	var wg sync.WaitGroup
	for i := range sources {
		wg.Add(1)
		go func(src *NonBlockingBitMap) {
			defer wg.Done()
			target.AtomicOrFrom(src, 0)
		}(&sources[i])
	}
	wg.Wait()

	for i := 0; i < parallelWorkers; i++ {
		if !target.Get(uint(i)) {
			t.Fatalf("bit %d missing after parallel AtomicOrFrom", i)
		}
	}
}

// TestAtomicOrFromGrowParallel: all goroutines target the same word after a
// large offset, forcing concurrent slice growth.
func TestAtomicOrFromGrowParallel(t *testing.T) {
	var target NonBlockingBitMap
	var wg sync.WaitGroup
	const offset uint = 5000
	for i := 0; i < parallelWorkers; i++ {
		wg.Add(1)
		go func(bit uint) {
			defer wg.Done()
			var src NonBlockingBitMap
			src.Set(0, true)
			target.AtomicOrFrom(&src, offset+bit)
		}(uint(i))
	}
	wg.Wait()
	for i := 0; i < parallelWorkers; i++ {
		if !target.Get(offset + uint(i)) {
			t.Fatalf("bit %d missing after parallel AtomicOrFrom with growth", offset+uint(i))
		}
	}
}

// TestConcurrentReadWrite: AtomicGet readers run concurrently with AtomicSet
// writers. Must not panic or race (verified by -race flag).
// Note: plain Get is NOT used here — it is only safe without concurrent
// writers. Use AtomicGet when Atomic* writers are active.
func TestConcurrentReadWrite(t *testing.T) {
	var bm NonBlockingBitMap
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writers
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(base uint) {
			defer wg.Done()
			for j := uint(0); j < 200; j++ {
				bm.AtomicSet(base+j, true)
				select {
				case <-stop:
					return
				default:
				}
			}
		}(uint(i) * 200)
	}

	// Readers — AtomicGet only while Atomic* writers are active
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(base uint) {
			defer wg.Done()
			for j := uint(0); j < 500; j++ {
				_ = bm.AtomicGet((base + j) % 1600)
				_ = bm.AtomicGet((base + j + 13) % 1600)
				select {
				case <-stop:
					return
				default:
				}
			}
		}(uint(i) * 100)
	}

	wg.Wait()
	close(stop)
}

// TestAtomicAndNotFromParallel: first set many bits, then N goroutines each
// AtomicAndNotFrom a different bit. All cleared bits must be gone.
func TestAtomicAndNotFromParallel(t *testing.T) {
	var target NonBlockingBitMap
	for i := 0; i < parallelWorkers; i++ {
		target.Set(uint(i), true)
	}

	var wg sync.WaitGroup
	for i := 0; i < parallelWorkers; i++ {
		wg.Add(1)
		go func(bit uint) {
			defer wg.Done()
			var mask NonBlockingBitMap
			mask.Set(0, true)
			target.AtomicAndNotFrom(&mask, bit) // clears bit `bit` in target
		}(uint(i))
	}
	wg.Wait()

	for i := 0; i < parallelWorkers; i++ {
		if target.Get(uint(i)) {
			t.Fatalf("bit %d should be cleared after parallel AtomicAndNotFrom", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Diagnostic / documentation helper
// ---------------------------------------------------------------------------

func TestPrintSizes(t *testing.T) {
	var empty NonBlockingBitMap
	t.Logf("empty NonBlockingBitMap ComputeSize = %d bytes", empty.ComputeSize())
	empty.Set(63, true)
	t.Logf("1-word NonBlockingBitMap ComputeSize = %d bytes", empty.ComputeSize())
}
