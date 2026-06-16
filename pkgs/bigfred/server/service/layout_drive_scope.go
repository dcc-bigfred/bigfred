package service

import (
	"context"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

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

// LesseesByTrain resolves active lessees per train on a layout.
func (s *LayoutVehicleService) LesseesByTrain(
	ctx context.Context,
	layoutID uint,
	entries []RosterTrainEntry,
) (map[uint][]domain.TrainLessee, error) {
	if entries == nil {
		var err error
		entries, err = s.ListTrains(ctx, layoutID)
		if err != nil {
			return nil, err
		}
	}
	return s.resolveLesseesByTrain(ctx, entries, time.Now().UTC())
}
