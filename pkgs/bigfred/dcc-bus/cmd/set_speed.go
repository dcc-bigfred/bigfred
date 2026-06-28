package cmd

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
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
	if err := r.applyMemberSetSpeed(ctx, actor, p.Address, p.Speed, p.Forward, p.Emergency, "throttle"); err != nil {
		code := locoCommandErrorCode(err)
		_ = resp.SendLocoError(ctx, p.Address, code, err.Error())
		return FailResult(code)
	}
	return OKResult()
}

func (r *Router) applyMemberSetSpeed(
	ctx context.Context,
	actor Actor,
	addr uint16,
	speed uint8,
	forward bool,
	emergency bool,
	source string,
) error {
	if err := r.dcc.SetSpeed(addr, speed, forward, emergency); err != nil {
		if stderrors.Is(err, commandstation.ErrSpeedSuperseded) {
			return nil
		}
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
	}
	r.log.WithFields(logrus.Fields{
		"addr":    addr,
		"speed":   speed,
		"forward": forward,
	}).Debug("dcc-bus command station SetSpeed ok")
	snap := contract.LocoStateWire{
		Address:            addr,
		Speed:              speed,
		Forward:            forward,
		ControlledByUserID: actor.UserID,
		Source:             source,
		At:                 time.Now().UTC().UnixMilli(),
	}
	if env, ok, err := r.redis.GetLocoCurrentState(ctx, addr); err == nil && ok {
		snap.Functions = env.Functions
	}
	if err := r.redis.StoreLocoCurrentState(ctx, snap, StateTTL); err != nil {
		r.log.WithError(err).Debug("dcc-bus redis store")
	}
	r.broadcastLocoState(ctx, snap)
	return nil
}
