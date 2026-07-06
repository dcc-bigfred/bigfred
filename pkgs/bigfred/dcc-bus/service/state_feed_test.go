package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/state"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type stubSubs struct {
	subscribed map[uint16]bool
}

func (s *stubSubs) SubscribedAddrs() []uint16 {
	out := make([]uint16, 0, len(s.subscribed))
	for a := range s.subscribed {
		out = append(out, a)
	}
	return out
}

func (s *stubSubs) IsSubscribed(addr uint16) bool {
	return s.subscribed[addr]
}

type captureHub struct {
	calls int
	last  contract.LocoStateWire
}

func (h *captureHub) Broadcast(_ context.Context, _ uint16, env contract.EnvelopeWire) {
	h.calls++
	_ = json.Unmarshal(env.Payload, &h.last)
}

func TestApplyObservationSkipsRedisWhenNoSubscribers(t *testing.T) {
	hub := &captureHub{}
	fnCache := NewFunctionsCache()
	roster := NewRosterCache(1)
	roster.ApplySnapshot(contract.AllowedVehicles{
		LayoutID: 1,
		Vehicles: []contract.AllowedVehicle{{Addr: 10}},
	})

	deps := FeedDeps{
		Roster:  roster,
		Hub:     hub,
		HubSubs: &stubSubs{subscribed: map[uint16]bool{}},
		FnCache: fnCache,
		Store:   state.NewLocoStateStore(nil, time.Minute, nil),
	}

	applyObservation(context.Background(), deps, commandstation.LocoObservation{
		Addr:       10,
		HasSpeed:   true,
		Speed:      5,
		HasForward: true,
		Forward:    true,
	}, "external")

	if hub.calls != 0 {
		t.Fatalf("broadcasts = %d, want 0 without subscribers", hub.calls)
	}

	applyObservation(context.Background(), deps, commandstation.LocoObservation{
		Addr:         10,
		FunctionMask: 1 << 0,
		FunctionBits: 1 << 0,
	}, "external")
	if !fnCache.Matches(10, 0, true) {
		t.Fatal("FnCache F0 should be true after observation without subscribers")
	}
}

func TestApplyObservationBroadcastsWhenSubscribed(t *testing.T) {
	hub := &captureHub{}
	roster := NewRosterCache(1)
	roster.ApplySnapshot(contract.AllowedVehicles{
		LayoutID: 1,
		Vehicles: []contract.AllowedVehicle{{Addr: 10}},
	})

	deps := FeedDeps{
		Roster:  roster,
		Hub:     hub,
		HubSubs: &stubSubs{subscribed: map[uint16]bool{10: true}},
		Store:   state.NewLocoStateStore(nil, time.Minute, nil),
	}

	applyObservation(context.Background(), deps, commandstation.LocoObservation{
		Addr:       10,
		HasSpeed:   true,
		Speed:      8,
		HasForward: true,
		Forward:    true,
	}, "external")

	if hub.calls != 1 {
		t.Fatalf("broadcasts = %d, want 1", hub.calls)
	}
	if hub.last.Speed != 8 {
		t.Fatalf("speed = %d, want 8", hub.last.Speed)
	}
}
