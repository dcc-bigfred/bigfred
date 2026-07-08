package cmd

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/state"
)

func TestCurrentFunctionState_defaultsOff(t *testing.T) {
	t.Parallel()
	r := &Router{store: state.NewLocoStateStore(nil, StateTTL, nil)}
	if r.currentFunctionState(31, 1) {
		t.Fatal("expected function off when store has no state")
	}
}

func TestCurrentFunctionState_readsStore(t *testing.T) {
	t.Parallel()
	store := state.NewLocoStateStore(nil, StateTTL, nil)
	store.SetFunction(31, 1, 1, true, "test")
	r := &Router{store: store}
	if !r.currentFunctionState(31, 1) {
		t.Fatal("expected F1 on from store")
	}
}

func TestHandleSetFunctionResolvesToggle(t *testing.T) {
	t.Parallel()
	store := state.NewLocoStateStore(nil, StateTTL, nil)
	store.SetFunction(31, 1, 1, true, "test")
	r := &Router{store: store}

	on := true
	pToggle := true
	if pToggle {
		on = !r.currentFunctionState(31, 1)
	}
	if on {
		t.Fatal("toggle should flip F1 from on to off")
	}

	store.SetFunction(31, 1, 1, false, "test")
	on = false
	if pToggle {
		on = !r.currentFunctionState(31, 1)
	}
	if !on {
		t.Fatal("toggle should flip F1 from off to on")
	}
}
