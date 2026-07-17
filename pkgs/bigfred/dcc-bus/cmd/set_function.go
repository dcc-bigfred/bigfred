package cmd

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
)

// HandleSetFunction sets a single function on or off.
func (r *Router) HandleSetFunction(ctx context.Context, actor Actor, resp Responder, p contract.LocoSetFunctionWire, _ string) Result {
	vehicle, onLayout := r.roster.AllowedVehicle(p.Address)
	if d := r.drive.CanDrive(actor.UserID, vehicle, onLayout); !d.Allowed {
		return FailResult(d.Reason)
	}
	if err := r.reserveDriveLease(actor, p.Address); err != nil {
		code := locoCommandErrorCode(err)
		_ = resp.SendLocoError(ctx, p.Address, code, err.Error())
		return FailResult(code)
	}
	on := p.On
	if p.Toggle {
		on = !r.currentFunctionState(p.Address, p.Function)
	}
	origin := remotes.HandsetClientKeyFromSession(actor.SessionID)
	if on {
		if def, ok := r.getMomentaryDef(p.Address, p.Function); ok {
			r.setTimedLocoFunctionWithRetry(p.Address, actor.UserID, p.Function, def.GetMomentaryDuration(), "throttle", 0, origin)
			return OKResult()
		}
	}
	if err := r.setLocoFunction(ctx, p.Address, actor.UserID, p.Function, on, "throttle", origin); err != nil {
		code := locoCommandErrorCode(err)
		_ = resp.SendLocoError(ctx, p.Address, code, err.Error())
		return FailResult(code)
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
