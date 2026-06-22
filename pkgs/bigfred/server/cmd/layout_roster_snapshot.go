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
			ControllerUserIDs:       foldDriveControllers(e.Vehicle.OwnerUserID, domain.VehicleLesseeUserIDs(lesseesByVehicle[e.Vehicle.ID])),
			ControllerSpeedLimits:   buildVehicleSpeedLimits(lesseesByVehicle[e.Vehicle.ID]),
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
) (map[domain.VehicleID][]domain.VehicleLessee, error) {
	now := time.Now().UTC()
	lesseesByVehicle := make(map[domain.VehicleID][]domain.VehicleLessee)

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
			lesseesByVehicle[lease.VehicleID] = append(lesseesByVehicle[lease.VehicleID], domain.VehicleLessee{
				UserID:     lease.ToUserID,
				SpeedLimit: lease.SpeedLimit,
			})
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
		trainLessee := make(map[domain.TrainID]domain.VehicleLessee, len(rows))
		for _, lease := range rows {
			trainLessee[lease.TrainID] = domain.VehicleLessee{
				UserID:     lease.ToUserID,
				SpeedLimit: lease.SpeedLimit,
			}
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

// foldDriveControllers returns the drive-authority controller set for a
// roster target published to dcc-bus. While at least one lessee holds an
// active lease the owner loses drive authority (mirrors
// security.DriveSecurityContext: "the owner may drive only while no one
// else holds a lease"). With no lessees the owner is the sole controller.
// This is what stops a lent-out vehicle from staying drivable by — and an
// emergency-stop target of — its original owner.
func foldDriveControllers(ownerID uint, lesseeIDs []uint) []uint {
	if len(lesseeIDs) == 0 {
		return helpers.MergeUserIDs(ownerID)
	}
	return helpers.MergeUserIDs(0, lesseeIDs...)
}

func buildVehicleSpeedLimits(lessees []domain.VehicleLessee) map[uint]uint8 {
	if len(lessees) == 0 {
		return nil
	}
	out := make(map[uint]uint8)
	for _, l := range lessees {
		if l.SpeedLimit > 0 && l.SpeedLimit < 100 {
			out[l.UserID] = l.SpeedLimit
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildTrainSpeedLimits(lessees []domain.TrainLessee) map[uint]uint8 {
	if len(lessees) == 0 {
		return nil
	}
	out := make(map[uint]uint8)
	for _, l := range lessees {
		if l.SpeedLimit > 0 && l.SpeedLimit < 100 {
			out[l.ToUserID] = l.SpeedLimit
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
			TrainID:               e.Train.ID.String(),
			OwnerUserID:           e.Train.OwnerUserID,
			ControllerUserIDs:     foldDriveControllers(e.Train.OwnerUserID, domain.TrainLesseeUserIDs(trainLessees[e.Train.ID])),
			ControllerSpeedLimits: buildTrainSpeedLimits(trainLessees[e.Train.ID]),
			Members:               make([]contract.DefinedTrainMember, 0, len(e.Members)),
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
			TrainID:    lease.TrainID,
			ToUserID:   lease.ToUserID,
			SpeedLimit: lease.SpeedLimit,
		})
	}
	return lesseesByTrain, nil
}
