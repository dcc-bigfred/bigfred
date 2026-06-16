package cmd

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/security"
)

// HandleSubscribe accepts a subscription request and immediately emits a
// state snapshot for each accepted address.
func (r *Router) HandleSubscribe(ctx context.Context, actor Actor, resp Responder, payload protocol.LocoSubscribePayload, _ string) Result {
	accepted := make([]uint16, 0, len(payload.Addresses))
	rejected := make([]uint16, 0)
	for _, addr := range payload.Addresses {
		if !r.roster.IsOnLayout(addr) {
			rejected = append(rejected, addr)
			_ = resp.SendLocoError(ctx, addr, security.ReasonVehicleNotOnLayout, "")
			continue
		}
		accepted = append(accepted, addr)
	}
	fields := logrus.Fields{
		"sessionId": actor.SessionID,
		"requested": payload.Addresses,
		"accepted":  accepted,
		"rejected":  rejected,
	}
	if len(rejected) > 0 {
		r.log.WithFields(fields).Warn("dcc-bus loco.subscribe rejected")
	} else {
		r.log.WithFields(fields).Debug("dcc-bus loco.subscribe")
	}
	resp.Subscribe(accepted...)

	for _, addr := range accepted {
		if snap, ok, err := r.redis.LoadState(ctx, addr); err == nil && ok {
			r.cache.Seed(addr, snap.Functions)
			_ = resp.SendLocoState(ctx, snap)
		}
	}
	return OKResult()
}
