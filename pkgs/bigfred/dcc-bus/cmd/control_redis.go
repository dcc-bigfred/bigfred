package cmd

import (
	"context"
	"encoding/json"
	stderrors "errors"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// HandleControlCommand decodes a server-initiated command frame from the
// Redis dcc-bus:cmd channel and applies it.
func (r *Router) HandleControlCommand(ctx context.Context, raw []byte) {
	var env contract.EnvelopeWire
	if err := json.Unmarshal(raw, &env); err != nil {
		r.log.WithError(err).Debug("dcc-bus control cmd: bad envelope")
		return
	}
	r.log.WithField("type", env.Type).Debug("dcc-bus control cmd")

	switch env.Type {
	case protocol.TypeLocoSetSpeed:
		var p contract.LocoSetSpeedWire
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		r.applyControlSetSpeed(ctx, p)

	case protocol.TypeLocoSetFunction:
		var p contract.LocoSetFunctionWire
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		r.applyControlSetFunction(ctx, p)

	case protocol.TypeSystemEStop:
		r.applyEStopAll(ctx, "system")

	case protocol.TypeSystemRadioStop:
		r.HandleRadioStop(ctx)

	case protocol.TypeSystemEStopTarget:
		var p contract.EStopTargetCommandWire
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		r.applyEStopTarget(ctx, p.Addresses)
	}
}

func (r *Router) applyControlSetSpeed(ctx context.Context, p contract.LocoSetSpeedWire) {
	if !r.roster.IsOnLayout(p.Address) {
		return
	}
	if err := r.dcc.SetSpeed(p.Address, p.Speed, p.Forward, p.Emergency); err != nil {
		if stderrors.Is(err, commandstation.ErrSpeedSuperseded) {
			return
		}
		r.log.WithError(err).WithField("addr", p.Address).Warn("dcc-bus control setSpeed failed")
		return
	}
	snap := contract.LocoStateWire{
		Address: p.Address,
		Speed:   p.Speed,
		Forward: p.Forward,
		Source:  "server",
		At:      contract.NowMS(),
	}
	if cached, ok, err := r.redis.GetLocoCurrentState(ctx, p.Address); err == nil && ok {
		snap.Functions = cached.Functions
		snap.ControlledByUserID = cached.ControlledByUserID
	}
	if err := r.redis.StoreLocoCurrentState(ctx, snap, StateTTL); err != nil {
		r.log.WithError(err).Debug("dcc-bus control redis store")
	}
	service.BroadcastLocoState(ctx, r.hub, snap)
}

func (r *Router) applyControlSetFunction(ctx context.Context, p contract.LocoSetFunctionWire) {
	if !r.roster.IsOnLayout(p.Address) {
		return
	}
	userID := uint(0)
	if cached, ok, err := r.redis.GetLocoCurrentState(ctx, p.Address); err == nil && ok {
		userID = cached.ControlledByUserID
	}
	_ = r.setLocoFunction(ctx, p.Address, userID, p.Function, p.On, "server")
}
