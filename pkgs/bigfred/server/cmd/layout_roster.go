package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

// RosterVehicleEntry is one dashboard row of the layout vehicle roster.
type RosterVehicleEntry struct {
	Vehicle            domain.Vehicle
	OwnerLogin         string
	OwnerOrganization  string
	AddedAt            time.Time
}

// RosterTrainEntry is the train-shaped sibling of RosterVehicleEntry.
type RosterTrainEntry struct {
	Train             domain.Train
	OwnerLogin        string
	OwnerOrganization string
	AddedAt           time.Time
	Members           []domain.TrainMember
}

// LayoutRoster implements per-layout roster CRUD (§3a.1, §6.3c).
type LayoutRoster struct {
	layoutVehicles *repo.LayoutVehicles
	layoutTrains   *repo.LayoutTrains
	vehicles       *repo.Vehicles
	trains         *repo.Trains
	members        *repo.TrainMembers
	users          *repo.Users
	hub            LayoutRosterHubPort
	sync           LayoutRosterSyncPort
	sec            security.LayoutSecurityContext
}

// NewLayoutRoster constructs a LayoutRoster use-case handler.
func NewLayoutRoster(
	layoutVehicles *repo.LayoutVehicles,
	layoutTrains *repo.LayoutTrains,
	vehicles *repo.Vehicles,
	trains *repo.Trains,
	members *repo.TrainMembers,
	users *repo.Users,
	hub LayoutRosterHubPort,
) *LayoutRoster {
	return &LayoutRoster{
		layoutVehicles: layoutVehicles,
		layoutTrains:   layoutTrains,
		vehicles:       vehicles,
		trains:         trains,
		members:        members,
		users:          users,
		hub:            hub,
		sec:            security.LayoutSecurityContext{},
	}
}

// SetSyncPort wires Redis roster snapshot publishing.
func (r *LayoutRoster) SetSyncPort(p LayoutRosterSyncPort) {
	r.sync = p
}

// ListVehicles returns the layout vehicle roster ordered by AddedAt.
func (r *LayoutRoster) ListVehicles(ctx context.Context, layoutID uint) ([]RosterVehicleEntry, error) {
	rows, err := r.layoutVehicles.ListByLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	ids := make([]domain.VehicleID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.VehicleID)
	}
	vehicles, err := r.vehicles.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[domain.VehicleID]domain.Vehicle, len(vehicles))
	for _, v := range vehicles {
		byID[v.ID] = v
	}

	out := make([]RosterVehicleEntry, 0, len(rows))
	for _, row := range rows {
		v, ok := byID[row.VehicleID]
		if !ok {
			continue
		}
		owner, err := r.users.FindByID(ctx, v.OwnerUserID)
		ownerLogin, ownerOrganization := "", ""
		if err == nil {
			ownerLogin = owner.Login
			ownerOrganization = owner.Organization
		}
		out = append(out, RosterVehicleEntry{
			Vehicle:           v,
			OwnerLogin:        ownerLogin,
			OwnerOrganization: ownerOrganization,
			AddedAt:           row.AddedAt,
		})
	}
	return out, nil
}

// ListTrains returns the layout train roster with members hydrated.
func (r *LayoutRoster) ListTrains(ctx context.Context, layoutID uint) ([]RosterTrainEntry, error) {
	rows, err := r.layoutTrains.ListByLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	ids := make([]domain.TrainID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.TrainID)
	}
	trains, err := r.trains.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[domain.TrainID]domain.Train, len(trains))
	for _, t := range trains {
		byID[t.ID] = t
	}

	out := make([]RosterTrainEntry, 0, len(rows))
	for _, row := range rows {
		t, ok := byID[row.TrainID]
		if !ok {
			continue
		}
		owner, err := r.users.FindByID(ctx, t.OwnerUserID)
		ownerLogin, ownerOrganization := "", ""
		if err == nil {
			ownerLogin = owner.Login
			ownerOrganization = owner.Organization
		}
		members, err := r.members.ListByTrain(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, RosterTrainEntry{
			Train:             t,
			OwnerLogin:        ownerLogin,
			OwnerOrganization: ownerOrganization,
			AddedAt:           row.AddedAt,
			Members:           members,
		})
	}
	return out, nil
}

// AddVehicle attaches a vehicle to the layout roster (owner-only).
func (r *LayoutRoster) AddVehicle(ctx context.Context, layoutID, actorID uint, vehicleID domain.VehicleID) (RosterVehicleEntry, error) {
	v, err := r.vehicles.FindByID(ctx, vehicleID)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleNotFound) {
			return RosterVehicleEntry{}, svcerrors.ErrVehicleNotFound
		}
		return RosterVehicleEntry{}, err
	}
	if v.OwnerUserID != actorID {
		return RosterVehicleEntry{}, svcerrors.ErrVehicleNotOwned
	}
	if _, err := r.layoutVehicles.FindByLayoutAndVehicle(ctx, layoutID, vehicleID); err == nil {
		return RosterVehicleEntry{}, svcerrors.ErrLayoutVehicleAlreadyOnRoster
	} else if !errors.Is(err, repo.ErrLayoutVehicleNotFound) {
		return RosterVehicleEntry{}, err
	}

	now := time.Now().UTC()
	row := domain.LayoutVehicle{
		LayoutID:      layoutID,
		VehicleID:     v.ID,
		AddedByUserID: actorID,
		AddedAt:       now,
	}
	if err := r.layoutVehicles.Insert(ctx, &row); err != nil {
		return RosterVehicleEntry{}, err
	}

	owner, err := r.users.FindByID(ctx, v.OwnerUserID)
	ownerLogin, ownerOrganization := "", ""
	if err == nil {
		ownerLogin = owner.Login
		ownerOrganization = owner.Organization
	}
	entry := RosterVehicleEntry{
		Vehicle:           v,
		OwnerLogin:        ownerLogin,
		OwnerOrganization: ownerOrganization,
		AddedAt:           row.AddedAt,
	}
	r.broadcastVehicleChanged(layoutID, v.ID, "added")
	return entry, nil
}

// RemoveVehicle detaches a vehicle from the layout roster.
func (r *LayoutRoster) RemoveVehicle(ctx context.Context, layoutID, actorID uint, vehicleID domain.VehicleID, eff domain.EffectiveRoles) error {
	row, err := r.layoutVehicles.FindByLayoutAndVehicle(ctx, layoutID, vehicleID)
	if err != nil {
		if errors.Is(err, repo.ErrLayoutVehicleNotFound) {
			return svcerrors.ErrLayoutVehicleNotOnRoster
		}
		return err
	}
	v, err := r.vehicles.FindByID(ctx, vehicleID)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleNotFound) {
			return svcerrors.ErrVehicleNotFound
		}
		return err
	}
	decision := r.sec.CanRemoveVehicleFromRoster(eff, actorID, v.OwnerUserID)
	if !decision.Allowed {
		switch decision.Reason {
		case security.ReasonVehicleNotOwned:
			return svcerrors.ErrVehicleNotOwned
		default:
			return errors.New(decision.Reason)
		}
	}
	if err := r.layoutVehicles.Delete(ctx, &row); err != nil {
		return err
	}
	r.broadcastVehicleChanged(layoutID, vehicleID, "removed")
	return nil
}

// AddTrain attaches a train to the layout roster (owner-only).
func (r *LayoutRoster) AddTrain(ctx context.Context, layoutID, actorID uint, trainID domain.TrainID) (RosterTrainEntry, error) {
	t, err := r.trains.FindByID(ctx, trainID)
	if err != nil {
		if errors.Is(err, repo.ErrTrainNotFound) {
			return RosterTrainEntry{}, svcerrors.ErrTrainNotFound
		}
		return RosterTrainEntry{}, err
	}
	if t.OwnerUserID != actorID {
		return RosterTrainEntry{}, svcerrors.ErrTrainNotOwned
	}
	if _, err := r.layoutTrains.FindByLayoutAndTrain(ctx, layoutID, trainID); err == nil {
		return RosterTrainEntry{}, svcerrors.ErrLayoutTrainAlreadyOnRoster
	} else if !errors.Is(err, repo.ErrLayoutTrainNotFound) {
		return RosterTrainEntry{}, err
	}

	now := time.Now().UTC()
	row := domain.LayoutTrain{
		LayoutID:      layoutID,
		TrainID:       t.ID,
		AddedByUserID: actorID,
		AddedAt:       now,
	}
	if err := r.layoutTrains.Insert(ctx, &row); err != nil {
		return RosterTrainEntry{}, err
	}

	owner, err := r.users.FindByID(ctx, t.OwnerUserID)
	ownerLogin, ownerOrganization := "", ""
	if err == nil {
		ownerLogin = owner.Login
		ownerOrganization = owner.Organization
	}
	members, err := r.members.ListByTrain(ctx, t.ID)
	if err != nil {
		return RosterTrainEntry{}, err
	}
	entry := RosterTrainEntry{
		Train:             t,
		OwnerLogin:        ownerLogin,
		OwnerOrganization: ownerOrganization,
		AddedAt:           row.AddedAt,
		Members:           members,
	}
	r.broadcastTrainChanged(layoutID, t.ID, "added")
	return entry, nil
}

// RemoveTrain detaches a train from the layout roster.
func (r *LayoutRoster) RemoveTrain(ctx context.Context, layoutID, actorID uint, trainID domain.TrainID, eff domain.EffectiveRoles) error {
	row, err := r.layoutTrains.FindByLayoutAndTrain(ctx, layoutID, trainID)
	if err != nil {
		if errors.Is(err, repo.ErrLayoutTrainNotFound) {
			return svcerrors.ErrLayoutTrainNotOnRoster
		}
		return err
	}
	t, err := r.trains.FindByID(ctx, trainID)
	if err != nil {
		if errors.Is(err, repo.ErrTrainNotFound) {
			return svcerrors.ErrTrainNotFound
		}
		return err
	}
	decision := r.sec.CanRemoveTrainFromRoster(eff, actorID, t.OwnerUserID)
	if !decision.Allowed {
		switch decision.Reason {
		case security.ReasonTrainNotOwned:
			return svcerrors.ErrTrainNotOwned
		default:
			return errors.New(decision.Reason)
		}
	}
	if err := r.layoutTrains.Delete(ctx, &row); err != nil {
		return err
	}
	r.broadcastTrainChanged(layoutID, trainID, "removed")
	return nil
}

// PurgeVehicle deletes every roster row pointing at a vehicle.
func (r *LayoutRoster) PurgeVehicle(ctx context.Context, vehicleID domain.VehicleID) error {
	rows, err := r.layoutVehicles.ListByVehicle(ctx, vehicleID)
	if err != nil {
		return err
	}
	if err := r.layoutVehicles.DeleteAllForVehicle(ctx, vehicleID); err != nil {
		return err
	}
	for _, row := range rows {
		r.broadcastVehicleChanged(row.LayoutID, vehicleID, "removed")
	}
	return nil
}

// PurgeTrain deletes every roster row pointing at a train.
func (r *LayoutRoster) PurgeTrain(ctx context.Context, trainID domain.TrainID) error {
	rows, err := r.layoutTrains.ListByTrain(ctx, trainID)
	if err != nil {
		return err
	}
	if err := r.layoutTrains.DeleteAllForTrain(ctx, trainID); err != nil {
		return err
	}
	for _, row := range rows {
		r.broadcastTrainChanged(row.LayoutID, trainID, "removed")
	}
	return nil
}

// BroadcastVehicleUpdated notifies dashboards after a catalogue vehicle update.
func (r *LayoutRoster) BroadcastVehicleUpdated(ctx context.Context, vehicleID domain.VehicleID) error {
	rows, err := r.layoutVehicles.ListByVehicle(ctx, vehicleID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		r.broadcastVehicleChanged(row.LayoutID, vehicleID, "updated")
	}
	if r.sync != nil {
		return r.sync.SyncForVehicleInTrains(ctx, vehicleID)
	}
	return nil
}

// BroadcastTrainUpdated notifies dashboards and republishes Redis train snapshots.
func (r *LayoutRoster) BroadcastTrainUpdated(ctx context.Context, trainID domain.TrainID) error {
	rows, err := r.layoutTrains.ListByTrain(ctx, trainID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if r.hub != nil {
			r.hub.BroadcastTrainChanged(row.LayoutID, trainID, "updated")
		}
	}
	if r.sync != nil {
		return r.sync.SyncForTrain(ctx, trainID)
	}
	return nil
}

func (r *LayoutRoster) broadcastVehicleChanged(layoutID uint, vehicleID domain.VehicleID, action string) {
	if r.hub != nil {
		r.hub.BroadcastVehicleChanged(layoutID, vehicleID, action)
	}
	if r.sync != nil && layoutID != 0 {
		_ = r.sync.SyncLayout(context.Background(), layoutID)
	}
}

func (r *LayoutRoster) broadcastTrainChanged(layoutID uint, trainID domain.TrainID, action string) {
	if r.hub != nil {
		r.hub.BroadcastTrainChanged(layoutID, trainID, action)
	}
	if r.sync != nil && layoutID != 0 {
		_ = r.sync.SyncLayout(context.Background(), layoutID)
	}
}
