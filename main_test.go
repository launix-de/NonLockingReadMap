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

import "fmt"
import "testing"

type KeyValue struct {
	Key, Value string
}

func (x KeyValue) ComputeSize() uint {
	return 16 + 8*(uint(len(x.Key)-1)/8+1) + 16 + 8*(uint(len(x.Value)-1)/8+1)
}

// implement the GetKey interface
func (kv KeyValue) GetKey() string {
	return kv.Key
}

func TestCreate(t *testing.T) {
	New[KeyValue, string]()
	/*
		if m != nil {
			t.Fatalf("New returned nil")
		}*/

}

func TestAll(t *testing.T) {
	// create
	m := New[KeyValue, string]()

	// write
	item := &KeyValue{"name", "Peter"}
	m.Set(item)

	// read
	item2 := m.Get("name")
	if item2.Value != "Peter" {
		t.Fatalf("Getter failed")
	}

	// nonexisting read
	item3 := m.Get("doesnotexist")
	if item3 != nil {
		t.Fatalf("nonexisting Get failed")
	}

	// remove
	m.Remove("name")
	if m.Get("name") != nil {
		t.Fatalf("Remove failed")
	}

	// easy set
	m.Set(&KeyValue{"job", "Developer"})
	if m.Get("job") == nil {
		t.Fatalf("Easy Set failed I")
	} else if m.Get("job").Value != "Developer" {
		t.Fatalf("Easy Set failed II")
	}

}

func TestSetReplacesWithoutDuplicatingKey(t *testing.T) {
	m := New[KeyValue, string]()
	old := &KeyValue{Key: "same", Value: "old"}
	newValue := &KeyValue{Key: "same", Value: "new"}
	if replaced := m.Set(old); replaced != nil {
		t.Fatalf("first Set replaced %v", replaced)
	}
	snapshot := m.GetAll()
	if replaced := m.Set(newValue); replaced != old {
		t.Fatalf("second Set replaced %v, want old value", replaced)
	}
	if snapshot[0] != old {
		t.Fatal("replacement mutated a previously published snapshot")
	}
	items := m.p.Load()
	if len(*items) != 1 {
		t.Fatalf("replacement left %d entries, want 1", len(*items))
	}
	if got := m.Get("same"); got != newValue {
		t.Fatalf("Get returned %v, want replacement", got)
	}
}

func TestSetMaintainsSortedSearchOrder(t *testing.T) {
	m := New[KeyValue, string]()
	for _, key := range []string{"zulu", "alpha", "middle", "beta"} {
		m.Set(&KeyValue{Key: key, Value: key})
	}
	for _, key := range []string{"alpha", "beta", "middle", "zulu"} {
		if got := m.Get(key); got == nil || got.Value != key {
			t.Fatalf("Get(%q) = %v after out-of-order inserts", key, got)
		}
	}
	items := m.p.Load()
	for index := 1; index < len(*items); index++ {
		if (*(*items)[index-1]).Key >= (*(*items)[index]).Key {
			t.Fatalf("entries are not sorted at index %d: %q >= %q", index,
				(*(*items)[index-1]).Key, (*(*items)[index]).Key)
		}
	}
}

func TestConcurrentRead(t *testing.T) {
	const workers = 128
	const readsPerWorker = 1000
	// create
	m := New[KeyValue, string]()

	// serial write
	for i := 0; i < 2048; i++ {
		item := &KeyValue{fmt.Sprintf("key%d", i), fmt.Sprintf("value %d", i)}
		m.Set(item)
	}

	// concurrent read
	done := make(chan bool, workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			for j := 0; j < readsPerWorker; j++ {
				num := (101*i + j + 13) % 2050
				item := m.Get(fmt.Sprintf("key%d", num))
				if num >= 2048 && item != nil {
					t.Errorf("concurrent nonexisting read fail")
					break
				} else if num < 2048 && item == nil {
					t.Errorf("concurrent read fail I")
					break
				} else if num < 2048 && item.Value != fmt.Sprintf("value %d", num) {
					t.Errorf("concurrent read fail II")
					break
				}
			}
			done <- true
		}(i)
	}

	for i := 0; i < workers; i++ {
		// collect all threads
		<-done
	}
}

func TestConcurrentWrite(t *testing.T) {
	const workers = 64
	const readsPerPass = 1000
	// create
	m := New[KeyValue, string]()

	// serial write
	for i := 0; i < 2048; i++ {
		item := &KeyValue{fmt.Sprintf("key%d", i), fmt.Sprintf("value %d", i)}
		m.Set(item)
	}

	// concurrent read
	done := make(chan int, workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer func() { done <- i }()
			for pass := 0; pass < 4; pass++ {
				for j := 0; j < readsPerPass; j++ {
					num := (101*i + j + 13) % 2050
					item := m.Get(fmt.Sprintf("key%d", num))
					if num >= 2048 && item != nil {
						t.Errorf("concurrent nonexisting read fail")
						return
					} else if num < 2048 && item == nil {
						t.Errorf("concurrent read fail I")
						return
					} else if num < 2048 && item.Value != fmt.Sprintf("value %d", num) && item.Value != fmt.Sprintf("value %d-new", num) {
						t.Errorf("concurrent read fail II")
						return
					}
				}
				m.Set(&KeyValue{fmt.Sprintf("key%d", i), fmt.Sprintf("value %d-new", i)})
			}
		}(i)
	}

	for i := 0; i < workers; i++ {
		// collect all threads
		num := <-done
		// check if they did their set
		item := m.Get(fmt.Sprintf("key%d", num))
		if item == nil {
			t.Fatalf("Concurrent Set failed I with thread %d", num)
		} else if item.Value != fmt.Sprintf("value %d-new", num) {
			t.Fatalf("Concurrent Set failed II with thread %d", num)
		}
	}
}
