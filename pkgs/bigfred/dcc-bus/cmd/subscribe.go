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
	r.reclaimSlotOwnership(accepted)

	for _, addr := range accepted {
		snap := r.locoSnapOrDefault(ctx, addr)
		if len(snap.Functions) > 0 {
			r.cache.Seed(addr, snap.Functions)
		}
		_ = resp.SendLocoState(ctx, snap)
	}
	return OKResult()
}

// reclaimSlotOwnership re-claims the command-station slot for each loco and
// syncs Redis/UI from the bus. Slots are owned per-locomotive by the server,
// not per session: a client leaving and returning to the throttle must not
// lose control just because the command station purged the idle slot to COMMON
// or reassigned it. This occupies the slot at the server level and runs
// independently of (and before) the drive-permission layer enforced in
// HandleSetSpeed — viewing is enough to keep BigFred's ownership warm.
//
// Best-effort and asynchronous: each ForceAcquireSlot is a command-station round
// trip, so it must not block the subscribe ack. Drivers without slots (e.g.
// Z21) do not implement SlotManager and are skipped.
func (r *Router) reclaimSlotOwnership(addrs []uint16) {
	if len(addrs) == 0 {
		return
	}
	sm, ok := station.AsSlotManager(r.station)
	if !ok {
		return
	}
	targets := append([]uint16(nil), addrs...)
	go func() {
		ctx := context.Background()
		for _, addr := range targets {
			r.cache.ClearAddr(addr)
			if err := sm.ForceAcquireSlot(commandstation.LocoAddr(addr)); err != nil {
				r.log.WithError(err).WithField("addr", addr).
					Warn("dcc-bus slot reclaim failed")
				continue
			}
			r.syncLocoStateFromBus(ctx, addr)
		}
	}()
}

// reclaimSlotsStillSubscribed revalidates locos from a closing session that
// remain subscribed elsewhere (e.g. load-test disconnect while the throttle
// tab stays open). Without this, a stale LocoNet slot mapping from the
// departing client can leave co-subscribers unable to drive.
func (r *Router) reclaimSlotsStillSubscribed(closingAddrs []uint16) {
	targets := make([]uint16, 0, len(closingAddrs))
	for _, addr := range closingAddrs {
		if r.addrStillSubscribed(addr) {
			targets = append(targets, addr)
		}
	}
	r.reclaimSlotOwnership(targets)
}
