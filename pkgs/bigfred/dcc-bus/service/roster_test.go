package service_test

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
)

func TestRosterCacheApplySnapshot(t *testing.T) {
	t.Parallel()
	c := service.NewRosterCache(2)
	if !c.ApplySnapshot(contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{{
			Addr:              31,
			ControllerUserIDs: []uint{1},
		}},
	}) {
		t.Fatal("expected snapshot applied")
	}
	if !c.IsOnLayout(31) {
		t.Fatal("expected addr 31 on layout")
	}
	if c.IsOnLayout(99) {
		t.Fatal("expected addr 99 off layout")
	}
	v, ok := c.AllowedVehicle(31)
	if !ok || len(v.ControllerUserIDs) != 1 || v.ControllerUserIDs[0] != 1 {
		t.Fatalf("vehicle = %+v, ok=%v", v, ok)
	}
}

func TestRosterCacheApplySnapshotIgnoresOtherLayout(t *testing.T) {
	t.Parallel()
	c := service.NewRosterCache(2)
	if c.ApplySnapshot(contract.AllowedVehicles{LayoutID: 3, Vehicles: []contract.AllowedVehicle{{Addr: 4}}}) {
		t.Fatal("expected false for foreign layout")
	}
	if c.IsOnLayout(4) {
		t.Fatal("roster must stay empty")
	}
}

func TestRosterCacheDiffRemoved(t *testing.T) {
	t.Parallel()
	c := service.NewRosterCache(2)
	c.ApplySnapshot(contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{
			{Addr: 31},
			{Addr: 42},
		},
	})

	removed := c.DiffRemoved(contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{{Addr: 42}},
	})
	if len(removed) != 1 || removed[0] != 31 {
		t.Fatalf("removed = %v, want [31]", removed)
	}

	if diff := c.DiffRemoved(contract.AllowedVehicles{LayoutID: 3, Vehicles: []contract.AllowedVehicle{{Addr: 99}}}); diff != nil {
		t.Fatalf("foreign layout diff = %v, want nil", diff)
	}
}
