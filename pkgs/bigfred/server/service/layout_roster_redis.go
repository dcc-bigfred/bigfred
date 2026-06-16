package service

import (
	"context"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/helpers"
)

// layoutRosterPublisher pushes full roster snapshots to Redis for dcc-bus.
type layoutRosterPublisher interface {
	PublishLayoutAllowedVehicles(ctx context.Context, snap contract.AllowedVehicles) error
	PublishLayoutDefinedTrains(ctx context.Context, snap contract.DefinedTrains) error
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
// roster with controllerUserIds folded from owner plus active leases.
func (s *LayoutVehicleService) buildAllowedVehiclesSnapshot(ctx context.Context, layoutID uint) (contract.AllowedVehicles, error) {
	entries, err := s.ListVehicles(ctx, layoutID)
	if err != nil {
		return contract.AllowedVehicles{}, err
	}
	trainEntries, err := s.ListTrains(ctx, layoutID)
	if err != nil {
		return contract.AllowedVehicles{}, err
	}

	lesseesByVehicle, err := s.resolveLesseesByVehicle(ctx, entries, trainEntries, time.Now().UTC())
	if err != nil {
		return contract.AllowedVehicles{}, err
	}

	out := contract.AllowedVehicles{
		LayoutID:  layoutID,
		UpdatedAt: contract.NowMS(),
		Vehicles:  make([]contract.AllowedVehicle, 0, len(entries)),
	}
	for _, e := range entries {
		if e.Vehicle.DCCAddress == nil {
			continue
		}
		out.Vehicles = append(out.Vehicles, contract.AllowedVehicle{
			VehicleID:               e.Vehicle.ID,
			Addr:                    *e.Vehicle.DCCAddress,
			OwnerUserID:             e.Vehicle.OwnerUserID,
			ControllerUserIDs:       helpers.MergeUserIDs(e.Vehicle.OwnerUserID, lesseesByVehicle[e.Vehicle.ID]...),
			Rp1Function:             e.Vehicle.Rp1Function,
			EmergencyLightsFunction: e.Vehicle.EmergencyLightsFunction,
			DeadManSwitchOption:     string(e.Vehicle.DeadManSwitchOption),
		})
	}
	return out, nil
}

// resolveLesseesByVehicle is the single place where a train "decomposes"
// into its member vehicles for driving-authority purposes. It returns,
// per vehicle id, the active lessees drawn from BOTH sources:
//
//   - VehicleLease — a lease over the individual vehicle;
//   - TrainLease   — a lease over a whole train, expanded onto every
//     current member vehicle (membership is point-in-time here).
//
// The result is a projection, not a persistence equivalence: it does
// not imply a train is interchangeable with its vehicles (ordering,
// reversal and train identity live in the defined_trains snapshot).
func (s *LayoutVehicleService) resolveLesseesByVehicle(
	ctx context.Context,
	vehicleEntries []RosterVehicleEntry,
	trainEntries []RosterTrainEntry,
	now time.Time,
) (map[uint][]uint, error) {
	lesseesByVehicle := make(map[uint][]uint)

	if s.vehicleLeases != nil && len(vehicleEntries) > 0 {
		vehicleIDs := make([]uint, 0, len(vehicleEntries))
		for _, e := range vehicleEntries {
			vehicleIDs = append(vehicleIDs, e.Vehicle.ID)
		}
		rows, err := s.vehicleLeases.ListActive(ctx, vehicleIDs, now)
		if err != nil {
			return nil, err
		}
		for _, lease := range rows {
			lesseesByVehicle[lease.VehicleID] = append(lesseesByVehicle[lease.VehicleID], lease.ToUserID)
		}
	}

	if s.trainLeases != nil && len(trainEntries) > 0 {
		trainIDs := make([]uint, 0, len(trainEntries))
		for _, e := range trainEntries {
			trainIDs = append(trainIDs, e.Train.ID)
		}
		rows, err := s.trainLeases.ListActive(ctx, trainIDs, now)
		if err != nil {
			return nil, err
		}
		trainLessee := make(map[uint]uint, len(rows))
		for _, lease := range rows {
			trainLessee[lease.TrainID] = lease.ToUserID
		}
		for _, te := range trainEntries {
			lessee, ok := trainLessee[te.Train.ID]
			if !ok {
				continue
			}
			for _, m := range te.Members {
				lesseesByVehicle[m.VehicleID] = append(lesseesByVehicle[m.VehicleID], lessee)
			}
		}
	}

	return lesseesByVehicle, nil
}

// resolveLesseesByTrain returns active lessees per train id on a layout
// roster. Unlike resolveLesseesByVehicle this is train-scoped only: a
// vehicle lease on a member does not appear here.
func (s *LayoutVehicleService) resolveLesseesByTrain(
	ctx context.Context,
	trainEntries []RosterTrainEntry,
	now time.Time,
) (map[uint][]domain.TrainLessee, error) {
	lesseesByTrain := make(map[uint][]domain.TrainLessee)
	if s.trainLeases == nil || len(trainEntries) == 0 {
		return lesseesByTrain, nil
	}
	trainIDs := make([]uint, 0, len(trainEntries))
	for _, e := range trainEntries {
		trainIDs = append(trainIDs, e.Train.ID)
	}
	rows, err := s.trainLeases.ListActive(ctx, trainIDs, now)
	if err != nil {
		return nil, err
	}
	for _, lease := range rows {
		lesseesByTrain[lease.TrainID] = append(lesseesByTrain[lease.TrainID], domain.TrainLessee{
			TrainID:  lease.TrainID,
			ToUserID: lease.ToUserID,
		})
	}
	return lesseesByTrain, nil
}

// buildDefinedTrainsSnapshot lists layout trains with member DCC
// addresses hydrated from the vehicle catalogue.
func (s *LayoutVehicleService) buildDefinedTrainsSnapshot(ctx context.Context, layoutID uint) (contract.DefinedTrains, error) {
	entries, err := s.ListTrains(ctx, layoutID)
	if err != nil {
		return contract.DefinedTrains{}, err
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
			return contract.DefinedTrains{}, err
		}
		for _, v := range rows {
			byVehicle[v.ID] = v
		}
	}

	now := time.Now().UTC()
	trainLessees, err := s.resolveLesseesByTrain(ctx, entries, now)
	if err != nil {
		return contract.DefinedTrains{}, err
	}

	out := contract.DefinedTrains{
		LayoutID:  layoutID,
		UpdatedAt: contract.NowMS(),
		Trains:    make([]contract.DefinedTrain, 0, len(entries)),
	}
	for _, e := range entries {
		dt := contract.DefinedTrain{
			TrainID:           e.Train.ID,
			OwnerUserID:       e.Train.OwnerUserID,
			ControllerUserIDs: helpers.MergeUserIDs(e.Train.OwnerUserID, domain.TrainLesseeUserIDs(trainLessees[e.Train.ID])...),
			Members:           make([]contract.DefinedTrainMember, 0, len(e.Members)),
		}
		for _, m := range e.Members {
			mult := m.SpeedMultiplier
			if mult <= 0 {
				mult = 1.0
			}
			member := contract.DefinedTrainMember{
				VehicleID:       m.VehicleID,
				Position:        m.Position,
				Reversed:        m.Reversed,
				SpeedMultiplier: mult,
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
