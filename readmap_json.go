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
import "sort"

func (m *ReadMap[K, V]) MarshalJSON() ([]byte, error) {
	temporary := make(map[K]V)
	if snapshot := m.current(); snapshot != nil {
		for _, entry := range snapshot.entries {
			temporary[entry.key] = entry.value
		}
	}
	return json.Marshal(temporary)
}

func (m *ReadMap[K, V]) UnmarshalJSON(data []byte) error {
	temporary := make(map[K]V)
	if err := json.Unmarshal(data, &temporary); err != nil {
		return err
	}
	keys := make([]K, 0, len(temporary))
	for key := range temporary {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left, right int) bool { return keys[left] < keys[right] })
	entries := make([]readMapEntry[K, V], len(keys))
	values := make([]V, len(keys))
	for index, key := range keys {
		value := temporary[key]
		entries[index] = readMapEntry[K, V]{key: key, value: value}
		values[index] = value
	}
	m.writes.Lock()
	defer m.writes.Unlock()
	m.snapshot.Store(&readMapSnapshot[K, V]{entries: entries, values: values})
	return nil
}
