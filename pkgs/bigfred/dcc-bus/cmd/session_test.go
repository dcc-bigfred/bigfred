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
	released []uint16
}

func (s *slotStubStation) AcquireSlot(commandstation.LocoAddr) error { return nil }
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

type stubHub struct {
	sessions []SessionView
}

func (h *stubHub) Broadcast(context.Context, uint16, contract.EnvelopeWire) {}
func (h *stubHub) SubscribedAddrs() []uint16                               { return nil }
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

func TestHandleSessionClose_releasesSlotsWhenNoSessionsRemain(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	ctx := context.Background()
	const addr uint16 = 7

	// The closing user was driving addr.
	if err := rs.StoreLocoCurrentState(ctx, contract.LocoStateWire{
		Address:            addr,
		ControlledByUserID: 42,
	}, StateTTL); err != nil {
		t.Fatal(err)
	}

	st := &slotStubStation{}
	r, err := NewRouter(ctx, Config{
		Station:          st,
		Hub:              &stubHub{}, // no remaining sessions
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

	r.HandleSessionClose(ctx, Actor{UserID: 42, SessionID: "last-tab"}, "ws_closed")

	got := waitForRelease(t, st, 1)
	if len(got) != 1 || got[0] != addr {
		t.Fatalf("released = %v, want [%d]", got, addr)
	}
}

func TestHandleSessionClose_keepsSlotWhenAnotherSessionSubscribed(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	ctx := context.Background()
	const addr uint16 = 7

	if err := rs.StoreLocoCurrentState(ctx, contract.LocoStateWire{
		Address:            addr,
		ControlledByUserID: 42,
	}, StateTTL); err != nil {
		t.Fatal(err)
	}

	st := &slotStubStation{}
	// A different user's session is still subscribed to addr.
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
			Vehicles: []contract.AllowedVehicle{{Addr: addr}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	r.HandleSessionClose(ctx, Actor{UserID: 42, SessionID: "last-tab"}, "ws_closed")

	// Give the async release a chance to (incorrectly) fire.
	time.Sleep(100 * time.Millisecond)
	if got := st.releasedAddrs(); len(got) != 0 {
		t.Fatalf("released = %v, want none (co-driver still subscribed)", got)
	}
}
