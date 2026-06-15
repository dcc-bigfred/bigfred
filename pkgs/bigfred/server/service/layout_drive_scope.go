package service

import (
	"context"
	"time"
)

// UserCanDriveVehicle reports whether userID may drive vehicleID on
// layoutID, respecting active leases (§4.3 drive-scope projection).
func (s *LayoutVehicleService) UserCanDriveVehicle(
	ctx context.Context,
	layoutID uint,
	userID uint,
	vehicleID uint,
) (bool, error) {
	entries, err := s.ListVehicles(ctx, layoutID)
	if err != nil {
		return false, err
	}
	var vehicleOwner uint
	found := false
	for _, e := range entries {
		if e.Vehicle.ID == vehicleID {
			vehicleOwner = e.Vehicle.OwnerUserID
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}

	trains, err := s.ListTrains(ctx, layoutID)
	if err != nil {
		return false, err
	}
	lesseesByVehicle, err := s.resolveLesseesByVehicle(ctx, entries, trains, time.Now().UTC())
	if err != nil {
		return false, err
	}
	return userCanDriveWithLessees(userID, vehicleOwner, lesseesByVehicle[vehicleID]), nil
}

func userCanDriveWithLessees(userID, ownerID uint, lessees []uint) bool {
	if userID == ownerID {
		return len(lessees) == 0
	}
	for _, l := range lessees {
		if l == userID {
			return true
		}
	}
	return false
}

// UserCanDriveWithLessees is exported for HTTP layer drive-scope checks.
func UserCanDriveWithLessees(userID, ownerID uint, lessees []uint) bool {
	return userCanDriveWithLessees(userID, ownerID, lessees)
}

// LesseesByVehicle resolves active lessees per vehicle on a layout.
func (s *LayoutVehicleService) LesseesByVehicle(
	ctx context.Context,
	layoutID uint,
	entries []RosterVehicleEntry,
	trains []RosterTrainEntry,
) (map[uint][]uint, error) {
	if entries == nil {
		var err error
		entries, err = s.ListVehicles(ctx, layoutID)
		if err != nil {
			return nil, err
		}
	}
	if trains == nil {
		var err error
		trains, err = s.ListTrains(ctx, layoutID)
		if err != nil {
			return nil, err
		}
	}
	return s.resolveLesseesByVehicle(ctx, entries, trains, time.Now().UTC())
}
