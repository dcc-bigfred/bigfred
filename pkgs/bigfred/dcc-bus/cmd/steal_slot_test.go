package cmd

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	buserrors "github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/security"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type stealRecordingStation struct {
	slotStubStation
	stealErr error
	stolen   []uint16
}

func (s *stealRecordingStation) StealSlot(addr commandstation.LocoAddr) error {
	if s.stealErr != nil {
		return s.stealErr
	}
	s.mu.Lock()
	s.stolen = append(s.stolen, uint16(addr))
	s.mu.Unlock()
	return nil
}

func TestHandleStealSlot_claimsWhenAllowed(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	st := &stealRecordingStation{}
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

	res := r.HandleStealSlot(context.Background(), Actor{UserID: 1, SessionID: "s1"}, &recordingResponder{}, protocol.LocoStealSlotPayload{Address: 42}, "")
	if !res.OK {
		t.Fatalf("HandleStealSlot: %s", res.Code)
	}
	st.mu.Lock()
	n := len(st.stolen)
	st.mu.Unlock()
	if n != 1 || st.stolen[0] != 42 {
		t.Fatalf("stolen = %v, want [42]", st.stolen)
	}
}

func TestHandleStealSlot_rejectsUnauthorized(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	st := &stealRecordingStation{}
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

	res := r.HandleStealSlot(context.Background(), Actor{UserID: 99, SessionID: "s1"}, &recordingResponder{}, protocol.LocoStealSlotPayload{Address: 42}, "")
	if res.OK || res.Code != security.ReasonNotAuthorized {
		t.Fatalf("got %+v, want not authorized", res)
	}
	st.mu.Lock()
	n := len(st.stolen)
	st.mu.Unlock()
	if n != 0 {
		t.Fatalf("must not steal when unauthorized, stolen=%v", st.stolen)
	}
}

func TestHandleStealSlot_mapsDriverErrors(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	st := &stealRecordingStation{stealErr: commandstation.ErrNoFreeSlot}
	r, err := NewRouter(context.Background(), Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 1,
			Vehicles: []contract.AllowedVehicle{{Addr: 7, ControllerUserIDs: []uint{2}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	res := r.HandleStealSlot(context.Background(), Actor{UserID: 2, SessionID: "s2"}, &recordingResponder{}, protocol.LocoStealSlotPayload{Address: 7}, "")
	if res.OK || res.Code != buserrors.CodeNoFreeSlot {
		t.Fatalf("got %+v, want %s", res, buserrors.CodeNoFreeSlot)
	}
}

func TestHandleStealSlot_unsupportedStation(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	r, err := NewRouter(context.Background(), Config{
		Station:          &commandstation.StubStation{},
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         1,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 1,
			Vehicles: []contract.AllowedVehicle{{Addr: 3, ControllerUserIDs: []uint{1}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	res := r.HandleStealSlot(context.Background(), Actor{UserID: 1, SessionID: "s1"}, &recordingResponder{}, protocol.LocoStealSlotPayload{Address: 3}, "")
	if res.OK || res.Code != buserrors.CodeCommandStationError {
		t.Fatalf("got %+v, want command_station_error", res)
	}
}
