package security

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

func TestRadioStopSecurityContextCanTrigger(t *testing.T) {
	sec := RadioStopSecurityContext{}
	roster := contract.AllowedVehicles{
		Vehicles: []contract.AllowedVehicle{
			{Addr: 3, ControllerUserIDs: []uint{10}},
			{Addr: 7, ControllerUserIDs: []uint{20, 30}},
		},
	}

	if d := sec.CanTrigger(10, roster); !d.Allowed {
		t.Fatalf("owner 10 should trigger: %s", d.Reason)
	}
	if d := sec.CanTrigger(30, roster); !d.Allowed {
		t.Fatalf("co-driver 30 should trigger: %s", d.Reason)
	}
	if d := sec.CanTrigger(99, roster); d.Allowed {
		t.Fatal("unrelated user should be denied")
	}
	if d := sec.CanTrigger(99, roster); d.Reason != "not_authorized_to_drive" {
		t.Fatalf("reason = %q", d.Reason)
	}
}
