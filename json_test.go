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
