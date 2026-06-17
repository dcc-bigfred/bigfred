package service

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// Layout roster sentinel errors — re-exported during migration.
var (
	ErrLayoutVehicleAlreadyOnRoster = svcerrors.ErrLayoutVehicleAlreadyOnRoster
	ErrLayoutVehicleNotOnRoster     = svcerrors.ErrLayoutVehicleNotOnRoster
	ErrLayoutTrainAlreadyOnRoster   = svcerrors.ErrLayoutTrainAlreadyOnRoster
	ErrLayoutTrainNotOnRoster       = svcerrors.ErrLayoutTrainNotOnRoster
)

// RosterVehicleEntry is one dashboard row of the layout vehicle roster.
type RosterVehicleEntry = cmd.RosterVehicleEntry

// RosterTrainEntry is the train-shaped sibling of RosterVehicleEntry.
type RosterTrainEntry = cmd.RosterTrainEntry

// VehicleChangedPayload is the JSON body of layout.vehiclesChanged.
type VehicleChangedPayload = protocol.VehicleChangedPayload

// TrainChangedPayload is the JSON body of layout.trainsChanged.
type TrainChangedPayload = protocol.TrainChangedPayload

// layoutRosterRedisSync publishes full roster snapshots for dcc-bus.
type layoutRosterRedisSync interface {
	layoutRosterPublisher
}

// LayoutVehicleService is the layout roster facade over cmd.LayoutRoster
// with Redis snapshot helpers for dcc-bus.
type LayoutVehicleService struct {
	*cmd.LayoutRoster
	*cmd.LayoutRosterSnapshot
	redis layoutRosterRedisSync
}

// NewLayoutVehicleService constructs a LayoutVehicleService.
func NewLayoutVehicleService(
	layoutVehicles *repo.LayoutVehicles,
	layoutTrains *repo.LayoutTrains,
	vehicles *repo.Vehicles,
	trains *repo.Trains,
	members *repo.TrainMembers,
	vehicleLeases repo.Leases[domain.VehicleLease],
	trainLeases repo.Leases[domain.TrainLease],
	users *repo.Users,
	hub *ws.Hub,
) *LayoutVehicleService {
	inner := cmd.NewLayoutRoster(
		layoutVehicles, layoutTrains, vehicles, trains, members, users,
		layoutRosterHub{hub: hub},
	)
	snapshots := cmd.NewLayoutRosterSnapshot(
		inner, layoutTrains, vehicles, members, vehicleLeases, trainLeases, nil,
	)
	return &LayoutVehicleService{
		LayoutRoster:         inner,
		LayoutRosterSnapshot: snapshots,
	}
}

// SetRedisRosterPublisher wires Redis so roster mutations reach dcc-bus.
func (s *LayoutVehicleService) SetRedisRosterPublisher(r layoutRosterRedisSync) {
	s.redis = r
	s.LayoutRosterSnapshot.SetPublisher(r)
	s.LayoutRoster.SetSyncPort(layoutRosterSyncAdapter{s: s})
}

type layoutRosterHub struct {
	hub *ws.Hub
}

func (h layoutRosterHub) BroadcastVehicleChanged(layoutID, vehicleID uint, action string) {
	if h.hub == nil {
		return
	}
	h.hub.BroadcastToLayout(layoutID, "layout.vehiclesChanged", protocol.VehicleChangedPayload{
		LayoutID:  layoutID,
		Action:    action,
		VehicleID: vehicleID,
	})
}

func (h layoutRosterHub) BroadcastTrainChanged(layoutID, trainID uint, action string) {
	if h.hub == nil {
		return
	}
	h.hub.BroadcastToLayout(layoutID, "layout.trainsChanged", protocol.TrainChangedPayload{
		LayoutID: layoutID,
		Action:   action,
		TrainID:  trainID,
	})
}

type layoutRosterSyncAdapter struct {
	s *LayoutVehicleService
}

func (a layoutRosterSyncAdapter) SyncLayout(ctx context.Context, layoutID uint) error {
	return a.s.SyncLayoutRosterToRedis(ctx, layoutID)
}

func (a layoutRosterSyncAdapter) SyncForTrain(ctx context.Context, trainID uint) error {
	return a.s.SyncLayoutRosterForTrain(ctx, trainID)
}

func (a layoutRosterSyncAdapter) SyncForVehicleInTrains(ctx context.Context, vehicleID uint) error {
	return a.s.SyncRosterForVehicleInTrains(ctx, vehicleID)
}
