package cmd

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
)

func (r *Router) leadingWasAtStartSpeed(ctx context.Context, leading contract.DefinedTrainMember) bool {
	if leading.Addr == nil {
		return true
	}
	env, ok, err := r.redis.GetLocoCurrentState(ctx, *leading.Addr)
	if err != nil || !ok {
		return true
	}
	return service.IsStartDelayPreviousSpeed(env.Speed)
}

// HandleTrainSetSpeed fans a throttle move to every powered member of a train.
func (r *Router) HandleTrainSetSpeed(ctx context.Context, actor Actor, _ Responder, p contract.TrainSetSpeedWire, _ string) Result {
	train, known := r.findDefinedTrain(p.TrainID)
	if d := r.trainPolicy.CanDriveTrain(actor.UserID, train, known); !d.Allowed {
		return FailResult(d.Reason)
	}
	if d := r.trainPolicy.CanDriveTrainMembers(train); !d.Allowed {
		return FailResult(d.Reason)
	}
	leading, hasLeading := train.LeadingMember()
	if !hasLeading {
		return FailResult(errors.CodeTrainNoPoweredMembers)
	}

	maxSpeed := contract.MaxSpeedForSpeedSteps(r.speedSteps)
	leadingSpeed := p.Speed
	if limit := train.ControllerSpeedLimits[actor.UserID]; limit > 0 {
		leadingSpeed = contract.ClampSpeedForControllerLimit(p.Speed, maxSpeed, limit)
	}
	leadingWasAtStartSpeed := r.leadingWasAtStartSpeed(ctx, leading)
	members := make([]service.TrainMemberSetSpeed, 0, len(train.Members))
	for _, m := range train.Members {
		if m.Addr == nil || m.ExcludeFromSpeed {
			continue
		}
		mult := m.SpeedMultiplier
		if m.IsLeading(leading) {
			mult = 1.0
		}
		speed := contract.EffectiveMemberSpeed(leadingSpeed, mult, maxSpeed)
		forward := p.Forward
		if m.Reversed {
			forward = !forward
		}
		currentSpeed := uint8(0)
		if env, ok, err := r.redis.GetLocoCurrentState(ctx, *m.Addr); err == nil && ok {
			currentSpeed = env.Speed
		}
		maxSteps := m.AccelRampMaxSteps
		if maxSteps <= 0 {
			maxSteps = 1
		}
		brakeSteps := m.BrakeRampMaxSteps
		if brakeSteps <= 0 {
			brakeSteps = 1
		}
		members = append(members, service.TrainMemberSetSpeed{
			Addr:              *m.Addr,
			CurrentSpeed:      currentSpeed,
			Speed:             speed,
			Forward:           forward,
			StartDelayMs:      m.StartDelayMs,
			AccelRampMs:       m.AccelRampMs,
			AccelRampMaxSteps: maxSteps,
			BrakeRampMs:       m.BrakeRampMs,
			BrakeRampMaxSteps: brakeSteps,
		})
	}

	apply := func(callCtx context.Context, addr uint16, speed uint8, forward bool) error {
		return r.applyMemberSetSpeed(callCtx, actor, addr, speed, forward, false, "train")
	}
	acks, allOK := r.trainSpeed.Apply(
		ctx,
		p.TrainID,
		p.Speed,
		leadingWasAtStartSpeed,
		apply,
		members,
	)

	res := OKResult()
	res.Members = acks
	if !allOK {
		res.OK = false
		res.Code = errors.CodePartialFailure
	}
	return res
}
