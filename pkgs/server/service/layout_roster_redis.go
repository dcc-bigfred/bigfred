package service

import (
	"context"

	"github.com/keskad/loco/pkgs/layoutroster"
	"github.com/keskad/loco/pkgs/server/domain"
)

// layoutRosterPublisher pushes full roster snapshots to Redis for dcc-bus.
type layoutRosterPublisher interface {
	PublishLayoutAllowedVehicles(ctx context.Context, snap layoutroster.AllowedVehicles) error
	PublishLayoutDefinedTrains(ctx context.Context, snap layoutroster.DefinedTrains) error
}

// SyncLayoutRosterForTrain republishes roster snapshots on every
// layout that has trainID on its roster. Called when the train
// catalogue changes (members, name) without a layout roster mutation.
func (s *LayoutVehicleService) SyncLayoutRosterForTrain(ctx context.Context, trainID uint) error {
	if s.redis == nil || trainID == 0 {
		return nil
	}
	rows, err := s.layoutTrains.ListByTrain(ctx, trainID)
	if err != nil {
		return err
	}
	seen := make(map[uint]struct{}, len(rows))
	for _, r := range rows {
		if _, ok := seen[r.LayoutID]; ok {
			continue
		}
		seen[r.LayoutID] = struct{}{}
		if err := s.SyncLayoutRosterToRedis(ctx, r.LayoutID); err != nil {
			return err
		}
	}
	return nil
}

// syncRosterForVehicleInTrains refreshes defined_trains (and
// allowed_vehicles) on layouts whose roster trains include vehicleID.
// Covers DCC-address edits on vehicles that are train members only.
func (s *LayoutVehicleService) syncRosterForVehicleInTrains(ctx context.Context, vehicleID uint) error {
	if s.redis == nil || vehicleID == 0 {
		return nil
	}
	memberRows, err := s.members.ListByVehicle(ctx, vehicleID)
	if err != nil {
		return err
	}
	seenTrain := make(map[uint]struct{}, len(memberRows))
	for _, m := range memberRows {
		if _, ok := seenTrain[m.TrainID]; ok {
			continue
		}
		seenTrain[m.TrainID] = struct{}{}
		if err := s.SyncLayoutRosterForTrain(ctx, m.TrainID); err != nil {
			return err
		}
	}
	return nil
}

// SyncLayoutRosterToRedis rebuilds and publishes both layout roster
// snapshots. Called after roster mutations and during server bootstrap.
func (s *LayoutVehicleService) SyncLayoutRosterToRedis(ctx context.Context, layoutID uint) error {
	if s.redis == nil || layoutID == 0 {
		return nil
	}
	pub, ok := s.redis.(layoutRosterPublisher)
	if !ok {
		return nil
	}
	vehicles, err := s.buildAllowedVehiclesSnapshot(ctx, layoutID)
	if err != nil {
		return err
	}
	trains, err := s.buildDefinedTrainsSnapshot(ctx, layoutID)
	if err != nil {
		return err
	}
	if err := pub.PublishLayoutAllowedVehicles(ctx, vehicles); err != nil {
		return err
	}
	return pub.PublishLayoutDefinedTrains(ctx, trains)
}

// buildAllowedVehiclesSnapshot lists drivable vehicles on the layout
// roster with per-vehicle controller user ids (owner today).
func (s *LayoutVehicleService) buildAllowedVehiclesSnapshot(ctx context.Context, layoutID uint) (layoutroster.AllowedVehicles, error) {
	entries, err := s.ListVehicles(ctx, layoutID)
	if err != nil {
		return layoutroster.AllowedVehicles{}, err
	}
	out := layoutroster.AllowedVehicles{
		LayoutID:  layoutID,
		UpdatedAt: layoutroster.NowMS(),
		Vehicles:  make([]layoutroster.AllowedVehicle, 0, len(entries)),
	}
	for _, e := range entries {
		if e.Vehicle.DCCAddress == nil {
			continue
		}
		out.Vehicles = append(out.Vehicles, layoutroster.AllowedVehicle{
			VehicleID:               e.Vehicle.ID,
			Addr:                    *e.Vehicle.DCCAddress,
			OwnerUserID:             e.Vehicle.OwnerUserID,
			ControllerUserIDs:       []uint{e.Vehicle.OwnerUserID},
			Rp1Function:             e.Vehicle.Rp1Function,
			EmergencyLightsFunction: e.Vehicle.EmergencyLightsFunction,
			DeadManSwitchOption:     string(e.Vehicle.DeadManSwitchOption),
		})
	}
	return out, nil
}

// buildDefinedTrainsSnapshot lists layout trains with member DCC
// addresses hydrated from the vehicle catalogue.
func (s *LayoutVehicleService) buildDefinedTrainsSnapshot(ctx context.Context, layoutID uint) (layoutroster.DefinedTrains, error) {
	entries, err := s.ListTrains(ctx, layoutID)
	if err != nil {
		return layoutroster.DefinedTrains{}, err
	}
	vehicleIDs := make([]uint, 0)
	for _, e := range entries {
		for _, m := range e.Members {
			vehicleIDs = append(vehicleIDs, m.VehicleID)
		}
	}
	byVehicle := map[uint]domain.Vehicle{}
	if len(vehicleIDs) > 0 {
		rows, err := s.vehicles.ListByIDs(ctx, vehicleIDs)
		if err != nil {
			return layoutroster.DefinedTrains{}, err
		}
		for _, v := range rows {
			byVehicle[v.ID] = v
		}
	}

	out := layoutroster.DefinedTrains{
		LayoutID:  layoutID,
		UpdatedAt: layoutroster.NowMS(),
		Trains:    make([]layoutroster.DefinedTrain, 0, len(entries)),
	}
	for _, e := range entries {
		dt := layoutroster.DefinedTrain{
			TrainID:     e.Train.ID,
			OwnerUserID: e.Train.OwnerUserID,
			Members:     make([]layoutroster.DefinedTrainMember, 0, len(e.Members)),
		}
		for _, m := range e.Members {
			member := layoutroster.DefinedTrainMember{
				VehicleID: m.VehicleID,
				Position:  m.Position,
				Reversed:  m.Reversed,
			}
			if v, ok := byVehicle[m.VehicleID]; ok && v.DCCAddress != nil {
				addr := *v.DCCAddress
				member.Addr = &addr
			}
			dt.Members = append(dt.Members, member)
		}
		out.Trains = append(out.Trains, dt)
	}
	return out, nil
}
