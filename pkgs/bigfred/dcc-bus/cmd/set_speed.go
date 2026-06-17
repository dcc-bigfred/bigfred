package cmd

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
)

// HandleSetSpeed forwards a throttle move to the command station, updates
// Redis, and fans the new state out to subscribed sessions.
func (r *Router) HandleSetSpeed(ctx context.Context, actor Actor, resp Responder, p contract.LocoSetSpeedWire, _ string) Result {
	vehicle, onLayout := r.roster.AllowedVehicle(p.Address)
	if d := r.drive.CanDrive(actor.UserID, vehicle, onLayout); !d.Allowed {
		return FailResult(d.Reason)
	}
	if err := r.applyMemberSetSpeed(ctx, actor, p.Address, p.Speed, p.Forward, p.Emergency, "throttle"); err != nil {
		_ = resp.SendLocoError(ctx, p.Address, errors.CodeCommandStationError, err.Error())
		return FailResult(errors.CodeCommandStationError)
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
		fields := r.stationLogFields()
		fields["addr"] = addr
		fields["speed"] = speed
		fields["forward"] = forward
		fields["emergency"] = emergency
		r.log.WithError(err).WithFields(fields).Warn("dcc-bus command station SetSpeed failed")
		return err
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
	service.BroadcastLocoState(ctx, r.hub, snap)
	return nil
}
