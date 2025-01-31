package utils

import (
	"container/heap"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestNewSkiplist(t *testing.T) {
	l := NewSkiplist()

	var keys []float64
	for i := 0; i < 1000; i++ {
		k := rand.Float64()
		if v := l.Get(k); v != nil {
			// do not overwrite
			continue
		}

		l.Set(k, k)
		keys = append(keys, k)
	}

	for i, k := range keys {
		require.Equal(t, k, l.Get(k).Value().(float64), strconv.Itoa(i))
	}
}

// -------------------------------------
// Heap
// -------------------------------------

// heapItem item that need to sort
type heapItem struct {
	p int
	k interface{}
}

// GetKey get key of item
func (it *heapItem) GetKey() interface{} {
	return it.k
}

// GetPriority get priority of item
func (it *heapItem) GetPriority() int {
	return it.p
}

var (
	itemsWaitToSort = HeapSlice{
		&heapItem{p: 1},
		&heapItem{p: 3},
		&heapItem{p: 55},
		&heapItem{p: 2},
		&heapItem{p: 4441},
		&heapItem{p: 15555},
		&heapItem{p: 122},
	}
)

func ExampleGetLargestNItems() {
	var (
		itemsWaitToSort = HeapSlice{
			&heapItem{p: 1},
			&heapItem{p: 3},
			&heapItem{p: 55},
			&heapItem{p: 2},
			&heapItem{p: 4441},
			&heapItem{p: 15555},
			&heapItem{p: 122},
		}
		itemChan = make(chan HeapItemItf)
	)

	go func() {
		for _, item := range itemsWaitToSort {
			itemChan <- item
		}

		close(itemChan)
	}()

	items, err := GetLargestNItems(itemChan, 3)
	if err != nil {
		panic(err)
	}

	for _, item := range items {
		// 15555
		// 4441
		// 112
		fmt.Println(item.GetPriority())
	}
}

func ExampleGetSmallestNItems() {
	var (
		itemsWaitToSort = HeapSlice{
			&heapItem{p: 1},
			&heapItem{p: 3},
			&heapItem{p: 55},
			&heapItem{p: 2},
			&heapItem{p: 4441},
			&heapItem{p: 15555},
			&heapItem{p: 122},
		}
		itemChan = make(chan HeapItemItf)
	)

	go func() {
		for _, item := range itemsWaitToSort {
			itemChan <- item
		}

		close(itemChan)
	}()

	items, err := GetSmallestNItems(itemChan, 3)
	if err != nil {
		panic(err)
	}

	for _, item := range items {
		// 1
		// 2
		// 3
		fmt.Println(item.GetPriority())
	}
}

func TestGetTopKItems(t *testing.T) {
	// defer utils.Logger.Sync()
	generate := func(itemChan chan HeapItemItf) {
		for _, item := range itemsWaitToSort {
			itemChan <- item
		}

		close(itemChan)
	}

	var (
		items    HeapSlice
		err      error
		itemChan chan HeapItemItf
	)

	// test highest
	itemChan = make(chan HeapItemItf)
	go generate(itemChan)
	items, err = GetTopKItems(itemChan, 3, true)
	if err != nil {
		t.Errorf("%+v", err)
	}

	if items[0].GetPriority() != 15555 {
		t.Errorf("expect 15555, got %+v", items[0].GetPriority())
	}
	if items[1].GetPriority() != 4441 {
		t.Errorf("expect 4441, got %+v", items[1].GetPriority())
	}
	if items[2].GetPriority() != 122 {
		t.Errorf("expect 122, got %+v", items[2].GetPriority())
	}

	// test lowest
	itemChan = make(chan HeapItemItf)
	go generate(itemChan)
	items, err = GetTopKItems(itemChan, 3, false)
	if err != nil {
		t.Errorf("%+v", err)
	}

	if items[0].GetPriority() != 1 {
		t.Errorf("expect 1, got %+v", items[0].GetPriority())
	}
	if items[1].GetPriority() != 2 {
		t.Errorf("expect 2, got %+v", items[1].GetPriority())
	}
	if items[2].GetPriority() != 3 {
		t.Errorf("expect 3, got %+v", items[2].GetPriority())
	}
}

func TestPriorityQ(t *testing.T) {
	for _, isMaxTop := range []bool{true, false} {
		q := NewPriorityQ(isMaxTop)
		heap.Init(q)
		var (
			v, n int
		)
		for i := 0; i < 10000; i++ {
			n = rand.Intn(100)
			if n < 50 {
				v = rand.Intn(1000)
				heap.Push(q, &heapItem{
					p: v,
					k: v,
				})
			} else if n < 75 {
				v = rand.Intn(1000)
				q.Remove(&heapItem{
					p: v,
					k: v,
				})
				heap.Init(q)
			} else {
				if q.Len() > 0 {
					heap.Pop(q)
				}
			}
		}

		heap.Push(q, &heapItem{
			p: 0,
			k: 0,
		})
		heap.Push(q, &heapItem{
			p: 1000,
			k: 1000,
		})

		results := make([]int, q.Len())[:0]
		var lastP, curP int
		for {
			if q.Len() == 0 {
				break
			}
			curP = heap.Pop(q).(*heapItem).GetPriority()
			if lastP != 0 {
				if isMaxTop && curP > lastP {
					t.Errorf("%v should <= %v", curP, lastP)
				} else if !isMaxTop && curP < lastP {
					t.Errorf("%v should >= %v", curP, lastP)
				}
			}

			lastP = curP
			results = append(results, curP)
		}
		t.Logf("%v[%v]: %v\n", isMaxTop, len(results), results[:10])
	}
	// t.Error("done")
}

func TestLimitSizeHeap(t *testing.T) {
	heap, err := NewLimitSizeHeap(5, true)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	var (
		it HeapItemItf
		n  int
	)
	for i := 0; i < 100; i++ {
		n = rand.Intn(1000)
		it = heap.Push(&heapItem{
			p: n,
			k: n,
		})
		if it != nil {
			t.Logf("push %v, pop %v", n, it.GetPriority())
		} else {
			t.Logf("push %v", n)
		}
	}

	var oldit HeapItemItf
	results := []int{}
	for {
		if it = heap.Pop(); it == nil {
			break
		}
		results = append(results, it.GetPriority())
		if oldit != nil {
			if oldit.GetPriority() > it.GetPriority() {
				t.Fatal(oldit.GetPriority(), "should <=", it.GetPriority(), ",", results)
			}
		}
		oldit = it
	}

	t.Log("results: ", results)
	// t.Error("done")
}

func BenchmarkLimitSizeHeap(b *testing.B) {
	heap5, err := NewLimitSizeHeap(5, true)
	if err != nil {
		b.Fatalf("%+v", err)
	}
	heap50, err := NewLimitSizeHeap(50, true)
	if err != nil {
		b.Fatalf("%+v", err)
	}
	heap500, err := NewLimitSizeHeap(500, true)
	if err != nil {
		b.Fatalf("%+v", err)
	}

	var n int
	b.Run("heap 5", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			n = rand.Intn(1000)
			heap5.Push(&heapItem{
				p: n,
				k: n,
			})
		}
	})
	b.Run("heap 50", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			n = rand.Intn(1000)
			heap50.Push(&heapItem{
				p: n,
				k: n,
			})
		}
	})
	b.Run("heap 500", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			n = rand.Intn(1000)
			heap500.Push(&heapItem{
				p: n,
				k: n,
			})
		}
	})

}

// -------------------------------------
// FIFO
// -------------------------------------

func ExampleFIFO() {
	f := NewFIFO()
	f.Put(1)
	v := f.Get()
	if v == nil {
		panic(v)
	}

	fmt.Println(v.(int))
	// Output: 1
}

func Test_UnsafePtr(t *testing.T) {
	var a int

	addr := unsafe.Pointer(&a)

	b := *(*int)(atomic.LoadPointer(&addr))
	require.Equal(t, a, b)
}

func TestNewFIFO(t *testing.T) {
	f := NewFIFO()
	var pool errgroup.Group
	start := make(chan struct{})

	var mu sync.Mutex
	var cnt int32
	var got []interface{}

	for i := 0; i < 100; i++ {
		pool.Go(func() error {
			<-start
			for i := 0; i < 100; i++ {
				switch rand.Intn(2) {
				case 0:
					f.Put(i)
					atomic.AddInt32(&cnt, 1)
				case 1:
					v := f.Get()
					if v != nil {
						mu.Lock()
						got = append(got, v)
						mu.Unlock()
					}
				}
			}

			return nil
		})
	}

	time.Sleep(time.Second)
	close(start)
	err := pool.Wait()
	require.NoError(t, err)
	f.Len()

	for {
		v := f.Get()
		if v == nil {
			break
		}

		got = append(got, v)
	}

	require.Equal(t, 0, f.Len(), "empty")
	require.Len(t, got, int(cnt), "total len")
}

// BenchmarkFIFO
//
//   cpu: Intel(R) Core(TM) i7-4790 CPU @ 3.60GHz
//   BenchmarkFIFO-8   	  368847	      3330 ns/op	      15 B/op	       0 allocs/op
func BenchmarkFIFO(b *testing.B) {
	b.Run("fifo", func(b *testing.B) {
		f := NewFIFO()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				switch rand.Intn(2) {
				case 0:
					f.Put(2)
				case 1:
					_ = f.Get()
				}
			}
		})
	})
}

func BenchmarkFIFOAndChan(b *testing.B) {

	b.Run("fifo", func(b *testing.B) {
		f := NewFIFO()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				for p.Next() {
					switch rand.Intn(2) {
					case 0:
						f.Put(2)
					case 1:
						_ = f.Get()
					}
				}
			}
		})
	})

	b.Run("channel struct", func(b *testing.B) {
		ch := make(chan struct{}, 10)
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				for p.Next() {
					switch rand.Intn(2) {
					case 0:
						select {
						case ch <- struct{}{}:
						default:
						}
					case 1:
						select {
						case <-ch:
						default:
						}
					}
				}
			}
		})
	})

	b.Run("channel int", func(b *testing.B) {
		ch := make(chan int, 10)
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				switch rand.Intn(2) {
				case 0:
					select {
					case ch <- 2:
					default:
					}
				case 1:
					select {
					case <-ch:
					default:
					}
				}
			}
		})
	})
}

func TestNewDeque(t *testing.T) {
	t.Run("err", func(t *testing.T) {
		var err error
		_, err = NewDeque(WithDequeCurrentCapacity(-1))
		require.Error(t, err)

		_, err = NewDeque(WithDequeMinimalCapacity(-1))
		require.Error(t, err)
	})

	d, err := NewDeque(
		WithDequeCurrentCapacity(0),
		WithDequeMinimalCapacity(0),
	)
	require.NoError(t, err)

	d.PushBack(10)
	d.PushFront(20)
	require.Equal(t, 2, d.Len())
	require.Equal(t, 20, d.PopFront())
	require.Equal(t, 10, d.PopBack())
}
