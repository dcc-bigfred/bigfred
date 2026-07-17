package cmd

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service/station"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// HandleStealSlot explicitly takes over a LocoNet slot held IN_USE by another
// throttle (user-confirmed). Requires CanDrive; reserves a lease then calls
// StealSlot on the driver.
func (r *Router) HandleStealSlot(ctx context.Context, actor Actor, resp Responder, p protocol.LocoStealSlotPayload, _ string) Result {
	vehicle, onLayout := r.roster.AllowedVehicle(p.Address)
	if d := r.drive.CanDrive(actor.UserID, vehicle, onLayout); !d.Allowed {
		_ = resp.SendLocoError(ctx, p.Address, d.Reason, "")
		return FailResult(d.Reason)
	}
	stealer, ok := station.AsSlotStealer(r.station)
	if !ok {
		err := fmt.Errorf("dcc-bus: slot steal not supported on this command station")
		_ = resp.SendLocoError(ctx, p.Address, errors.CodeCommandStationError, err.Error())
		return FailResult(errors.CodeCommandStationError)
	}
	if err := r.reserveDriveLease(actor, p.Address); err != nil {
		code := locoCommandErrorCode(err)
		_ = resp.SendLocoError(ctx, p.Address, code, err.Error())
		return FailResult(code)
	}
	if err := stealer.StealSlot(commandstation.LocoAddr(p.Address)); err != nil {
		code := locoCommandErrorCode(err)
		_ = resp.SendLocoError(ctx, p.Address, code, err.Error())
		return FailResult(code)
	}
	resp.SetSelected(p.Address)
	r.log.WithFields(logrus.Fields{
		"sessionId": actor.SessionID,
		"userId":    actor.UserID,
		"addr":      p.Address,
	}).Info("dcc-bus loco.stealSlot")
	return OKResult()
}
