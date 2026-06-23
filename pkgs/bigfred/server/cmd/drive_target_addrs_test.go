package cmd

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

type stubDriveTargetRoster struct {
	vehicles []RosterVehicleEntry
	trains   []RosterTrainEntry
}

func (s stubDriveTargetRoster) ListVehicles(context.Context, uint) ([]RosterVehicleEntry, error) {
	return s.vehicles, nil
}

func (s stubDriveTargetRoster) ListTrains(context.Context, uint) ([]RosterTrainEntry, error) {
	return s.trains, nil
}

func TestResolveDriveTargetAddrs_vehicle(t *testing.T) {
	t.Parallel()
	addr := uint16(42)
	vehicleID, err := domain.NewVehicleID()
	if err != nil {
		t.Fatal(err)
	}
	roster := stubDriveTargetRoster{
		vehicles: []RosterVehicleEntry{{
			Vehicle: domain.Vehicle{ID: vehicleID, DCCAddress: ptrUint16(addr)},
		}},
	}
	addrs, err := ResolveDriveTargetAddrs(
		context.Background(),
		roster,
		1,
		domain.TakeoverTargetVehicle,
		vehicleID.String(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 1 || addrs[0] != addr {
		t.Fatalf("addrs = %v, want [%d]", addrs, addr)
	}
}

func TestResolveDriveTargetAddrs_train(t *testing.T) {
	t.Parallel()
	addr := uint16(7)
	vehicleID, err := domain.NewVehicleID()
	if err != nil {
		t.Fatal(err)
	}
	trainID, err := domain.NewTrainID()
	if err != nil {
		t.Fatal(err)
	}
	roster := stubDriveTargetRoster{
		vehicles: []RosterVehicleEntry{{
			Vehicle: domain.Vehicle{ID: vehicleID, DCCAddress: ptrUint16(addr)},
		}},
		trains: []RosterTrainEntry{{
			Train:   domain.Train{ID: trainID},
			Members: []domain.TrainMember{{VehicleID: vehicleID, Position: 1}},
		}},
	}
	addrs, err := ResolveDriveTargetAddrs(
		context.Background(),
		roster,
		1,
		domain.TakeoverTargetTrain,
		trainID.String(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 1 || addrs[0] != addr {
		t.Fatalf("addrs = %v, want [%d]", addrs, addr)
	}
}

func ptrUint16(v uint16) *uint16 { return &v }
