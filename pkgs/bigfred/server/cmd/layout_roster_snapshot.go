package cmd

import (
	"context"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/helpers"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
)

// LayoutRosterSnapshotPublisher pushes full roster snapshots to infrastructure.
type LayoutRosterSnapshotPublisher interface {
	PublishLayoutAllowedVehicles(ctx context.Context, snap contract.AllowedVehicles) error
	PublishLayoutDefinedTrains(ctx context.Context, snap contract.DefinedTrains) error
}

// LayoutRosterSnapshot builds dcc-bus roster snapshots from layout roster data.
type LayoutRosterSnapshot struct {
	roster        *LayoutRoster
	layoutTrains  *repo.LayoutTrains
	vehicles      *repo.Vehicles
	members       *repo.TrainMembers
	vehicleLeases repo.VehicleLeaseStore
	trainLeases   repo.TrainLeaseStore
	publisher     LayoutRosterSnapshotPublisher
}

func NewLayoutRosterSnapshot(
	roster *LayoutRoster,
	layoutTrains *repo.LayoutTrains,
	vehicles *repo.Vehicles,
	members *repo.TrainMembers,
	vehicleLeases repo.VehicleLeaseStore,
	trainLeases repo.TrainLeaseStore,
	publisher LayoutRosterSnapshotPublisher,
) *LayoutRosterSnapshot {
	return &LayoutRosterSnapshot{
		roster:        roster,
		layoutTrains:  layoutTrains,
		vehicles:      vehicles,
		members:       members,
		vehicleLeases: vehicleLeases,
		trainLeases:   trainLeases,
		publisher:     publisher,
	}
}

func (s *LayoutRosterSnapshot) SetPublisher(p LayoutRosterSnapshotPublisher) {
	s.publisher = p
}

// SyncLayoutRosterForTrain republishes roster snapshots on every layout
// that has trainID on its roster.
func (s *LayoutRosterSnapshot) SyncLayoutRosterForTrain(ctx context.Context, trainID domain.TrainID) error {
	if s == nil || s.publisher == nil || s.layoutTrains == nil || trainID.IsZero() {
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

// SyncRosterForVehicleInTrains refreshes snapshots for layouts whose
// roster trains include vehicleID.
func (s *LayoutRosterSnapshot) SyncRosterForVehicleInTrains(ctx context.Context, vehicleID domain.VehicleID) error {
	if s == nil || s.publisher == nil || s.members == nil || vehicleID.IsZero() {
		return nil
	}
	memberRows, err := s.members.ListByVehicle(ctx, vehicleID)
	if err != nil {
		return err
	}
	seenTrain := make(map[domain.TrainID]struct{}, len(memberRows))
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

// SyncLayoutRosterToRedis rebuilds and publishes both layout roster snapshots.
func (s *LayoutRosterSnapshot) SyncLayoutRosterToRedis(ctx context.Context, layoutID uint) error {
	if s == nil || s.publisher == nil || layoutID == 0 {
		return nil
	}
	vehicles, err := s.BuildAllowedVehiclesSnapshot(ctx, layoutID)
	if err != nil {
		return err
	}
	trains, err := s.BuildDefinedTrainsSnapshot(ctx, layoutID)
	if err != nil {
		return err
	}
	if err := s.publisher.PublishLayoutAllowedVehicles(ctx, vehicles); err != nil {
		return err
	}
	return s.publisher.PublishLayoutDefinedTrains(ctx, trains)
}

// BuildAllowedVehiclesSnapshot lists drivable vehicles on the layout
// roster with controllerUserIds folded from owner plus active leases.
func (s *LayoutRosterSnapshot) BuildAllowedVehiclesSnapshot(ctx context.Context, layoutID uint) (contract.AllowedVehicles, error) {
	entries, err := s.roster.ListVehicles(ctx, layoutID)
	if err != nil {
		return contract.AllowedVehicles{}, err
	}
	trainEntries, err := s.roster.ListTrains(ctx, layoutID)
	if err != nil {
		return contract.AllowedVehicles{}, err
	}

	lesseesByVehicle, err := s.LesseesByVehicle(ctx, entries, trainEntries)
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
			VehicleID:               e.Vehicle.ID.String(),
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

// LesseesByVehicle returns active vehicle controllers expanded from both
// vehicle leases and train leases.
func (s *LayoutRosterSnapshot) LesseesByVehicle(
	ctx context.Context,
	vehicleEntries []RosterVehicleEntry,
	trainEntries []RosterTrainEntry,
) (map[domain.VehicleID][]uint, error) {
	now := time.Now().UTC()
	lesseesByVehicle := make(map[domain.VehicleID][]uint)

	if s.vehicleLeases != nil && len(vehicleEntries) > 0 {
		vehicleIDs := make([]domain.VehicleID, 0, len(vehicleEntries))
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
		trainIDs := make([]domain.TrainID, 0, len(trainEntries))
		for _, e := range trainEntries {
			trainIDs = append(trainIDs, e.Train.ID)
		}
		rows, err := s.trainLeases.ListActive(ctx, trainIDs, now)
		if err != nil {
			return nil, err
		}
		trainLessee := make(map[domain.TrainID]uint, len(rows))
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

// BuildDefinedTrainsSnapshot lists layout trains with member DCC
// addresses hydrated from the vehicle catalogue.
func (s *LayoutRosterSnapshot) BuildDefinedTrainsSnapshot(ctx context.Context, layoutID uint) (contract.DefinedTrains, error) {
	entries, err := s.roster.ListTrains(ctx, layoutID)
	if err != nil {
		return contract.DefinedTrains{}, err
	}
	vehicleIDs := make([]domain.VehicleID, 0)
	for _, e := range entries {
		for _, m := range e.Members {
			vehicleIDs = append(vehicleIDs, m.VehicleID)
		}
	}
	byVehicle := map[domain.VehicleID]domain.Vehicle{}
	if len(vehicleIDs) > 0 {
		rows, err := s.vehicles.ListByIDs(ctx, vehicleIDs)
		if err != nil {
			return contract.DefinedTrains{}, err
		}
		for _, v := range rows {
			byVehicle[v.ID] = v
		}
	}

	trainLessees, err := s.TrainLessees(ctx, entries)
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
			TrainID:           e.Train.ID.String(),
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
				VehicleID:        m.VehicleID.String(),
				Position:         m.Position,
				Reversed:         m.Reversed,
				SpeedMultiplier:  mult,
				ExcludeFromSpeed: m.ExcludeFromSpeed,
				StartDelayMs:          m.StartDelayMs,
				AccelRampMs:           m.AccelRampMs,
				AccelRampMaxSteps:     m.AccelRampMaxSteps,
				BrakeRampMs:           m.BrakeRampMs,
				BrakeRampMaxSteps:     m.BrakeRampMaxSteps,
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

// TrainLessees returns active lessees per train id for roster entries.
func (s *LayoutRosterSnapshot) TrainLessees(
	ctx context.Context,
	trainEntries []RosterTrainEntry,
) (map[domain.TrainID][]domain.TrainLessee, error) {
	lesseesByTrain := make(map[domain.TrainID][]domain.TrainLessee)
	if s.trainLeases == nil || len(trainEntries) == 0 {
		return lesseesByTrain, nil
	}
	trainIDs := make([]domain.TrainID, 0, len(trainEntries))
	for _, e := range trainEntries {
		trainIDs = append(trainIDs, e.Train.ID)
	}
	rows, err := s.trainLeases.ListActive(ctx, trainIDs, time.Now().UTC())
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
