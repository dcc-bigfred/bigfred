package cmd

import (
	"context"
	"sync"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type speedCall struct {
	addr    uint16
	speed   uint8
	forward bool
}

type speedRecordingStation struct {
	commandstation.StubStation
	mu    sync.Mutex
	calls []speedCall
}

func (s *speedRecordingStation) SetSpeed(addr commandstation.LocoAddr, speed uint8, forward bool, _ uint8) error {
	s.mu.Lock()
	s.calls = append(s.calls, speedCall{uint16(addr), speed, forward})
	s.mu.Unlock()
	return nil
}

func (s *speedRecordingStation) snapshot() []speedCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]speedCall(nil), s.calls...)
}

func newSingleVehicleTestRouter(t *testing.T, enabled bool, vehicles []contract.AllowedVehicle) (*Router, *speedRecordingStation) {
	t.Helper()
	rs, cleanup := testRedis(t)
	t.Cleanup(cleanup)
	st := &speedRecordingStation{}
	r, err := NewRouter(context.Background(), Config{
		Station:              st,
		Hub:                  &stubHub{},
		Redis:                rs,
		LayoutID:             1,
		CommandStationID:     1,
		SpeedSteps:           128,
		SingleVehicleControl: enabled,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 1,
			Vehicles: vehicles,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r, st
}

func seedMovingVehicle(r *Router, addr uint16, userID uint, speed uint8) {
	r.store.SetSpeed(addr, speed, true, userID, "test")
}

func TestSingleVehicleControl_disabled_doesNotBrake(t *testing.T) {
	t.Parallel()
	r, st := newSingleVehicleTestRouter(t, false, []contract.AllowedVehicle{
		{Addr: 10, ControllerUserIDs: []uint{1}},
		{Addr: 20, ControllerUserIDs: []uint{1}},
	})
	seedMovingVehicle(r, 20, 1, 15)

	actor := Actor{UserID: 1, SessionID: "s1"}
	res := r.HandleSetSpeed(context.Background(), actor, &recordingResponder{}, contract.LocoSetSpeedWire{
		Address: 10,
		Speed:   20,
		Forward: true,
	}, "")
	if !res.OK {
		t.Fatalf("SetSpeed: %s", res.Code)
	}
	for _, c := range st.snapshot() {
		if c.addr == 20 && c.speed == 0 {
			t.Fatalf("disabled policy must not brake other vehicle: %+v", st.snapshot())
		}
	}
}

func TestSingleVehicleControl_setSpeed_brakesOtherMovingVehicle(t *testing.T) {
	t.Parallel()
	r, st := newSingleVehicleTestRouter(t, true, []contract.AllowedVehicle{
		{Addr: 10, ControllerUserIDs: []uint{1}},
		{Addr: 20, ControllerUserIDs: []uint{1}},
	})
	seedMovingVehicle(r, 20, 1, 15)

	actor := Actor{UserID: 1, SessionID: "s1"}
	res := r.HandleSetSpeed(context.Background(), actor, &recordingResponder{}, contract.LocoSetSpeedWire{
		Address: 10,
		Speed:   20,
		Forward: true,
	}, "")
	if !res.OK {
		t.Fatalf("SetSpeed: %s", res.Code)
	}
	if snap := r.store.Snapshot(20); snap.Speed != 0 {
		t.Fatalf("other vehicle speed = %d, want 0", snap.Speed)
	}
	var braked bool
	for _, c := range st.snapshot() {
		if c.addr == 20 && c.speed == 0 {
			braked = true
		}
	}
	if !braked {
		t.Fatalf("expected stop command for addr 20, got %+v", st.snapshot())
	}
}

func TestSingleVehicleControl_setSpeed_skipsWhenSpeedNotMoving(t *testing.T) {
	t.Parallel()
	r, st := newSingleVehicleTestRouter(t, true, []contract.AllowedVehicle{
		{Addr: 10, ControllerUserIDs: []uint{1}},
		{Addr: 20, ControllerUserIDs: []uint{1}},
	})
	seedMovingVehicle(r, 20, 1, 1)

	actor := Actor{UserID: 1, SessionID: "s1"}
	res := r.HandleSetSpeed(context.Background(), actor, &recordingResponder{}, contract.LocoSetSpeedWire{
		Address: 10,
		Speed:   20,
		Forward: true,
	}, "")
	if !res.OK {
		t.Fatalf("SetSpeed: %s", res.Code)
	}
	for _, c := range st.snapshot() {
		if c.addr == 20 {
			t.Fatalf("vehicle at speed 1 must not be braked: %+v", st.snapshot())
		}
	}
}

func TestSingleVehicleControl_select_brakesOtherMovingVehicle(t *testing.T) {
	t.Parallel()
	r, st := newSingleVehicleTestRouter(t, true, []contract.AllowedVehicle{
		{Addr: 10, ControllerUserIDs: []uint{1}},
		{Addr: 20, ControllerUserIDs: []uint{1}},
	})
	seedMovingVehicle(r, 20, 1, 12)

	actor := Actor{UserID: 1, SessionID: "s1"}
	res := r.HandleLocoSelect(context.Background(), actor, &recordingResponder{}, protocol.LocoSelectPayload{Address: 10}, "")
	if !res.OK {
		t.Fatalf("select: %s", res.Code)
	}
	if snap := r.store.Snapshot(20); snap.Speed != 0 {
		t.Fatalf("other vehicle speed = %d, want 0", snap.Speed)
	}
	var braked bool
	for _, c := range st.snapshot() {
		if c.addr == 20 && c.speed == 0 {
			braked = true
		}
	}
	if !braked {
		t.Fatalf("expected stop command for addr 20, got %+v", st.snapshot())
	}
}

func TestSingleVehicleControl_skipsLentOutVehicle(t *testing.T) {
	t.Parallel()
	r, st := newSingleVehicleTestRouter(t, true, []contract.AllowedVehicle{
		{Addr: 10, ControllerUserIDs: []uint{1}},
		{Addr: 20, ControllerUserIDs: []uint{2}}, // lent out: lessee drives
	})
	seedMovingVehicle(r, 20, 2, 18)

	actor := Actor{UserID: 1, SessionID: "s1"}
	res := r.HandleSetSpeed(context.Background(), actor, &recordingResponder{}, contract.LocoSetSpeedWire{
		Address: 10,
		Speed:   20,
		Forward: true,
	}, "")
	if !res.OK {
		t.Fatalf("SetSpeed: %s", res.Code)
	}
	if snap := r.store.Snapshot(20); snap.Speed != 18 {
		t.Fatalf("lent-out vehicle speed = %d, want 18", snap.Speed)
	}
	for _, c := range st.snapshot() {
		if c.addr == 20 {
			t.Fatalf("lent-out vehicle must not be braked: %+v", st.snapshot())
		}
	}
}

func TestSingleVehicleControl_doesNotBrakeActiveVehicle(t *testing.T) {
	t.Parallel()
	r, _ := newSingleVehicleTestRouter(t, true, []contract.AllowedVehicle{
		{Addr: 10, ControllerUserIDs: []uint{1}},
	})
	seedMovingVehicle(r, 10, 1, 25)

	actor := Actor{UserID: 1, SessionID: "s1"}
	res := r.HandleSetSpeed(context.Background(), actor, &recordingResponder{}, contract.LocoSetSpeedWire{
		Address: 10,
		Speed:   30,
		Forward: true,
	}, "")
	if !res.OK {
		t.Fatalf("SetSpeed: %s", res.Code)
	}
	if snap := r.store.Snapshot(10); snap.Speed != 30 {
		t.Fatalf("active vehicle speed = %d, want 30", snap.Speed)
	}
}
