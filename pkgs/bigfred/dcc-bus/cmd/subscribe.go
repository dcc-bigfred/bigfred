package cmd

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/security"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service/station"
	"github.com/keskad/loco/pkgs/loco/commandstation"
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
	r.ensureSlotOwnership(accepted)

	for _, addr := range accepted {
		if snap, ok, err := r.redis.GetLocoCurrentState(ctx, addr); err == nil && ok {
			r.cache.Seed(addr, snap.Functions)
			_ = resp.SendLocoState(ctx, snap)
		}
	}
	return OKResult()
}

// ensureSlotOwnership re-claims the command-station slot for each freshly
// subscribed loco. Slots are owned per-locomotive by the server, not per
// session: a client leaving and returning to the throttle must not lose
// control just because the command station purged the idle slot to COMMON or
// reassigned it. This occupies the slot at the server level and runs
// independently of (and before) the drive-permission layer enforced in
// HandleSetSpeed — viewing is enough to keep BigFred's ownership warm.
//
// Best-effort and asynchronous: each AcquireSlot is a command-station round
// trip, so it must not block the subscribe ack. Drivers without slots (e.g.
// Z21) do not implement SlotManager and are skipped.
func (r *Router) ensureSlotOwnership(addrs []uint16) {
	if len(addrs) == 0 {
		return
	}
	sm, ok := station.AsSlotManager(r.station)
	if !ok {
		return
	}
	targets := append([]uint16(nil), addrs...)
	go func() {
		for _, addr := range targets {
			if err := sm.AcquireSlot(commandstation.LocoAddr(addr)); err != nil {
				r.log.WithError(err).WithField("addr", addr).
					Debug("dcc-bus subscribe: slot acquire failed")
			}
		}
	}()
}
