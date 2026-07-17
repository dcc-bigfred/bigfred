package cmd

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/slotlease"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/state"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// observingSlotStubStation simulates LocoNet SendFn acquiring a slot and
// notifying the leaser via OnSlotInUse (F0–F8 only).
type observingSlotStubStation struct {
	slotStubStation
	obs commandstation.SlotObserver
}

func (s *observingSlotStubStation) SetSlotObserver(obs commandstation.SlotObserver) {
	s.obs = obs
}

func (s *observingSlotStubStation) SendFn(mode commandstation.Mode, addr commandstation.LocoAddr, num commandstation.FuncNum, toggle bool) error {
	if s.SendFnFn != nil {
		return s.SendFnFn(mode, addr, num, toggle)
	}
	if s.obs != nil && num <= 8 {
		s.obs.OnSlotInUse(addr)
	}
	return nil
}

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

func TestHandleSetFunction_reservesWSLeaseBeforeSendFn(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	st := &observingSlotStubStation{}
	r, err := NewRouter(context.Background(), Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 1,
			Vehicles: []contract.AllowedVehicle{{Addr: 88, ControllerUserIDs: []uint{2}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	actor := Actor{UserID: 2, SessionID: "ws-tab-1"}
	res := r.HandleSetFunction(context.Background(), actor, &recordingResponder{}, contract.LocoSetFunctionWire{
		Address:  88,
		Function: 0,
		On:       true,
	}, "")
	if !res.OK {
		t.Fatalf("HandleSetFunction: %s", res.Code)
	}

	lease := leaseForAddr(t, r.SlotLeaser(), 88)
	if len(lease.Holders) != 1 {
		t.Fatalf("holders = %d, want 1", len(lease.Holders))
	}
	if lease.Holders[0].UserID != 2 || lease.Holders[0].Source != "ws" {
		t.Fatalf("holder = %+v, want userID=2 source=ws", lease.Holders[0])
	}
}

func TestHandleSetFunction_afterAdminRelease_staysWSNotExternal(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	st := &observingSlotStubStation{}
	r, err := NewRouter(context.Background(), Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 1,
			Vehicles: []contract.AllowedVehicle{{Addr: 99, ControllerUserIDs: []uint{3}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	actor := Actor{UserID: 3, SessionID: "ws-tab-2"}
	payload := contract.LocoSetFunctionWire{Address: 99, Function: 1, On: true}
	if res := r.HandleSetFunction(context.Background(), actor, &recordingResponder{}, payload, ""); !res.OK {
		t.Fatalf("first HandleSetFunction: %s", res.Code)
	}
	if !r.SlotLeaser().ForceRelease(99) {
		t.Fatal("ForceRelease returned false")
	}

	if res := r.HandleSetFunction(context.Background(), actor, &recordingResponder{}, payload, ""); !res.OK {
		t.Fatalf("second HandleSetFunction: %s", res.Code)
	}

	lease := leaseForAddr(t, r.SlotLeaser(), 99)
	if len(lease.Holders) != 1 {
		t.Fatalf("holders = %d, want 1", len(lease.Holders))
	}
	h := lease.Holders[0]
	if h.UserID != 3 || h.Source != "ws" {
		t.Fatalf("holder = %+v, want userID=3 source=ws (not external)", h)
	}
}

func TestHandleSetFunction_mapsSlotInUseLikeSetSpeed(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	st := &observingSlotStubStation{}
	st.SendFnFn = func(commandstation.Mode, commandstation.LocoAddr, commandstation.FuncNum, bool) error {
		return commandstation.ErrSlotInUse
	}
	r, err := NewRouter(context.Background(), Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 1,
			Vehicles: []contract.AllowedVehicle{{Addr: 55, ControllerUserIDs: []uint{1}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	res := r.HandleSetFunction(context.Background(), Actor{UserID: 1, SessionID: "ws-1"}, &recordingResponder{}, contract.LocoSetFunctionWire{
		Address:  55,
		Function: 0,
		On:       true,
	}, "")
	if res.OK {
		t.Fatal("expected failure when slot is in use")
	}
	if res.Code != errors.CodeSlotInUse {
		t.Fatalf("code = %q, want %q (not command_station_error)", res.Code, errors.CodeSlotInUse)
	}
}

func leaseForAddr(t *testing.T, leaser *slotlease.Leaser, addr uint16) slotlease.LeaseInfo {
	t.Helper()
	snap := leaser.DiagnosticSnapshot()
	for _, le := range snap.Leases {
		if le.Addr == addr {
			return le
		}
	}
	t.Fatalf("expected lease for addr %d", addr)
	return slotlease.LeaseInfo{}
}
