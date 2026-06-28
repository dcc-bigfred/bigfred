package state

import (
	"sync"
	"testing"
)

func TestLocoLocksSerializeSameAddr(t *testing.T) {
	locks := NewLocoLocks()
	const addr uint16 = 42
	order := make([]int, 0, 2)
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		unlock := locks.Acquire(addr)
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
		unlock()
	}()
	go func() {
		defer wg.Done()
		unlock := locks.Acquire(addr)
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
		unlock()
	}()
	wg.Wait()
	if len(order) != 2 {
		t.Fatalf("order len = %d, want 2", len(order))
	}
}
