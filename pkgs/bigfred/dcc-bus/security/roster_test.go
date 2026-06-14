package security

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

func TestRosterGateCanDrive(t *testing.T) {
	t.Parallel()
	g := NewRosterGate(2)
	g.ApplySnapshot(contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{{
			Addr:              31,
			ControllerUserIDs: []uint{1},
		}},
	})

	if !g.IsLocoAllowedOnLayout(31) {
		t.Fatal("expected addr 31 on layout")
	}
	if g.IsLocoAllowedOnLayout(99) {
		t.Fatal("expected addr 99 off layout")
	}
	if !g.UserCanControlLoco(1, 31) {
		t.Fatal("expected user 1 may drive 31")
	}
	if g.UserCanControlLoco(2, 31) {
		t.Fatal("expected user 2 denied")
	}
	if reason := g.DenyDriveCommand(1, 31); reason != "" {
		t.Fatalf("expected allow, got %q", reason)
	}
	if reason := g.DenyDriveCommand(2, 31); reason != ReasonNotAuthorized {
		t.Fatalf("reason = %q, want %s", reason, ReasonNotAuthorized)
	}
	if reason := g.DenyDriveCommand(1, 99); reason != ReasonVehicleNotOnLayout {
		t.Fatalf("reason = %q, want %s", reason, ReasonVehicleNotOnLayout)
	}
}

func TestRosterGateApplySnapshotIgnoresOtherLayout(t *testing.T) {
	t.Parallel()
	g := NewRosterGate(2)
	if g.ApplySnapshot(contract.AllowedVehicles{LayoutID: 3, Vehicles: []contract.AllowedVehicle{{Addr: 4}}}) {
		t.Fatal("expected false for foreign layout")
	}
	if g.IsLocoAllowedOnLayout(4) {
		t.Fatal("roster must stay empty")
	}
}

func TestRosterGateDiffRemoved(t *testing.T) {
	t.Parallel()
	g := NewRosterGate(2)
	g.ApplySnapshot(contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{
			{Addr: 31},
			{Addr: 42},
		},
	})

	removed := g.DiffRemoved(contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{{Addr: 42}},
	})
	if len(removed) != 1 || removed[0] != 31 {
		t.Fatalf("removed = %v, want [31]", removed)
	}

	if diff := g.DiffRemoved(contract.AllowedVehicles{LayoutID: 3, Vehicles: []contract.AllowedVehicle{{Addr: 99}}}); diff != nil {
		t.Fatalf("foreign layout diff = %v, want nil", diff)
	}
}
