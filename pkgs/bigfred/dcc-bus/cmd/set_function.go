package cmd

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
)

// HandleSetFunction sets a single function on or off.
func (r *Router) HandleSetFunction(ctx context.Context, actor Actor, resp Responder, p contract.LocoSetFunctionWire, _ string) Result {
	vehicle, onLayout := r.roster.AllowedVehicle(p.Address)
	if d := r.drive.CanDrive(actor.UserID, vehicle, onLayout); !d.Allowed {
		return FailResult(d.Reason)
	}
	on := p.On
	if p.Toggle {
		on = !r.currentFunctionState(p.Address, p.Function)
	}
	origin := remotes.HandsetClientKeyFromSession(actor.SessionID)
	if err := r.setLocoFunction(ctx, p.Address, actor.UserID, p.Function, on, "throttle", origin); err != nil {
		_ = resp.SendLocoError(ctx, p.Address, errors.CodeCommandStationError, err.Error())
		return FailResult(errors.CodeCommandStationError)
	}
	return OKResult()
}

func (r *Router) currentFunctionState(addr uint16, fn uint8) bool {
	if r.store == nil {
		return false
	}
	env := r.store.Snapshot(addr)
	if int(fn) >= len(env.Functions) {
		return false
	}
	return env.Functions[fn]
}
