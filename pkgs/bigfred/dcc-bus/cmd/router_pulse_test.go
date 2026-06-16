package cmd

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
)

func TestPulseOverlapRejectedUntilEnd(t *testing.T) {
	t.Parallel()
	r := &Router{pulseActive: make(map[service.FnKey]bool)}
	key := service.FnKey{Addr: 31, Fn: 2}

	if !r.markTimedFunctionStarted(key) {
		t.Fatal("expected first pulse to begin")
	}
	if r.markTimedFunctionStarted(key) {
		t.Fatal("overlapping pulse should be rejected")
	}
	r.markTimedFunctionStopped(key)
	if !r.markTimedFunctionStarted(key) {
		t.Fatal("expected pulse after end")
	}
	r.markTimedFunctionStopped(key)
}

func TestPulseDifferentFunctionsIndependent(t *testing.T) {
	t.Parallel()
	r := &Router{pulseActive: make(map[service.FnKey]bool)}
	if !r.markTimedFunctionStarted(service.FnKey{Addr: 31, Fn: 0}) {
		t.Fatal("F0 pulse")
	}
	if !r.markTimedFunctionStarted(service.FnKey{Addr: 31, Fn: 1}) {
		t.Fatal("F1 pulse should not block F0")
	}
}
