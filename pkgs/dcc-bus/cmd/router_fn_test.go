package cmd

import (
	"context"
	"testing"
)

func TestFnStateMatchesWithoutCacheEntry(t *testing.T) {
	t.Parallel()
	r := &Router{fnCache: make(map[fnKey]bool)}
	if r.checkFnStateMatches(context.Background(), 31, 0, true) {
		t.Fatal("empty fnCache must not suppress a function command")
	}
}

func TestSeedFnCacheFromSubscribeSnapshot(t *testing.T) {
	t.Parallel()
	r := &Router{fnCache: make(map[fnKey]bool)}
	r.fnCache[fnKey{Addr: 31, Fn: 0}] = true
	r.fnCache[fnKey{Addr: 31, Fn: 1}] = true

	r.seedFnCache(31, []bool{false, false, true})

	if r.fnCache[fnKey{Addr: 31, Fn: 0}] {
		t.Fatal("expected F0 seeded to false")
	}
	if r.fnCache[fnKey{Addr: 31, Fn: 1}] {
		t.Fatal("expected F1 seeded to false")
	}
	if !r.fnCache[fnKey{Addr: 31, Fn: 2}] {
		t.Fatal("expected F2 seeded to true")
	}
}
