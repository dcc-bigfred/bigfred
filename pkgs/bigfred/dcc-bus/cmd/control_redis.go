package cmd

import (
	"context"
	"encoding/json"
	stderrors "errors"

	"github.com/sirupsen/logrus"

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

	case protocol.TypeLocoCVWrite:
		var p protocol.LocoCVWritePayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		r.logControlProgramming(env.Type, r.HandleLocoCVWrite(ctx, controlActor, noopResponder{}, p, ""))

	case protocol.TypeLocoCVRead:
		var p protocol.LocoCVReadPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		r.logControlProgramming(env.Type, r.HandleLocoCVRead(ctx, controlActor, noopResponder{}, p, ""))

	case protocol.TypeLocoAddrSet:
		var p protocol.LocoAddrSetPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		r.logControlProgramming(env.Type, r.HandleLocoAddrSet(ctx, controlActor, noopResponder{}, p, ""))

	case protocol.TypeLocoAddrGet:
		var p protocol.LocoAddrGetPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		r.logControlProgramming(env.Type, r.HandleLocoAddrGet(ctx, controlActor, noopResponder{}, p, ""))
	}
}

// controlActor labels commands that arrive on the Redis control channel
// rather than from a browser session.
var controlActor = Actor{Source: "server"}

// logControlProgramming reports the outcome of a control-channel
// programming command. The channel is fire-and-forget, so on rejection
// the daemon publishes a control.programming.rejected event on its
// event channel — that is the server's only signal that the command
// did not run (loco-server can log it, surface it to an admin HUD, or
// retry against a different station). On success the daemon log is
// enough; no event is emitted.
func (r *Router) logControlProgramming(frameType string, res Result) {
	if res.OK {
		return
	}
	r.log.WithFields(logrus.Fields{
		"type": frameType,
		"code": res.Code,
	}).Warn("dcc-bus control programming command rejected")
	if r.redis != nil {
		_ = r.redis.Publish(context.Background(), protocol.TypeControlProgrammingRejected,
			protocol.ControlProgrammingRejectedPayload{
				FrameType: frameType,
				Code:      res.Code,
				Address:   res.LocoAddress,
			})
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
	snap := r.store.SetSpeedPreservingUser(p.Address, contract.UISpeedFromWire(service.WireSpeedFromPayload(p.Speed, p.Emergency)), p.Forward, "server")
	service.BroadcastLocoState(ctx, r.hub, snap)
}

func (r *Router) applyControlSetFunction(ctx context.Context, p contract.LocoSetFunctionWire) {
	if !r.roster.IsOnLayout(p.Address) {
		return
	}
	// userID 0 preserves the current controller and avoids a Snapshot→
	// SetFunction TOCTOU where a concurrent observation could reset
	// ownership between the read and the write.
	_ = r.setLocoFunction(ctx, p.Address, 0, p.Function, p.On, "server", "")
}
