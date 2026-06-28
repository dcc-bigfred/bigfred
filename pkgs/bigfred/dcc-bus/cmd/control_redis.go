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
	snap := r.store.SetSpeedPreservingUser(p.Address, p.Speed, p.Forward, "server")
	service.BroadcastLocoState(ctx, r.hub, snap)
}

func (r *Router) applyControlSetFunction(ctx context.Context, p contract.LocoSetFunctionWire) {
	if !r.roster.IsOnLayout(p.Address) {
		return
	}
	// userID 0 preserves the current controller and avoids a Snapshot→
	// SetFunction TOCTOU where a concurrent observation could reset
	// ownership between the read and the write.
	_ = r.setLocoFunction(ctx, p.Address, 0, p.Function, p.On, "server")
}
