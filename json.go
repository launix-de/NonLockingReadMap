package NonLockingReadMap

import "sort"
import "encoding/json"

func (m NonLockingReadMap[T, TK]) MarshalJSON() ([]byte, error) {
	// serialize through map (inefficient but nobody cares for now)
	temp := make(map[TK]*T)
	for _, v := range m.GetAll() {
		temp[(*v).GetKey()] = v
	}
	return json.Marshal(temp)
}

func (m *NonLockingReadMap[T, TK]) UnmarshalJSON(b []byte) (error) {
	// deserialize through map (inefficient but nobody cares for now)
	newhandle := new([]*T)
	temp := make(map[TK]*T)
	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}
	*newhandle = make([]*T, len(temp))
	i := 0
	for _, v := range temp {
		(*newhandle)[i] = v
		i++
	}
	sort.Slice(*newhandle, func (i, j int) bool { // sort
		return (*(*newhandle)[i]).GetKey() < (*(*newhandle)[j]).GetKey()
	})
	m.p.Store(newhandle) // forcably store since we are still in single core context, no value escaping yet
	return nil
}

