package cmd

import "testing"

func TestPulseOverlapRejectedUntilEnd(t *testing.T) {
	t.Parallel()
	r := &Router{pulseActive: make(map[fnKey]bool)}
	key := fnKey{Addr: 31, Fn: 2}

	if !r.beginPulse(key) {
		t.Fatal("expected first pulse to begin")
	}
	if r.beginPulse(key) {
		t.Fatal("overlapping pulse should be rejected")
	}
	r.endPulse(key)
	if !r.beginPulse(key) {
		t.Fatal("expected pulse after end")
	}
	r.endPulse(key)
}

func TestPulseDifferentFunctionsIndependent(t *testing.T) {
	t.Parallel()
	r := &Router{pulseActive: make(map[fnKey]bool)}
	if !r.beginPulse(fnKey{Addr: 31, Fn: 0}) {
		t.Fatal("F0 pulse")
	}
	if !r.beginPulse(fnKey{Addr: 31, Fn: 1}) {
		t.Fatal("F1 pulse should not block F0")
	}
}
