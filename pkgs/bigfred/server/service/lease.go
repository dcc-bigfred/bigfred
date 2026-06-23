package service

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// LeaseService is the lease management facade.
type LeaseService struct {
	*cmd.Lease
}

type LeaseConfig struct {
	VehicleLeases  repo.VehicleLeaseStore
	TrainLeases    repo.TrainLeaseStore
	LayoutVehicles *repo.LayoutVehicles
	LayoutTrains   *repo.LayoutTrains
	Vehicles       *repo.Vehicles
	Trains         *repo.Trains
	Users          *repo.Users
	Roster         *LayoutVehicleService
	Hub            *ws.Hub
	Audit          cmd.AuditPublisher
	Brake          cmd.LeaseBrakePort
}

func NewLeaseService(cfg LeaseConfig) *LeaseService {
	var roster cmd.LeaseRosterPort
	if cfg.Roster != nil {
		roster = leaseRoster{svc: cfg.Roster}
	}
	var hub cmd.LeaseHubPort
	if cfg.Hub != nil {
		hub = leaseHub{hub: cfg.Hub}
	}
	return &LeaseService{Lease: cmd.NewLease(cmd.LeaseConfig{
		VehicleLeases:  cfg.VehicleLeases,
		TrainLeases:    cfg.TrainLeases,
		LayoutVehicles: cfg.LayoutVehicles,
		LayoutTrains:   cfg.LayoutTrains,
		Vehicles:       cfg.Vehicles,
		Trains:         cfg.Trains,
		Users:          cfg.Users,
		Roster:         roster,
		Hub:            hub,
		Audit:          cfg.Audit,
		Brake:          cfg.Brake,
	})}
}

type leaseRoster struct {
	svc *LayoutVehicleService
}

func (r leaseRoster) ListVehicles(ctx context.Context, layoutID uint) ([]cmd.RosterVehicleEntry, error) {
	return r.svc.ListVehicles(ctx, layoutID)
}

func (r leaseRoster) ListTrains(ctx context.Context, layoutID uint) ([]cmd.RosterTrainEntry, error) {
	return r.svc.ListTrains(ctx, layoutID)
}

func (r leaseRoster) SyncLayoutRoster(ctx context.Context, layoutID uint) error {
	return r.svc.SyncLayoutRosterToRedis(ctx, layoutID)
}

type leaseHub struct {
	hub *ws.Hub
}

func (h leaseHub) BroadcastLease(layoutID, userID uint, typ string, payload contract.LeaseEventWire) {
	h.hub.BroadcastToUserInLayout(layoutID, userID, typ, payload)
}
