/*
Copyright (C) 2026  Carl-Philip Hänsch

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

import "sync"
import "sync/atomic"
import "golang.org/x/exp/constraints"

type readMapEntry[K constraints.Ordered, V any] struct {
	key   K
	value V
}

// readMapSnapshot is immutable after publication. The values slice makes
// GetAll allocation-free without exposing the searchable entry array.
type readMapSnapshot[K constraints.Ordered, V any] struct {
	entries []readMapEntry[K, V]
	values  []V
}

// ReadMap is a read-optimized ordered key/value map. Readers only load one
// atomically published immutable snapshot and never acquire a lock. Writes are
// serialized because rebuilding the same snapshot concurrently only wastes
// work; they never block readers.
//
// A ReadMap must be constructed with NewReadMap and must not be copied. Values
// returned by GetAll belong to the immutable snapshot and must not be assigned
// through the returned slice.
type ReadMap[K constraints.Ordered, V any] struct {
	writes   sync.Mutex
	snapshot atomic.Pointer[readMapSnapshot[K, V]]
}

func NewReadMap[K constraints.Ordered, V any]() *ReadMap[K, V] {
	result := new(ReadMap[K, V])
	result.snapshot.Store(new(readMapSnapshot[K, V]))
	return result
}

func (m *ReadMap[K, V]) current() *readMapSnapshot[K, V] {
	return m.snapshot.Load()
}

// Get returns the value associated with key, or the zero value of V when the
// key is absent.
func (m *ReadMap[K, V]) Get(key K) V {
	snapshot := m.current()
	if snapshot == nil {
		var zero V
		return zero
	}
	entries := snapshot.entries
	lower, upper := 0, len(entries)
	for lower < upper {
		pivot := (lower + upper) / 2
		if entries[pivot].key == key {
			return entries[pivot].value
		}
		if entries[pivot].key < key {
			lower = pivot + 1
		} else {
			upper = pivot
		}
	}
	var zero V
	return zero
}

// GetAll returns the values in key order. The returned snapshot is read-only.
func (m *ReadMap[K, V]) GetAll() []V {
	if snapshot := m.current(); snapshot != nil {
		return snapshot.values
	}
	return nil
}

// Set atomically publishes value under key and returns the previous value, or
// the zero value of V when the key did not exist.
func (m *ReadMap[K, V]) Set(key K, value V) V {
	m.writes.Lock()
	defer m.writes.Unlock()

	old := m.current()
	var oldEntries []readMapEntry[K, V]
	var oldValues []V
	if old != nil {
		oldEntries = old.entries
		oldValues = old.values
	}
	index := searchReadMap(oldEntries, key)
	length := len(oldEntries)
	if index < length && oldEntries[index].key == key {
		entries := make([]readMapEntry[K, V], length)
		values := make([]V, length)
		copy(entries, oldEntries)
		copy(values, oldValues)
		previous := entries[index].value
		entries[index].value = value
		values[index] = value
		m.snapshot.Store(&readMapSnapshot[K, V]{entries: entries, values: values})
		return previous
	}

	entries := make([]readMapEntry[K, V], length+1)
	values := make([]V, length+1)
	copy(entries, oldEntries[:index])
	copy(entries[index+1:], oldEntries[index:])
	copy(values, oldValues[:index])
	copy(values[index+1:], oldValues[index:])
	entries[index] = readMapEntry[K, V]{key: key, value: value}
	values[index] = value
	m.snapshot.Store(&readMapSnapshot[K, V]{entries: entries, values: values})
	var zero V
	return zero
}

// Remove atomically removes key and returns its previous value, or the zero
// value of V when the key did not exist.
func (m *ReadMap[K, V]) Remove(key K) V {
	m.writes.Lock()
	defer m.writes.Unlock()

	old := m.current()
	if old == nil {
		var zero V
		return zero
	}
	index := searchReadMap(old.entries, key)
	if index == len(old.entries) || old.entries[index].key != key {
		var zero V
		return zero
	}
	entries := make([]readMapEntry[K, V], len(old.entries)-1)
	values := make([]V, len(old.values)-1)
	copy(entries, old.entries[:index])
	copy(entries[index:], old.entries[index+1:])
	copy(values, old.values[:index])
	copy(values[index:], old.values[index+1:])
	previous := old.entries[index].value
	m.snapshot.Store(&readMapSnapshot[K, V]{entries: entries, values: values})
	return previous
}

func searchReadMap[K constraints.Ordered, V any](entries []readMapEntry[K, V], key K) int {
	lower, upper := 0, len(entries)
	for lower < upper {
		pivot := (lower + upper) / 2
		if entries[pivot].key < key {
			lower = pivot + 1
		} else {
			upper = pivot
		}
	}
	return lower
}
