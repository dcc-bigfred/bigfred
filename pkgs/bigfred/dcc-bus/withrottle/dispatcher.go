package withrottle

import (
	"sync"
	"sync/atomic"
)

type task func()

type dispatcher struct {
	shards []chan task
	wg     sync.WaitGroup
	inline atomic.Int64
}

func newDispatcher(shards, buf int) *dispatcher {
	if shards <= 0 {
		shards = 1
	}
	d := &dispatcher{shards: make([]chan task, shards)}
	for i := range d.shards {
		ch := make(chan task, buf)
		d.shards[i] = ch
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			for t := range ch {
				if t != nil {
					t()
				}
			}
		}()
	}
	return d
}

func shardIndex(key string, shards int) int {
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return int(h % uint32(shards))
}

func (d *dispatcher) dispatch(key string, t task) {
	ch := d.shards[shardIndex(key, len(d.shards))]
	select {
	case ch <- t:
	default:
		d.inline.Add(1)
		t()
	}
}

func (d *dispatcher) InlineFallbacks() int64 { return d.inline.Load() }

func (d *dispatcher) close() {
	for _, ch := range d.shards {
		close(ch)
	}
	d.wg.Wait()
}
