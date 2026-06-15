package cmd

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const (
	// defaultPollInterval is used when --poll-interval was not set and
	// the driver cannot push state.
	defaultPollInterval = 750 * time.Millisecond

	// pollFnRange is the highest DCC function the polling fallback
	// reconciles. ListFunctions reports the active set; we expand it to
	// an explicit on/off vector over this range so an external throttle
	// turning a function OFF is detected, not just ON.
	pollFnRange = 28

	// subRefreshInterval is how often the push path re-subscribes the
	// command station to the locos that currently have live WS
	// subscribers. Re-subscribing is cheap and survives the Z21's
	// per-client subscription FIFO / client time-out.
	subRefreshInterval = 5 * time.Second
)

// RunStateFeed keeps Redis and connected WS clients in sync with state
// changes that originate OUTSIDE BigFred — a physical throttle plugged
// into the same command station as this daemon. The command station is
// only one of several possible controllers for a loco, so its bus is
// authoritative and we mirror whatever we observe.
//
// When the driver implements commandstation.StateObserver (e.g. the
// shared LocoNet bus) the feed consumes its push channel for real-time
// updates. Otherwise it falls back to polling GetSpeed / ListFunctions
// for the addresses that currently have at least one live subscriber.
//
// RunStateFeed blocks until ctx is cancelled and is meant to run in its
// own goroutine.
func (r *Router) RunStateFeed(ctx context.Context) {
	if obs, ok := r.station.(commandstation.StateObserver); ok {
		r.log.Info("dcc-bus state feed: driver supports push, consuming observations")
		r.runObserverFeed(ctx, obs)
		return
	}
	interval := r.statePollInterval()
	r.log.WithField("interval", interval).Info("dcc-bus state feed: driver has no push, falling back to polling")
	r.runPollFeed(ctx, interval)
}

func (r *Router) statePollInterval() time.Duration {
	if r.pollInterval > 0 {
		return r.pollInterval
	}
	return defaultPollInterval
}

func (r *Router) runObserverFeed(ctx context.Context, obs commandstation.StateObserver) {
	// Some push drivers (Z21) only emit unsolicited state for locos they
	// were explicitly subscribed to; without this, an external handset
	// moving a loco the daemon never queried stays invisible. Drivers on
	// a shared bus (LocoNet) don't implement this and need no refresh.
	if sub, ok := r.station.(commandstation.LocoInfoSubscriber); ok {
		go r.runSubscriptionRefresh(ctx, sub)
	}

	ch := obs.ObserveStates()
	for {
		select {
		case <-ctx.Done():
			return
		case o, ok := <-ch:
			if !ok {
				return
			}
			r.applyObservation(ctx, o, "external")
		}
	}
}

// runSubscriptionRefresh keeps the command station's per-loco
// subscriptions aligned with the locos that currently have a live WS
// subscriber, so external-throttle changes are pushed back even on Z21
// firmware without the FW≥1.24 "all locos" broadcast flag.
func (r *Router) runSubscriptionRefresh(ctx context.Context, sub commandstation.LocoInfoSubscriber) {
	ticker := time.NewTicker(subRefreshInterval)
	defer ticker.Stop()
	r.refreshSubscriptions(sub)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.refreshSubscriptions(sub)
		}
	}
}

func (r *Router) refreshSubscriptions(sub commandstation.LocoInfoSubscriber) {
	for _, addr := range r.hub.SubscribedAddrs() {
		if !r.roster.IsLocoAllowedOnLayout(addr) {
			continue
		}
		if err := sub.SubscribeLocoInfo(commandstation.LocoAddr(addr)); err != nil {
			r.log.WithError(err).WithField("addr", addr).
				Debug("dcc-bus state feed: subscribe loco-info failed")
		}
	}
}

func (r *Router) runPollFeed(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, addr := range r.hub.SubscribedAddrs() {
				if !r.roster.IsLocoAllowedOnLayout(addr) {
					continue
				}
				r.pollOne(ctx, addr)
			}
		}
	}
}

// pollOne reads the current speed/direction/functions for one address
// and reconciles them against the cached snapshot.
func (r *Router) pollOne(ctx context.Context, addr uint16) {
	speed, forward, err := r.station.GetSpeed(commandstation.LocoAddr(addr))
	if err != nil {
		r.log.WithError(err).WithField("addr", addr).Debug("dcc-bus state feed: GetSpeed failed")
		return
	}
	o := commandstation.LocoObservation{
		Addr:       commandstation.LocoAddr(addr),
		HasSpeed:   true,
		Speed:      speed,
		HasForward: true,
		Forward:    forward,
		Functions:  make(map[int]bool, pollFnRange+1),
	}
	if fns, err := r.station.ListFunctions(commandstation.LocoAddr(addr)); err == nil {
		active := make(map[int]bool, len(fns))
		for _, f := range fns {
			active[f] = true
		}
		for f := 0; f <= pollFnRange; f++ {
			o.Functions[f] = active[f]
		}
	}
	r.applyObservation(ctx, o, "poller")
}

// applyObservation merges a (possibly partial) observation onto the
// last known snapshot, and only stores + fans the result when it
// actually changes state. The change-guard also collapses the echo of
// BigFred's own writes (those already updated Redis with their original
// `source`/`controlledBy`, so the merged values match and we skip),
// preserving driver attribution for in-app commands while still
// surfacing genuine external changes.
func (r *Router) applyObservation(ctx context.Context, o commandstation.LocoObservation, source string) {
	addr := uint16(o.Addr)
	if !r.roster.IsLocoAllowedOnLayout(addr) {
		return
	}

	snap := contract.LocoStateWire{Address: addr}
	if cached, ok, err := r.redis.LoadState(ctx, addr); err == nil && ok {
		snap = cached
	}

	changed := false
	if o.HasSpeed {
		observed := uiSpeedFromWire(o.Speed)
		if snap.Speed != observed {
			snap.Speed = observed
			changed = true
		}
	}
	if o.HasForward && snap.Forward != o.Forward {
		snap.Forward = o.Forward
		changed = true
	}
	for fn, on := range o.Functions {
		if fn < 0 {
			continue
		}
		if len(snap.Functions) <= fn {
			grown := make([]bool, fn+1)
			copy(grown, snap.Functions)
			snap.Functions = grown
		}
		if snap.Functions[fn] != on {
			snap.Functions[fn] = on
			changed = true
			// Keep the function dedup cache honest after an external
			// change so the next in-app toggle isn't wrongly collapsed.
			r.fnCache.Set(addr, uint8(fn), on)
		}
	}
	if !changed {
		return
	}

	// The change came from off-app, so nobody in BigFred owns it.
	snap.ControlledByUserID = 0
	snap.Source = source
	snap.At = time.Now().UTC().UnixMilli()

	if err := r.redis.StoreState(ctx, snap, stateTTL); err != nil {
		r.log.WithError(err).Debug("dcc-bus state feed: redis store")
	}
	r.broadcastLocoStateToObservers(ctx, snap)
	r.log.WithFields(logrus.Fields{
		"addr":    addr,
		"speed":   snap.Speed,
		"forward": snap.Forward,
		"source":  source,
	}).Debug("dcc-bus state feed: external change applied")
}
