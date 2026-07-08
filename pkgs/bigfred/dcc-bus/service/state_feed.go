package service

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service/station"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/state"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const (
	defaultPollInterval = 750 * time.Millisecond
	pollFnRange         = 28
	subRefreshInterval  = 5 * time.Second
)

// FeedDeps are the inputs required to mirror external throttle changes.
type FeedDeps struct {
	Station       commandstation.Station
	Roster        *RosterCache
	Store         *state.LocoStateStore
	Hub           StateBroadcaster
	HubSubs      SubscriptionSource
	FnCache      *FunctionsCache
	LocoObservers *remotes.LocoStateNotifier
	Log           *logrus.Logger
	PollInterval time.Duration
	StateTTL     time.Duration
}

// SubscriptionSource exposes the union of subscribed locomotive addresses.
type SubscriptionSource interface {
	SubscribedAddrs() []uint16
	IsSubscribed(addr uint16) bool
}

// RunStateFeed keeps Redis and connected WS clients in sync with state
// changes that originate outside BigFred. Blocks until ctx is cancelled.
func RunStateFeed(ctx context.Context, deps FeedDeps) {
	if obs, ok := station.AsStateObserver(deps.Station); ok {
		if deps.Log != nil {
			deps.Log.Info("dcc-bus state feed: driver supports push, consuming observations")
		}
		runObserverFeed(ctx, deps, obs)
		return
	}
	interval := deps.pollInterval()
	if deps.Log != nil {
		deps.Log.WithField("interval", interval).Info("dcc-bus state feed: driver has no push, falling back to polling")
	}
	runPollFeed(ctx, deps, interval)
}

func (d FeedDeps) pollInterval() time.Duration {
	if d.PollInterval > 0 {
		return d.PollInterval
	}
	return defaultPollInterval
}

func runObserverFeed(ctx context.Context, deps FeedDeps, obs commandstation.StateObserver) {
	if sub, ok := station.AsLocoInfoSubscriber(deps.Station); ok {
		go runSubscriptionRefresh(ctx, deps, sub)
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
			applyObservation(ctx, deps, o, "external")
		}
	}
}

func runSubscriptionRefresh(ctx context.Context, deps FeedDeps, sub commandstation.LocoInfoSubscriber) {
	ticker := time.NewTicker(subRefreshInterval)
	defer ticker.Stop()
	refreshSubscriptions(deps, sub)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshSubscriptions(deps, sub)
		}
	}
}

func refreshSubscriptions(deps FeedDeps, sub commandstation.LocoInfoSubscriber) {
	if deps.HubSubs == nil || deps.Roster == nil {
		return
	}
	for _, addr := range deps.HubSubs.SubscribedAddrs() {
		if !deps.Roster.IsOnLayout(addr) {
			continue
		}
		if err := sub.SubscribeLocoInfo(commandstation.LocoAddr(addr)); err != nil && deps.Log != nil {
			deps.Log.WithError(err).WithField("addr", addr).
				Debug("dcc-bus state feed: subscribe loco-info failed")
		}
	}
}

func runPollFeed(ctx context.Context, deps FeedDeps, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if deps.HubSubs == nil {
				continue
			}
			for _, addr := range deps.HubSubs.SubscribedAddrs() {
				if deps.Roster != nil && !deps.Roster.IsOnLayout(addr) {
					continue
				}
				pollOne(ctx, deps, addr)
			}
		}
	}
}

func pollOne(ctx context.Context, deps FeedDeps, addr uint16) {
	speed, forward, err := deps.Station.GetSpeed(commandstation.LocoAddr(addr))
	if err != nil {
		if deps.Log != nil {
			deps.Log.WithError(err).WithField("addr", addr).Debug("dcc-bus state feed: GetSpeed failed")
		}
		return
	}
	o := commandstation.LocoObservation{
		Addr:       commandstation.LocoAddr(addr),
		HasSpeed:   true,
		Speed:      speed,
		HasForward: true,
		Forward:    forward,
	}
	if fns, err := deps.Station.ListFunctions(commandstation.LocoAddr(addr)); err == nil {
		active := make(map[int]bool, len(fns))
		for _, f := range fns {
			active[f] = true
		}
		for f := 0; f <= pollFnRange; f++ {
			o.FunctionMask |= 1 << uint(f)
			if active[f] {
				o.FunctionBits |= 1 << uint(f)
			}
		}
	}
	applyObservation(ctx, deps, o, "poller")
}

func applyObservation(ctx context.Context, deps FeedDeps, o commandstation.LocoObservation, source string) {
	addr := uint16(o.Addr)
	if deps.Roster != nil && !deps.Roster.IsOnLayout(addr) {
		return
	}

	noWS := deps.HubSubs != nil && !deps.HubSubs.IsSubscribed(addr)
	noHandset := deps.LocoObservers == nil || !deps.LocoObservers.AnyRegistered()
	if noWS && noHandset {
		if o.FunctionMask != 0 && deps.FnCache != nil {
			for fn := 0; fn <= pollFnRange; fn++ {
				bit := uint32(1) << uint(fn)
				if o.FunctionMask&bit == 0 {
					continue
				}
				deps.FnCache.Set(addr, uint8(fn), o.FunctionBits&bit != 0)
			}
		}
		return
	}

	if deps.Store == nil {
		return
	}

	snap, changed := deps.Store.ApplyObservation(o, source)
	if !changed {
		return
	}
	if o.FunctionMask != 0 && deps.FnCache != nil {
		for fn := 0; fn <= pollFnRange; fn++ {
			bit := uint32(1) << uint(fn)
			if o.FunctionMask&bit == 0 {
				continue
			}
			deps.FnCache.Set(addr, uint8(fn), o.FunctionBits&bit != 0)
		}
	}
	BroadcastLocoState(ctx, deps.Hub, snap)
	if deps.LocoObservers != nil {
		deps.LocoObservers.Notify(ctx, snap, "")
	}
	if deps.Log != nil {
		deps.Log.WithFields(logrus.Fields{
			"addr":    addr,
			"speed":   snap.Speed,
			"forward": snap.Forward,
			"source":  source,
		}).Debug("dcc-bus state feed: external change applied")
	}
}
