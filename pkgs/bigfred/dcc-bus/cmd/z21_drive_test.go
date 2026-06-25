package cmd

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/security"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
)

func TestAuthorizeZ21Drive(t *testing.T) {
	t.Parallel()
	roster := service.NewRosterCache(1)
	roster.ApplySnapshot(contract.AllowedVehicles{
		LayoutID: 1,
		Vehicles: []contract.AllowedVehicle{{
			VehicleID:         "V-1",
			Addr:              3,
			OwnerUserID:       5,
			ControllerUserIDs: []uint{5, 9},
		}},
	})
	r := &Router{
		layoutID: 1,
		roster:   roster,
		drive:    security.DrivePolicy{},
	}

	if !r.AuthorizeZ21Drive(9, 3, nil, true) {
		t.Fatal("expected allowAll for lessee")
	}
	if r.AuthorizeZ21Drive(9, 3, []uint16{10}, false) {
		t.Fatal("expected fixed list reject")
	}
	if !r.AuthorizeZ21Drive(9, 3, []uint16{3}, false) {
		t.Fatal("expected fixed list allow")
	}
	if r.AuthorizeZ21Drive(99, 3, nil, true) {
		t.Fatal("expected unauthorized user reject")
	}
}
