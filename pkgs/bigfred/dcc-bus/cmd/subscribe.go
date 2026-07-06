package cmd

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/security"
)

const defaultSubscriptionCap = 8

func (r *Router) subscriptionCap() int {
	if r == nil || r.maxVehiclesPerUser <= 0 {
		return defaultSubscriptionCap
	}
	return r.maxVehiclesPerUser
}

// enforceSubscriptionCap adds incoming addresses while respecting the per-session
// subscription limit (D16). When full, the oldest subscription is dropped and
// subscription_cap is emitted for the evicted address.
func (r *Router) enforceSubscriptionCap(ctx context.Context, resp Responder, incoming []uint16) {
	cap := r.subscriptionCap()
	known := make(map[uint16]struct{}, len(incoming))
	for _, a := range resp.SubscribedAddrs() {
		known[a] = struct{}{}
	}
	for _, addr := range incoming {
		if _, ok := known[addr]; ok {
			continue
		}
		for len(resp.SubscribedAddrs()) >= cap {
			oldest, ok := resp.OldestSubscribed()
			if !ok {
				break
			}
			resp.Unsubscribe(oldest)
			delete(known, oldest)
			_ = resp.SendLocoError(ctx, oldest, errors.CodeSubscriptionCap, "")
			r.slotMetrics.RecordSubscribeCap()
			r.log.WithFields(logrus.Fields{
				"evictedAddr": oldest,
				"incoming":    addr,
				"cap":         cap,
			}).Debug("dcc-bus subscription cap: dropped oldest")
		}
		resp.Subscribe(addr)
		known[addr] = struct{}{}
	}
}

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
	r.enforceSubscriptionCap(ctx, resp, accepted)

	for _, addr := range accepted {
		snap := r.locoSnapOrDefault(ctx, addr)
		if len(snap.Functions) > 0 {
			r.cache.Seed(addr, snap.Functions)
		}
		_ = resp.SendLocoState(ctx, snap)
	}
	return OKResult()
}
