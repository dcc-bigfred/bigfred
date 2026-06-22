package service

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

type EStopTargetConfig struct {
	DccBus      *DccBusService
	Roster      *LayoutVehicleService
	Layouts     CommandStationIDsForLayout
	Auth        *cmd.Auth
	Audit       cmd.AuditPublisher
	IlkSessions *repo.InterlockingSessions
	LayoutIlks  *repo.LayoutInterlockings
	Log         *logrus.Logger
}

// EStopTargetService adapts ws sessions to cmd.EStopTarget.
type EStopTargetService struct {
	core *cmd.EStopTarget
}

func NewEStopTargetService(cfg EStopTargetConfig) *EStopTargetService {
	var roster cmd.EStopTargetRosterPort
	if cfg.Roster != nil {
		roster = eStopTargetRoster{roster: cfg.Roster}
	}
	return &EStopTargetService{core: cmd.NewEStopTarget(cmd.EStopTargetConfig{
		DccBus:      cfg.DccBus,
		Roster:      roster,
		Layouts:     cfg.Layouts,
		Auth:        cfg.Auth,
		Audit:       cfg.Audit,
		IlkSessions: cfg.IlkSessions,
		LayoutIlks:  cfg.LayoutIlks,
		Log:         cfg.Log,
	})}
}

func (s *EStopTargetService) Trigger(
	ctx context.Context,
	sess *ws.DriveSession,
	target domain.TakeoverTarget,
	targetID string,
) (bool, string) {
	return s.core.Trigger(ctx, controlSession{session: sess}, target, targetID)
}

type eStopTargetRoster struct {
	roster *LayoutVehicleService
}

func (r eStopTargetRoster) ListVehicles(ctx context.Context, layoutID uint) ([]cmd.RosterVehicleEntry, error) {
	return r.roster.ListVehicles(ctx, layoutID)
}

func (r eStopTargetRoster) ListTrains(ctx context.Context, layoutID uint) ([]cmd.RosterTrainEntry, error) {
	return r.roster.ListTrains(ctx, layoutID)
}

func (r eStopTargetRoster) LesseesByVehicle(
	ctx context.Context,
	vehicleEntries []cmd.RosterVehicleEntry,
	trainEntries []cmd.RosterTrainEntry,
) (map[domain.VehicleID][]domain.VehicleLessee, error) {
	return r.roster.LayoutRosterSnapshot.LesseesByVehicle(ctx, vehicleEntries, trainEntries)
}
