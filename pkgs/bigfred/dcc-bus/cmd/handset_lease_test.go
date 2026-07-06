package cmd

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
)

func TestPrepareHandsetLease_acquiresOnFirstDrive(t *testing.T) {
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
			Vehicles: []contract.AllowedVehicle{{Addr: 55, ControllerUserIDs: []uint{3}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	actor := remotes.ThrottleActor{
		UserID:    3,
		SessionID: remotes.HandsetSessionID("z21-1"),
		Source:    "z21",
	}
	if res := r.prepareHandsetLease(actor, 55); !res.OK {
		t.Fatalf("prepareHandsetLease: %s", res.Code)
	}
	if got := st.acquiredAddrs(); len(got) != 1 || got[0] != 55 {
		t.Fatalf("acquired = %v, want [55]", got)
	}
}

func TestSetSpeedFromHandset_acquiresBeforeDrive(t *testing.T) {
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
			Vehicles: []contract.AllowedVehicle{{Addr: 66, ControllerUserIDs: []uint{5}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	actor := remotes.ThrottleActor{
		UserID:    5,
		SessionID: remotes.HandsetSessionID("wt-1"),
		Source:    "withrottle",
	}
	result := r.SetSpeed(context.Background(), actor, &recordingResponder{}, contract.LocoSetSpeedWire{
		Address: 66,
		Speed:   10,
		Forward: true,
	})
	if !result.OK {
		t.Fatalf("SetSpeed: %s", result.Code)
	}
	if got := st.acquiredAddrs(); len(got) != 1 || got[0] != 66 {
		t.Fatalf("acquired = %v, want [66]", got)
	}
}
