package cmd

import (
	"context"
	stderrors "errors"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// HandleSetSpeed forwards a throttle move to the command station, updates
// Redis, and fans the new state out to subscribed sessions.
func (r *Router) HandleSetSpeed(ctx context.Context, actor Actor, resp Responder, p contract.LocoSetSpeedWire, _ string) Result {
	vehicle, onLayout := r.roster.AllowedVehicle(p.Address)
	if d := r.drive.CanDrive(actor.UserID, vehicle, onLayout); !d.Allowed {
		_ = resp.SendLocoError(ctx, p.Address, d.Reason, "")
		return FailResult(d.Reason)
	}
	maxSpeed := contract.MaxSpeedForSpeedSteps(r.speedSteps)
	if limit := vehicle.ControllerSpeedLimits[actor.UserID]; limit > 0 {
		p.Speed = contract.ClampSpeedForControllerLimit(p.Speed, maxSpeed, limit)
	}
	if err := r.applyMemberSetSpeed(ctx, actor, p.Address, p.Speed, p.Forward, p.Emergency, "throttle", remotes.HandsetClientKeyFromSession(actor.SessionID)); err != nil {
		code := locoCommandErrorCode(err)
		_ = resp.SendLocoError(ctx, p.Address, code, err.Error())
		return FailResult(code)
	}
	if p.Speed > 1 && !p.Emergency {
		r.enforceSingleVehicleControl(ctx, actor, p.Address)
	}
	return OKResult()
}

// reserveDriveLease records a driver holder for addr before the command
// station acquires the physical slot. OnSlotInUse confirms acquiredAt once
// the driver reports IN_USE.
func (r *Router) reserveDriveLease(actor Actor, addr uint16) error {
	_, err := r.leaser.Reserve(actor.UserID, actor.SessionID, actor.LeaseSource(), addr)
	return err
}

func (r *Router) applyMemberSetSpeed(
	ctx context.Context,
	actor Actor,
	addr uint16,
	speed uint8,
	forward bool,
	emergency bool,
	source string,
	originClientKey string,
) error {
	if err := r.reserveDriveLease(actor, addr); err != nil {
		return err
	}
	if err := r.dcc.SetSpeed(addr, speed, forward, emergency); err != nil {
		if stderrors.Is(err, commandstation.ErrSpeedSuperseded) {
			return nil
		}
		if commandstation.IsSlotAcquireError(err) {
			r.forceRevalidateSlot(addr)
			if retryErr := r.dcc.SetSpeed(addr, speed, forward, emergency); retryErr != nil {
				if stderrors.Is(retryErr, commandstation.ErrSpeedSuperseded) {
					return nil
				}
				fields := r.stationLogFields()
				fields["addr"] = addr
				fields["speed"] = speed
				fields["forward"] = forward
				fields["emergency"] = emergency
				r.log.WithError(retryErr).WithFields(fields).Warn("dcc-bus command station SetSpeed failed")
				return retryErr
			}
			r.log.WithField("addr", addr).Debug("dcc-bus SetSpeed succeeded after slot revalidate")
		} else {
			fields := r.stationLogFields()
			fields["addr"] = addr
			fields["speed"] = speed
			fields["forward"] = forward
			fields["emergency"] = emergency
			r.log.WithError(err).WithFields(fields).Warn("dcc-bus command station SetSpeed failed")
			return err
		}
	}
	r.log.WithFields(logrus.Fields{
		"addr":    addr,
		"speed":   speed,
		"forward": forward,
	}).Debug("dcc-bus command station SetSpeed ok")

	snap := r.store.SetSpeed(addr, contract.UISpeedFromWire(service.WireSpeedFromPayload(speed, emergency)), forward, actor.UserID, source)
	r.broadcastLocoState(ctx, snap, originClientKey)
	return nil
}
