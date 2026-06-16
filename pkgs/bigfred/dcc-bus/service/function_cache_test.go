package service

import "testing"

func TestFunctionsCacheMatchesAndRollback(t *testing.T) {
	t.Parallel()
	c := NewFunctionsCache()
	if c.Matches(31, 0, true) {
		t.Fatal("empty cache should not match on")
	}
	c.Set(31, 0, true)
	if !c.Matches(31, 0, true) {
		t.Fatal("expected match after set")
	}
	prev, had := c.Stage(31, 0, false)
	if !had || !prev {
		t.Fatalf("prev=%v had=%v", prev, had)
	}
	c.Rollback(31, 0, true, true)
	if !c.Matches(31, 0, true) {
		t.Fatal("rollback should restore prior on state")
	}
}
