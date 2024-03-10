package NonLockingReadMap

import "testing"
import "encoding/json"


func TestJSON(t *testing.T) {
	// create
	m := New[KeyValue, string]()

	// write
	item := &KeyValue{"name", "Peter"}
	m.Set(item)
	item = &KeyValue{"job", "Developer"}
	m.Set(item)

	// serialize
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal error")
	}
	if string(b) != "{\"job\":{\"Key\":\"job\",\"Value\":\"Developer\"},\"name\":{\"Key\":\"name\",\"Value\":\"Peter\"}}" {
		t.Fatalf("json output mismatch: " + string(b))
	}

	var m2 NonLockingReadMap[KeyValue, string]
	json.Unmarshal(b, &m2)

	// read
	item2 := m2.Get("name")
	if item2 == nil {
		t.Fatalf("JSON Getter failed I")
	}
	if item2.Value != "Peter" {
		t.Fatalf("JSON Getter failed II")
	}

	// nonexisting read
	item3 := m2.Get("doesnotexist")
	if item3 != nil {
		t.Fatalf("nonexisting Get failed")
	}

	// remove
	m2.Remove("name")
	if m2.Get("name") != nil {
		t.Fatalf("Remove failed")
	}

	// read after remove in old
	item2 = m.Get("name")
	if item2 == nil {
		t.Fatalf("Getter after remove failed II")
	}
	if item2.Value != "Peter" {
		t.Fatalf("Getter after remove failed II")
	}

}
