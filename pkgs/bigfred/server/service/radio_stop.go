package service

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

type RadioStopConfig struct {
	Hub    *ws.Hub
	Redis  *RedisService
	Roster *LayoutVehicleService
	Auth   *cmd.Auth
	Log    *logrus.Logger
}

// RadioStopService adapts ws sessions to cmd.RadioStop.
type RadioStopService struct {
	core *cmd.RadioStop
}

func NewRadioStopService(cfg RadioStopConfig) *RadioStopService {
	var hub cmd.RadioStopHubPort
	if cfg.Hub != nil {
		hub = radioStopHub{hub: cfg.Hub}
	}
	var roster cmd.RadioStopRosterPort
	if cfg.Roster != nil {
		roster = radioStopRoster{roster: cfg.Roster}
	}
	return &RadioStopService{core: cmd.NewRadioStop(cmd.RadioStopConfig{
		Hub:    hub,
		Redis:  cfg.Redis,
		Roster: roster,
		Auth:   cfg.Auth,
		Log:    cfg.Log,
	})}
}

func (s *RadioStopService) Trigger(ctx context.Context, sess *ws.DriveSession) (bool, string) {
	return s.core.Trigger(ctx, controlSession{session: sess})
}

type radioStopHub struct {
	hub *ws.Hub
}

func (h radioStopHub) BroadcastRadioStop(layoutID uint, push contract.RadioStopPushWire) {
	h.hub.BroadcastToLayout(layoutID, ws.TypeSystemRadioStop, push)
}

type radioStopRoster struct {
	roster *LayoutVehicleService
}

func (r radioStopRoster) BuildAllowedVehiclesSnapshot(ctx context.Context, layoutID uint) (contract.AllowedVehicles, error) {
	return r.roster.BuildAllowedVehiclesSnapshot(ctx, layoutID)
}
