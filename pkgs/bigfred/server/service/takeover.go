package service

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

var (
	ErrTakeoverNotConfigured     = svcerrors.ErrTakeoverNotConfigured
	ErrTakeoverTargetNotOnLayout = svcerrors.ErrTakeoverTargetNotOnLayout
	ErrTakeoverNotOwner          = svcerrors.ErrTakeoverNotOwner
	ErrTakeoverAlreadyPending    = svcerrors.ErrTakeoverAlreadyPending
	ErrTakeoverNotFound          = svcerrors.ErrTakeoverNotFound
	ErrTakeoverInvalidState      = svcerrors.ErrTakeoverInvalidState
	ErrTakeoverNotDriver         = svcerrors.ErrTakeoverNotDriver
	ErrTakeoverNotSignalman      = svcerrors.ErrTakeoverNotSignalman
	ErrNotInterlockingOccupant   = svcerrors.ErrNotInterlockingOccupant
)

type TakeoverConfig struct {
	Requests      repo.TakeoverRequestStore
	VehicleLeases repo.VehicleLeaseStore
	TrainLeases   repo.TrainLeaseStore
	Vehicles      *repo.Vehicles
	Trains        *repo.Trains
	TrainMembers  *repo.TrainMembers
	IlkSessions   *repo.InterlockingSessions
	Users         *repo.Users
	Roster        *LayoutVehicleService
	Auth          *cmd.Auth
	Hub           *ws.Hub
	Audit         cmd.AuditPublisher
}

// TakeoverService is the legacy facade for cmd.Takeover.
type TakeoverService struct {
	*cmd.Takeover
}

func NewTakeoverService(cfg TakeoverConfig) *TakeoverService {
	var roster cmd.TakeoverRosterPort
	if cfg.Roster != nil {
		roster = takeoverRoster{roster: cfg.Roster}
	}
	var hub cmd.TakeoverHubPort
	if cfg.Hub != nil {
		hub = takeoverHub{hub: cfg.Hub}
	}
	return &TakeoverService{Takeover: cmd.NewTakeover(cmd.TakeoverConfig{
		Requests:      cfg.Requests,
		VehicleLeases: cfg.VehicleLeases,
		TrainLeases:   cfg.TrainLeases,
		Vehicles:      cfg.Vehicles,
		Trains:        cfg.Trains,
		TrainMembers:  cfg.TrainMembers,
		IlkSessions:   cfg.IlkSessions,
		Users:         cfg.Users,
		Roster:        roster,
		Auth:          cfg.Auth,
		Hub:           hub,
		Audit:         cfg.Audit,
	})}
}

func TakeoverDeniedCode(err error) string { return cmd.TakeoverDeniedCode(err) }

type takeoverRoster struct {
	roster *LayoutVehicleService
}

func (r takeoverRoster) ListVehicles(ctx context.Context, layoutID uint) ([]cmd.RosterVehicleEntry, error) {
	return r.roster.ListVehicles(ctx, layoutID)
}

func (r takeoverRoster) ListTrains(ctx context.Context, layoutID uint) ([]cmd.RosterTrainEntry, error) {
	return r.roster.ListTrains(ctx, layoutID)
}

func (r takeoverRoster) SyncLayoutRoster(ctx context.Context, layoutID uint) error {
	return r.roster.SyncLayoutRosterToRedis(ctx, layoutID)
}

type takeoverHub struct {
	hub *ws.Hub
}

func (h takeoverHub) BroadcastTakeover(layoutID, userID uint, typ string, payload any) {
	h.hub.BroadcastToUserInLayout(layoutID, userID, typ, payload)
}
