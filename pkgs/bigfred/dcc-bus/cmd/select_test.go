package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

type recordingResponder struct {
	selected uint16
	acks     []protocol.AckPayload
	subs     []uint16
}

func (r *recordingResponder) Subscribe(addrs ...uint16)                          { r.subs = append(r.subs, addrs...) }
func (r *recordingResponder) Unsubscribe(addrs ...uint16)                        {}
func (r *recordingResponder) SubscribedAddrs() []uint16                          { return r.subs }
func (r *recordingResponder) OldestSubscribed() (uint16, bool)                   { return 0, false }
func (r *recordingResponder) SelectedAddr() uint16                         { return r.selected }
func (r *recordingResponder) SetSelected(addr uint16)                      { r.selected = addr }
func (r *recordingResponder) ClearSelected()                               { r.selected = 0 }
func (r *recordingResponder) SendLocoState(context.Context, contract.LocoStateWire) error {
	return nil
}
func (r *recordingResponder) SendLocoError(context.Context, uint16, string, string) error {
	return nil
}
func (r *recordingResponder) SendLocoErrorPayload(context.Context, protocol.LocoErrorPayload) error {
	return nil
}
func (r *recordingResponder) SendAck(_ context.Context, _ string, p protocol.AckPayload) error {
	r.acks = append(r.acks, p)
	return nil
}

func TestHandleLocoSelect_acquiresSlot(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	st := &slotStubStation{}
	r, err := NewRouter(context.Background(), Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 1,
			Vehicles: []contract.AllowedVehicle{{Addr: 42, ControllerUserIDs: []uint{1}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := &recordingResponder{}
	res := r.HandleLocoSelect(context.Background(), Actor{UserID: 1, SessionID: "s1"}, resp, protocol.LocoSelectPayload{Address: 42}, "req-1")
	if !res.OK {
		t.Fatalf("select failed: %s", res.Code)
	}
	if got := st.acquiredAddrs(); len(got) != 1 || got[0] != 42 {
		t.Fatalf("acquired = %v, want [42]", got)
	}
	if resp.selected != 42 {
		t.Fatalf("selected = %d, want 42", resp.selected)
	}
}

func TestHandleLocoSelect_switcherReleasesPrevious(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	st := &slotStubStation{}
	r, err := NewRouter(context.Background(), Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 1,
			Vehicles: []contract.AllowedVehicle{
				{Addr: 10, ControllerUserIDs: []uint{1}},
				{Addr: 20, ControllerUserIDs: []uint{1}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := &recordingResponder{}
	actor := Actor{UserID: 1, SessionID: "s1"}

	if res := r.HandleLocoSelect(context.Background(), actor, resp, protocol.LocoSelectPayload{Address: 10}, ""); !res.OK {
		t.Fatalf("first select: %s", res.Code)
	}
	if res := r.HandleLocoSelect(context.Background(), actor, resp, protocol.LocoSelectPayload{Address: 20}, ""); !res.OK {
		t.Fatalf("second select: %s", res.Code)
	}
	if resp.selected != 20 {
		t.Fatalf("selected = %d, want 20", resp.selected)
	}
	// Switcher change defers the previous slot's release by SwitcherGrace so a
	// quick A→B→A switch reuses A's slot. The previous slot is NOT released
	// immediately; only after the grace window elapses (SweepDeferred).
	if released := st.releasedAddrs(); len(released) != 0 {
		t.Fatalf("released before grace = %v, want [] (deferred)", released)
	}
	// Re-selecting 10 within the grace window must reuse the slot, not release it.
	if res := r.HandleLocoSelect(context.Background(), actor, resp, protocol.LocoSelectPayload{Address: 10}, ""); !res.OK {
		t.Fatalf("re-select 10: %s", res.Code)
	}
	if res := r.HandleLocoSelect(context.Background(), actor, resp, protocol.LocoSelectPayload{Address: 20}, ""); !res.OK {
		t.Fatalf("re-select 20: %s", res.Code)
	}
	// After the grace window elapses, the deferred release fires.
	r.leaser.SweepDeferred(time.Now().Add(2 * time.Hour))
	released := st.releasedAddrs()
	if len(released) != 1 || released[0] != 10 {
		t.Fatalf("released after grace = %v, want [10]", released)
	}
}
