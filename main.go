package NonLockingReadMap

import "sort"
import "sync/atomic"
import "golang.org/x/exp/constraints"

/*
 this is a read optimized map.

 properties of this map:
  - read in O(log(N))
  - read is always nonblocking
  - write in O(N*log(N))
  - write is optimistic, worst case is a eternal loop (with a probability of 0%)
  - use this map if you read often but write very seldom
  - internally, a ordered list is rebuilt each time there is a write

*/

type KeyGetter[TK any] interface {
	GetKey() TK
}
type NonLockingReadMap[T *KeyGetter[TK], TK constraints.Ordered] struct {
	p atomic.Pointer[[]T]
}
type NonLockingReadMapContent[T any] []T

/* implement sort.Interface */
func (x NonLockingReadMapContent[T]) Len() int {
	return len(x)
}

func (m *NonLockingReadMap[T, TK]) Get(key TK) T {
	v, _ := m.GetHandle(key)
	return v
}

func (m *NonLockingReadMap[T, TK]) GetHandle(key TK) (T, *[]T) {
	items := m.p.Load() // atomically work on the current list
	var lower int = 0
	var upper int = len(*items)
	for {
		if lower == upper {
			return nil, items // item does not exist
		}
		pivot := (lower + upper) / 2
		item := (*items)[pivot]
		itemkey := (*item).GetKey()
		if key == itemkey {
			return item, items // found item
		} else if key < itemkey {
			upper = pivot
		} else {
			lower = pivot + 1
		}
	}
}

/* in case, true is returned, the value is safed. Otherwise, you must repeat GetHandle + SetHandle */
func (m *NonLockingReadMap[T, TK]) SetHandle(v T, handle *[]T) bool {
	newhandle := new([]T) // new pointer wrapper
	*newhandle = make([]T, len(*handle) + 1) // create new slice
	*newhandle = append(*newhandle, (*handle)...) // copy old array
	*newhandle = append(*newhandle, v) // add new item
	sort.Slice(*newhandle, func (i, j int) bool { // sort
		return (*(*newhandle)[i]).GetKey() < (*(*newhandle)[j]).GetKey()
	})
	return m.p.CompareAndSwap(handle, newhandle)
}


