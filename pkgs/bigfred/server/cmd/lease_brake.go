package cmd

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	dccprotocol "github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// LeaseBrakePort stops a leased vehicle or train on the layout when a lease
// ends. Implementations are best-effort when dcc-bus is unavailable.
type LeaseBrakePort interface {
	StopLeasedTarget(ctx context.Context, layoutID uint, kind domain.TakeoverTarget, targetID string) error
}

// LeaseBrake publishes a per-target emergency stop to every command station
// on the layout.
type LeaseBrake struct {
	dccBus  EStopTargetDccBusPort
	roster  DriveTargetRosterPort
	layouts EStopTargetLayoutsPort
	log     *logrus.Logger
}

type LeaseBrakeConfig struct {
	DccBus  EStopTargetDccBusPort
	Roster  DriveTargetRosterPort
	Layouts EStopTargetLayoutsPort
	Log     *logrus.Logger
}

func NewLeaseBrake(cfg LeaseBrakeConfig) *LeaseBrake {
	log := cfg.Log
	if log == nil {
		log = logrus.New()
	}
	return &LeaseBrake{
		dccBus:  cfg.DccBus,
		roster:  cfg.Roster,
		layouts: cfg.Layouts,
		log:     log,
	}
}

func (b *LeaseBrake) StopLeasedTarget(
	ctx context.Context,
	layoutID uint,
	kind domain.TakeoverTarget,
	targetID string,
) error {
	if b == nil || b.dccBus == nil || b.roster == nil || b.layouts == nil {
		return nil
	}
	addrs, err := ResolveDriveTargetAddrs(ctx, b.roster, layoutID, kind, targetID)
	if err != nil {
		if errors.Is(err, errEStopTargetNotOnLayout) {
			return nil
		}
		return err
	}
	if len(addrs) == 0 {
		return nil
	}
	csIDs, err := b.layouts.CommandStationIDsForLayout(ctx, layoutID)
	if err != nil {
		b.log.WithError(err).WithField("layoutId", layoutID).Warn("lease brake: list command stations")
		return err
	}
	if len(csIDs) == 0 {
		return nil
	}
	payload := contract.EStopTargetCommandWire{Addresses: addrs}
	var lastErr error
	for _, csID := range csIDs {
		if err := b.dccBus.PublishCommand(ctx, layoutID, csID, dccprotocol.TypeSystemEStopTarget, payload); err != nil {
			b.log.WithError(err).WithFields(logrus.Fields{
				"layoutId":         layoutID,
				"commandStationId": csID,
				"target":           kind,
				"targetId":         targetID,
				"addrs":            addrs,
			}).Warn("lease brake: publish")
			lastErr = err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	b.log.WithFields(logrus.Fields{
		"layoutId": layoutID,
		"target":   kind,
		"targetId": targetID,
		"addrs":    addrs,
	}).Info("lease brake triggered")
	return nil
}
