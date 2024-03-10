package NonLockingReadMap

import "fmt"
import "testing"

type KeyValue struct {
	key, value string
}

// implement the GetKey interface
func (kv KeyValue) GetKey() string {
	return kv.key
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
	if item2.value != "Peter" {
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
	} else if m.Get("job").value != "Developer" {
		t.Fatalf("Easy Set failed II")
	}

}

func TestConcurrentRead(t *testing.T) {
	// create
	m := New[KeyValue, string]()

	// serial write
	for i := 0; i < 2048; i++ {
		item := &KeyValue{fmt.Sprintf("key%d", i), fmt.Sprintf("value %d", i)}
		m.Set(item)
	}

	// concurrent read
	done := make(chan bool, 10)
	for i := 0; i < 10000; i++ {
		go func (i int) {
			for j := 0; j < 10000; j++ {
				num := (101 * i + j + 13) % 2050
				item := m.Get(fmt.Sprintf("key%d", num))
				if num >= 2048 && item != nil {
					t.Fatalf("concurrent nonexisting read fail")
				} else if num < 2048 && item == nil {
					t.Fatalf("concurrent read fail I")
				} else if num < 2048 && item.value != fmt.Sprintf("value %d", num) {
					t.Fatalf("concurrent read fail II")
				}
			}
			done <- true
		}(i)
	}

	for i := 0; i < 1000; i++ {
		// collect all threads
		<- done
	}
}

func TestConcurrentWrite(t *testing.T) {
	// create
	m := New[KeyValue, string]()

	// serial write
	for i := 0; i < 2048; i++ {
		item := &KeyValue{fmt.Sprintf("key%d", i), fmt.Sprintf("value %d", i)}
		m.Set(item)
	}

	// concurrent read
	done := make(chan int, 10)
	for i := 0; i < 1000; i++ {
		go func (i int) {
			for pass := 0; pass < 4; pass++ {
				for j := 0; j < 10000; j++ {
					num := (101 * i + j + 13) % 2050
					item := m.Get(fmt.Sprintf("key%d", num))
					if num >= 2048 && item != nil {
						t.Fatalf("concurrent nonexisting read fail")
					} else if num < 2048 && item == nil {
						t.Fatalf("concurrent read fail I")
					} else if num < 2048 && item.value != fmt.Sprintf("value %d", num) && item.value != fmt.Sprintf("value %d-new", num) {
						t.Fatalf("concurrent read fail II")
					}
				}
				m.Set(&KeyValue{fmt.Sprintf("key%d", i), fmt.Sprintf("value %d-new", i)})
			}
			done <- i
		}(i)
	}

	for i := 0; i < 1000; i++ {
		// collect all threads
		num := <- done
		// check if they did their set
		item := m.Get(fmt.Sprintf("key%d", num))
		if item == nil {
			t.Fatalf("Concurrent Set failed I with thread %d", num)
		} else if item.value != fmt.Sprintf("value %d-new", num) {
			t.Fatalf("Concurrent Set failed II with thread %d", num)
		}
	}
}
