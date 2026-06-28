package withrottle

import (
	"sync"
	"sync/atomic"
)

type task func()

type dispatcher struct {
	shards  []chan task
	wg      sync.WaitGroup
	inline  atomic.Int64
	closed  atomic.Bool
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
				d.runTask(t)
			}
		}()
	}
	return d
}

func (d *dispatcher) runTask(t task) {
	if t == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			// Panic is contained so one bad line cannot kill the shard worker.
			_ = r
		}
	}()
	t()
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
		if t != nil {
			t()
		}
		return
	}
	if d.closed.Load() {
		d.inline.Add(1)
		d.runTask(t)
		return
	}
	ch := d.shards[shardIndex(key, len(d.shards))]
	select {
	case ch <- t:
	default:
		d.inline.Add(1)
		d.runTask(t)
	}
}

func (d *dispatcher) InlineFallbacks() int64 { return d.inline.Load() }

func (d *dispatcher) close() {
	if d == nil {
		return
	}
	d.closed.Store(true)
	for _, ch := range d.shards {
		close(ch)
	}
	d.wg.Wait()
}
