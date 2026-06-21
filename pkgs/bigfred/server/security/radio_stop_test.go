package security_test

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

func TestRadioStopCanTriggerDriveScope(t *testing.T) {
	sec := security.RadioStopSecurityContext{}
	eff := domain.NewEffectiveRoles(domain.RoleDriver)
	roster := contract.AllowedVehicles{
		Vehicles: []contract.AllowedVehicle{{
			VehicleID:         "V-1",
			ControllerUserIDs: []uint{42},
		}},
	}
	if d := sec.CanTrigger(eff, 42, roster); !d.Allowed {
		t.Fatalf("expected allow for lessee in controllerUserIds, got %q", d.Reason)
	}
	if d := sec.CanTrigger(eff, 99, roster); d.Allowed {
		t.Fatal("expected deny for unrelated driver")
	}
}

func TestRadioStopCanTriggerSignalmanIdle(t *testing.T) {
	sec := security.RadioStopSecurityContext{}
	eff := domain.NewEffectiveRoles(domain.RoleSignalman)
	if d := sec.CanTrigger(eff, 7, contract.AllowedVehicles{}); !d.Allowed {
		t.Fatalf("expected idle signalman to trigger, got %q", d.Reason)
	}
}

func TestRadioStopDeniesAdminAlone(t *testing.T) {
	sec := security.RadioStopSecurityContext{}
	eff := domain.NewEffectiveRoles(domain.RoleAdmin)
	if d := sec.CanTrigger(eff, 1, contract.AllowedVehicles{}); d.Allowed {
		t.Fatal("expected permanent admin without drive/signalman to be denied")
	}
}
