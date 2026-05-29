package layoutroster

import (
	"testing"
)

func TestAllowedVehiclesRoundTrip(t *testing.T) {
	in := AllowedVehicles{
		LayoutID:  7,
		UpdatedAt: 1,
		Vehicles: []AllowedVehicle{{
			VehicleID:         10,
			Addr:              3,
			OwnerUserID:       5,
			ControllerUserIDs: []uint{5},
		}},
	}
	raw, err := Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := UnmarshalAllowedVehicles(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Vehicles) != 1 || out.Vehicles[0].Addr != 3 {
		t.Fatalf("got %+v", out)
	}
}

func TestDefinedTrainsRoundTrip(t *testing.T) {
	addr := uint16(42)
	in := DefinedTrains{
		LayoutID: 1,
		Trains: []DefinedTrain{{
			TrainID:     2,
			OwnerUserID: 3,
			Members: []DefinedTrainMember{{
				VehicleID: 9,
				Position:  0,
				Addr:      &addr,
			}},
		}},
	}
	raw, err := Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := UnmarshalDefinedTrains(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Trains) != 1 || out.Trains[0].Members[0].Addr == nil || *out.Trains[0].Members[0].Addr != 42 {
		t.Fatalf("got %+v", out)
	}
}
