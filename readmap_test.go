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

import "encoding/json"
import "fmt"
import "sync"
import "testing"

func TestReadMap(t *testing.T) {
	m := NewReadMap[string, *KeyValue]()
	first := &KeyValue{Key: "name", Value: "Peter"}
	if previous := m.Set("name", first); previous != nil {
		t.Fatalf("first Set returned %v", previous)
	}
	if got := m.Get("name"); got != first {
		t.Fatalf("Get returned %v, want %v", got, first)
	}
	if got := m.Get("missing"); got != nil {
		t.Fatalf("missing Get returned %v", got)
	}

	snapshot := m.GetAll()
	replacement := &KeyValue{Key: "name", Value: "Jane"}
	if previous := m.Set("name", replacement); previous != first {
		t.Fatalf("replacement returned %v, want %v", previous, first)
	}
	if snapshot[0] != first {
		t.Fatal("replacement mutated a previously published snapshot")
	}
	if got := m.Remove("name"); got != replacement {
		t.Fatalf("Remove returned %v, want %v", got, replacement)
	}
	if got := m.Remove("name"); got != nil {
		t.Fatalf("second Remove returned %v", got)
	}
}

func TestReadMapOrdering(t *testing.T) {
	m := NewReadMap[string, string]()
	for _, key := range []string{"zulu", "alpha", "middle", "beta"} {
		m.Set(key, key)
	}
	got := m.GetAll()
	want := []string{"alpha", "beta", "middle", "zulu"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("GetAll()[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestReadMapJSON(t *testing.T) {
	m := NewReadMap[string, *KeyValue]()
	m.Set("name", &KeyValue{Key: "ignored", Value: "Peter"})
	m.Set("job", &KeyValue{Key: "ignored", Value: "Developer"})
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ReadMap[string, *KeyValue]
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded.Get("name"); got == nil || got.Value != "Peter" {
		t.Fatalf("decoded name = %v", got)
	}
}

func TestReadMapConcurrentSnapshots(t *testing.T) {
	m := NewReadMap[int, int]()
	const count = 128
	for index := 0; index < count; index++ {
		m.Set(index, index)
	}
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for pass := 0; pass < 1000; pass++ {
				key := (worker + pass) % count
				_ = m.Get(key)
				values := m.GetAll()
				if len(values) != count {
					t.Errorf("snapshot length = %d, want %d", len(values), count)
					return
				}
			}
		}(worker)
	}
	for worker := 0; worker < 4; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for pass := 0; pass < 250; pass++ {
				key := (worker + pass) % count
				m.Set(key, worker*1000+pass)
			}
		}(worker)
	}
	wait.Wait()
}

func BenchmarkReadMapGet(b *testing.B) {
	for _, count := range []int{1, 16, 256, 2048} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			m := NewReadMap[string, *KeyValue]()
			keys := make([]string, count)
			for index := 0; index < count; index++ {
				key := fmt.Sprintf("key-%04d", index)
				keys[index] = key
				m.Set(key, &KeyValue{Key: key, Value: key})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				if m.Get(keys[index&(count-1)]) == nil {
					b.Fatal("missing benchmark key")
				}
			}
		})
	}
}
