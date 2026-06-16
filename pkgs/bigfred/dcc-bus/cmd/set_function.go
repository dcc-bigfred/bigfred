package cmd

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
)

// HandleSetFunction sets a single function on or off.
func (r *Router) HandleSetFunction(ctx context.Context, actor Actor, resp Responder, p contract.LocoSetFunctionWire, _ string) Result {
	vehicle, onLayout := r.roster.AllowedVehicle(p.Address)
	if d := r.drive.CanDrive(actor.UserID, vehicle, onLayout); !d.Allowed {
		return FailResult(d.Reason)
	}
	if err := r.setLocoFunction(ctx, p.Address, actor.UserID, p.Function, p.On, "throttle"); err != nil {
		_ = resp.SendLocoError(ctx, p.Address, errors.CodeCommandStationError, err.Error())
		return FailResult(errors.CodeCommandStationError)
	}
	return OKResult()
}
