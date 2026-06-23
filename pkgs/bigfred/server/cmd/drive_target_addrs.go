package cmd

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// DriveTargetRosterPort lists layout roster rows for drive-target address
// resolution (vehicle or train DCC members).
type DriveTargetRosterPort interface {
	ListVehicles(ctx context.Context, layoutID uint) ([]RosterVehicleEntry, error)
	ListTrains(ctx context.Context, layoutID uint) ([]RosterTrainEntry, error)
}

// ResolveDriveTargetAddrs returns the DCC addresses of a vehicle or every
// powered member of a train on the layout roster. Dummy vehicles (no DCC
// address) yield an empty slice without error.
func ResolveDriveTargetAddrs(
	ctx context.Context,
	roster DriveTargetRosterPort,
	layoutID uint,
	target domain.TakeoverTarget,
	targetID string,
) ([]uint16, error) {
	entries, err := roster.ListVehicles(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	switch target {
	case domain.TakeoverTargetVehicle:
		addrs, _, err := vehicleDriveAddrs(entries, domain.VehicleID(targetID))
		return addrs, err
	case domain.TakeoverTargetTrain:
		trains, err := roster.ListTrains(ctx, layoutID)
		if err != nil {
			return nil, err
		}
		addrs, _, err := trainDriveAddrs(entries, trains, domain.TrainID(targetID))
		return addrs, err
	default:
		return nil, errEStopTargetInvalidState
	}
}

func vehicleDriveAddrs(entries []RosterVehicleEntry, vehicleID domain.VehicleID) ([]uint16, uint, error) {
	for i := range entries {
		if entries[i].Vehicle.ID != vehicleID {
			continue
		}
		if entries[i].Vehicle.DCCAddress == nil {
			return nil, entries[i].Vehicle.OwnerUserID, nil
		}
		return []uint16{uint16(*entries[i].Vehicle.DCCAddress)}, entries[i].Vehicle.OwnerUserID, nil
	}
	return nil, 0, errEStopTargetNotOnLayout
}

func trainDriveAddrs(
	entries []RosterVehicleEntry,
	trains []RosterTrainEntry,
	trainID domain.TrainID,
) ([]uint16, uint, error) {
	trainEntry, err := rosterTrainEntry(trains, trainID)
	if err != nil {
		return nil, 0, err
	}
	addrByVehicle := make(map[domain.VehicleID]uint16, len(entries))
	for _, e := range entries {
		if e.Vehicle.DCCAddress != nil {
			addrByVehicle[e.Vehicle.ID] = uint16(*e.Vehicle.DCCAddress)
		}
	}
	addrs := make([]uint16, 0, len(trainEntry.Members))
	for _, m := range trainEntry.Members {
		if addr, ok := addrByVehicle[m.VehicleID]; ok {
			addrs = append(addrs, addr)
		}
	}
	return addrs, trainEntry.Train.OwnerUserID, nil
}

func rosterTrainEntry(trains []RosterTrainEntry, trainID domain.TrainID) (*RosterTrainEntry, error) {
	for i := range trains {
		if trains[i].Train.ID == trainID {
			return &trains[i], nil
		}
	}
	return nil, errEStopTargetNotOnLayout
}
