package z21server

import (
	"testing"
)

func TestShardIndexStable(t *testing.T) {
	a := shardIndex("z21:10.0.0.1:40001", dispatchShards)
	b := shardIndex("z21:10.0.0.1:40001", dispatchShards)
	if a != b {
		t.Fatalf("shard index not stable: %d vs %d", a, b)
	}
	if a < 0 || a >= dispatchShards {
		t.Fatalf("shard out of range: %d", a)
	}
}

func TestDispatcherPreservesPerKeyOrder(t *testing.T) {
	d := newDispatcher(4, 8)

	seq := make([]int, 0, 5)
	done := make(chan struct{})
	for i := 0; i < 5; i++ {
		i := i
		d.dispatch("z21:1.2.3.4:5", func() {
			seq = append(seq, i)
		})
	}
	// close drains the workers (happens-before the assertions below).
	d.close()
	close(done)

	if len(seq) != 5 {
		t.Fatalf("expected 5 tasks ran, got %d", len(seq))
	}
	for i, v := range seq {
		if v != i {
			t.Fatalf("per-key order broken: seq=%v", seq)
		}
	}
}

func TestDispatcherInlineFallbackCounted(t *testing.T) {
	d := newDispatcher(1, 1)

	// Block the single worker so the queue fills and the next dispatch
	// runs inline.
	block := make(chan struct{})
	ran := make(chan struct{})
	d.dispatch("k", func() {
		<-block
	})
	// Fill the 1-slot buffer.
	d.dispatch("k", func() {})
	// This one must run inline.
	d.dispatch("k", func() {
		close(ran)
	})
	<-ran
	if got := d.InlineFallbacks(); got < 1 {
		t.Fatalf("expected inline fallback counted, got %d", got)
	}
	close(block)
	d.close()
}
