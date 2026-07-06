package cmd

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// slotStubStation is a Station that also implements SlotManager, recording
// ReleaseSlot calls so session-close slot cleanup can be asserted.
type slotStubStation struct {
	commandstation.StubStation
	mu       sync.Mutex
	acquired []uint16
	released []uint16
}

func (s *slotStubStation) AcquireSlot(addr commandstation.LocoAddr) error {
	s.mu.Lock()
	s.acquired = append(s.acquired, uint16(addr))
	s.mu.Unlock()
	return nil
}
func (s *slotStubStation) ForceAcquireSlot(commandstation.LocoAddr) error { return nil }
func (s *slotStubStation) DispatchSlot(commandstation.LocoAddr) error { return nil }
func (s *slotStubStation) AcquireDispatched() (commandstation.LocoAddr, error) {
	return 0, nil
}
func (s *slotStubStation) ReleaseSlot(addr commandstation.LocoAddr) error {
	s.mu.Lock()
	s.released = append(s.released, uint16(addr))
	s.mu.Unlock()
	return nil
}
func (s *slotStubStation) releasedAddrs() []uint16 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]uint16(nil), s.released...)
}
func (s *slotStubStation) acquiredAddrs() []uint16 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]uint16(nil), s.acquired...)
}

type stubHub struct {
	sessions []SessionView
}

func (h *stubHub) Broadcast(context.Context, uint16, contract.EnvelopeWire) {}
func (h *stubHub) SubscribedAddrs() []uint16                               { return nil }
func (h *stubHub) IsSubscribed(uint16) bool                                { return false }
func (h *stubHub) UnsubscribeAll(...uint16)                                {}
func (h *stubHub) Snapshot() []SessionView                                 { return append([]SessionView(nil), h.sessions...) }
func (h *stubHub) SessionsForUser(userID uint) []SessionView {
	out := make([]SessionView, 0, len(h.sessions))
	for _, s := range h.sessions {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out
}

func TestIsLastSessionForUser_reconnectSkipsDMS(t *testing.T) {
	t.Parallel()
	r := &Router{hub: &stubHub{}}
	old := Actor{UserID: 42, SessionID: "old-tab"}

	if !r.isLastSessionForUser(old) {
		t.Fatal("no live sessions: closing tab should be last for user")
	}

	r.hub = &stubHub{sessions: []SessionView{
		{ID: "new-tab", UserID: 42},
	}}
	if r.isLastSessionForUser(old) {
		t.Fatal("reconnected tab: old session close must not trigger layout-wide DMS")
	}
}

// waitForRelease polls the stub until n addresses are released or the deadline
// elapses, since releaseUnusedSlots runs asynchronously.
func waitForRelease(t *testing.T, st *slotStubStation, n int) []uint16 {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := st.releasedAddrs(); len(got) >= n {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	return st.releasedAddrs()
}

func TestHandleSessionClose_releasesLeaseOnSessionClose(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	ctx := context.Background()
	const addr uint16 = 7

	st := &slotStubStation{}
	r, err := NewRouter(ctx, Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         2,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 2,
			Vehicles: []contract.AllowedVehicle{{Addr: addr, ControllerUserIDs: []uint{42}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.leaser.Select(42, "last-tab", "ws", addr); err != nil {
		t.Fatal(err)
	}
	r.HandleSessionClose(ctx, Actor{UserID: 42, SessionID: "last-tab"}, "ws_closed")

	got := waitForRelease(t, st, 1)
	if len(got) != 1 || got[0] != addr {
		t.Fatalf("released = %v, want [%d]", got, addr)
	}
}

// Subscribing without selecting does not hold a slot; closing a view-only
// session must not release command-station slots.
func TestHandleSessionClose_subscribeOnlyDoesNotReleaseSlot(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	ctx := context.Background()
	const addr uint16 = 31

	st := &slotStubStation{}
	r, err := NewRouter(ctx, Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         2,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 2,
			Vehicles: []contract.AllowedVehicle{{Addr: addr}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	r.HandleSessionClose(ctx, Actor{
		UserID:                 42,
		SessionID:              "last-tab",
		ClosingSubscribedAddrs: []uint16{addr},
	}, "ws_closed")

	time.Sleep(100 * time.Millisecond)
	if got := st.releasedAddrs(); len(got) != 0 {
		t.Fatalf("released = %v, want none (subscribe is view-only)", got)
	}
}

func TestHandleSessionClose_releasesLeaseEvenWhenOthersSubscribed(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	ctx := context.Background()
	const addr uint16 = 7

	st := &slotStubStation{}
	hub := &stubHub{sessions: []SessionView{
		{ID: "other-user-tab", UserID: 99, SubscribedAddrs: []uint16{addr}},
	}}
	r, err := NewRouter(ctx, Config{
		Station:          st,
		Hub:              hub,
		Redis:            rs,
		LayoutID:         2,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 2,
			Vehicles: []contract.AllowedVehicle{{Addr: addr, ControllerUserIDs: []uint{42}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.leaser.Select(42, "last-tab", "ws", addr); err != nil {
		t.Fatal(err)
	}
	r.HandleSessionClose(ctx, Actor{UserID: 42, SessionID: "last-tab"}, "ws_closed")

	got := waitForRelease(t, st, 1)
	if len(got) != 1 || got[0] != addr {
		t.Fatalf("released = %v, want [%d] (viewers do not hold slots)", got, addr)
	}
}
