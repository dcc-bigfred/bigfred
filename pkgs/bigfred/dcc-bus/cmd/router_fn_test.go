package cmd

import (
	"context"
	"testing"
)

func TestFnStateMatchesWithoutCacheEntry(t *testing.T) {
	t.Parallel()
	r := &Router{fnCache: NewFunctionsCache()}
	if r.checkFnStateMatches(context.Background(), 31, 0, true) {
		t.Fatal("empty fnCache must not suppress a function command")
	}
}

func TestSeedFnCacheFromSubscribeSnapshot(t *testing.T) {
	t.Parallel()
	c := NewFunctionsCache()
	c.Set(31, 0, true)
	c.Set(31, 1, true)

	c.Seed(31, []bool{false, false, true})

	if on, _ := c.Get(31, 0); on {
		t.Fatal("expected F0 seeded to false")
	}
	if on, _ := c.Get(31, 1); on {
		t.Fatal("expected F1 seeded to false")
	}
	if on, ok := c.Get(31, 2); !ok || !on {
		t.Fatal("expected F2 seeded to true")
	}
}
