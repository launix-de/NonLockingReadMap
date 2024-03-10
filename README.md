# NonLockingReadMap
Golang implementation of a non locking read optimized map


properties of this map:
- read in O(log(N))
- read is always nonblocking and nonlocking
- write in O(N*log(N))
- write is optimistic, worst case is a eternal loop (with a probability of 0%)
- use this map if you read often but write very seldom
- internally, a ordered list is rebuilt each time there is a write

## Interface

```go
import "github.com/launix-de/NonLockingReadMap"

/*
the interface needs two types: T and TK.
T must be a pointer to something,
TK must be a string, int or float for the keys.
T must provide a method GetKey() TK
*/

func New[T, TK]() NonLockingReadMap[T1, T2]
func (m *NonLockingReadMap[T, TK]) GetAll() []*T
func (m *NonLockingReadMap[T, TK]) Get(key TK) *T
func (m *NonLockingReadMap[T, TK]) Set(v *T) *T
func (m *NonLockingReadMap[T, TK]) Remove(key TK) *T
```

## Example

```go
import "github.com/launix-de/NonLockingReadMap"

type KeyValue struct {
	key, value string
}

// implement the GetKey interface
func (kv KeyValue) GetKey() string {
	return kv.key
}

func main() {
	// create
	m := NonLockingReadMap.New[KeyValue, string]()

	// write
	item := &KeyValue{"name", "Peter"}
	m.Set(item) // will return the old object

	// read
	item2 := m.Get("name")
	fmt.Println("name = " + item2.value)

	// remove
	m.Remove("name")
}
```
