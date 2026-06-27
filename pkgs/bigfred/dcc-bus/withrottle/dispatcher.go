package withrottle

import (
	"sync"
)

type task func()

type dispatcher struct {
	shards []chan task
	wg     sync.WaitGroup
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
	if d == nil {
		return
	}
	ch := d.shards[shardIndex(key, len(d.shards))]
	ch <- t
}

func (d *dispatcher) InlineFallbacks() int64 { return 0 }

func (d *dispatcher) close() {
	for _, ch := range d.shards {
		close(ch)
	}
	d.wg.Wait()
}
