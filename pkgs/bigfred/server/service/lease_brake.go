package service

import (
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
)

type LeaseBrakeConfig struct {
	DccBus  *DccBusService
	Roster  *LayoutVehicleService
	Layouts CommandStationIDsForLayout
	Log     *logrus.Logger
}

// NewLeaseBrake returns a lease-end brake publisher, or nil when dcc-bus is
// not configured.
func NewLeaseBrake(cfg LeaseBrakeConfig) cmd.LeaseBrakePort {
	if cfg.DccBus == nil || cfg.Roster == nil || cfg.Layouts == nil {
		return nil
	}
	return cmd.NewLeaseBrake(cmd.LeaseBrakeConfig{
		DccBus:  cfg.DccBus,
		Roster:  cfg.Roster,
		Layouts: cfg.Layouts,
		Log:     cfg.Log,
	})
}
