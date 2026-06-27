package z21server

import (
	"sync"
	"sync/atomic"
)

// task is one unit of work routed through the dispatcher.
type task func()

// dispatcher routes inbound UDP packets to a fixed pool of worker
// goroutines keyed by client key. Per-client ordering is preserved
// (every packet for one handset lands on the same shard and runs
// sequentially), while a slow Redis round-trip on one shard no longer
// stalls the entire UDP read loop — only that shard's clients degrade.
//
// The read loop never blocks on a saturated shard: when a shard's
// queue is full it falls back to running the task inline, which
// preserves liveness for the rest of the server at the cost of
// temporarily serialising that shard's clients. Inline fallbacks are
// counted so overload is observable.
type dispatcher struct {
	shards  []chan task
	wg      sync.WaitGroup
	inline  atomic.Int64
}

// newDispatcher starts shards worker goroutines, each with a buffered
// queue of depth buf.
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

// shardIndex maps key to a shard using an allocation-free FNV-1a hash.
// Avoiding the hash/fnv hasher allocation matters on the per-packet hot
// path (one dispatch per UDP datagram).
func shardIndex(key string, shards int) int {
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return int(h % uint32(shards))
}

// dispatch routes t to the worker owning key, or runs it inline when
// that worker's queue is full (backpressure without stalling the read
// loop). Inline fallbacks are counted via InlineFallbacks.
func (d *dispatcher) dispatch(key string, t task) {
	ch := d.shards[shardIndex(key, len(d.shards))]
	select {
	case ch <- t:
	default:
		d.inline.Add(1)
		t()
	}
}

// InlineFallbacks reports how many tasks ran in the read goroutine
// because a shard queue was full. A sustained non-zero rate signals
// overload on one shard (slow Redis, too few shards, or a hot client).
func (d *dispatcher) InlineFallbacks() int64 { return d.inline.Load() }

// close drains every shard and waits for the workers to exit.
func (d *dispatcher) close() {
	for _, ch := range d.shards {
		close(ch)
	}
	d.wg.Wait()
}
