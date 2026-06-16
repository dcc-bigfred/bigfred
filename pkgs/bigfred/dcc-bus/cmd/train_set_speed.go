package cmd

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

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
	acks := make([]protocol.TrainSetSpeedMemberAck, 0, len(train.Members))
	allOK := true
	for _, m := range train.Members {
		if m.Addr == nil {
			continue
		}
		mult := m.SpeedMultiplier
		if m.VehicleID == leading.VehicleID && m.Position == leading.Position {
			mult = 1.0
		}
		speed := contract.EffectiveMemberSpeed(p.Speed, mult, maxSpeed)
		forward := p.Forward
		if m.Reversed {
			forward = !forward
		}
		if err := r.applyMemberSetSpeed(ctx, actor, *m.Addr, speed, forward, false, "train"); err != nil {
			allOK = false
			acks = append(acks, protocol.TrainSetSpeedMemberAck{Addr: *m.Addr, OK: false, Error: errors.CodeCommandStationError})
			continue
		}
		acks = append(acks, protocol.TrainSetSpeedMemberAck{Addr: *m.Addr, OK: true})
	}
	res := OKResult()
	res.Members = acks
	if !allOK {
		res.OK = false
		res.Code = errors.CodePartialFailure
	}
	return res
}
