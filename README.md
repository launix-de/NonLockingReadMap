# NonLockingReadMap
Golang implementation of a non locking read optimized map


properties of this map:
- read in O(log(N))
- read is always nonblocking and nonlocking
- write in O(N*log(N))
- write is optimistic, worst case is a eternal loop (with a probability of 0%)
- use this map if you read often but write very seldom
- internally, a ordered list is rebuilt each time there is a write
- this library uses atomic compare and swap in order to ensure safety in concurrency

Read more:
https://launix.de/launix/revisiting-non-blocking-maps-in-go/

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

## Benchmark
Test setup:
- AMD Ryzen 9 7900X3D 12-Core Processor
- 64GiB of DDR5 RAM
- 3D V-Cache


Read benchmark:
- 2048 items serial write
- 10,000 go routines
- each reading and checking 10,000 values
- in total 100,000,000 items read in 2.62s on 24 threads
- so 38M reads/sec

Write benchmark:
- 2048 items serial write
- 1,000 go routines
- each doing 4 passes
- each pass is 10,000 reads and one write
- in total 4,000 reads in 1.14s on 24 thread (during a heavvy read load)
- so 3,5K writes/sec (while rebuilding a 2,048 item list and having 10,000 reads between each write)

# NonBlockingBitMap

properties of this map:
  - non-blocking read
  - non-blocking write

## Interface

```go
import "github.com/launix-de/NonLockingReadMap"

/*
the interface needs two types: T and TK.
T must be a pointer to something,
TK must be a string, int or float for the keys.
T must provide a method GetKey() TK
*/

func NewBitMap() NonBlockingBitMap
func (b *NonBlockingBitMap) Reset()
func (b *NonBlockingBitMap) Get(i int) bool
func (b *NonBlockingBitMap) Set(i int, val bool)
```
