package cmd

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service/station"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const syncFnRange = 28

// forceRevalidateSlot drops any debounce window and re-queries the command
// station for addr's slot mapping. Best-effort: callers retry drive commands
// after this when the first attempt may have hit a stale cached slot.
func (r *Router) forceRevalidateSlot(addr uint16) {
	sm, ok := station.AsSlotManager(r.station)
	if !ok {
		return
	}
	if err := sm.ForceAcquireSlot(commandstation.LocoAddr(addr)); err != nil {
		r.log.WithError(err).WithField("addr", addr).
			Debug("dcc-bus slot force revalidate failed")
	}
}

// syncLocoStateFromBus reads speed and functions from the command station and
// publishes them to Redis/WS so the UI matches the track after slot reclaim.
func (r *Router) syncLocoStateFromBus(ctx context.Context, addr uint16) {
	speed, forward, err := r.station.GetSpeed(commandstation.LocoAddr(addr))
	if err != nil {
		r.log.WithError(err).WithField("addr", addr).
			Debug("dcc-bus sync from bus: GetSpeed failed")
		return
	}

	snap := contract.LocoStateWire{
		Address: addr,
		Speed:   service.UISpeedFromWire(speed),
		Forward: forward,
		Source:  "bus-sync",
		At:      time.Now().UTC().UnixMilli(),
	}
	if cached, ok, err := r.redis.GetLocoCurrentState(ctx, addr); err == nil && ok {
		snap.ControlledByUserID = cached.ControlledByUserID
		snap.Functions = append([]bool(nil), cached.Functions...)
	}

	active := map[int]bool{}
	if fns, err := r.station.ListFunctions(commandstation.LocoAddr(addr)); err == nil {
		for _, fn := range fns {
			active[fn] = true
		}
	}
	if len(snap.Functions) < syncFnRange+1 {
		grown := make([]bool, syncFnRange+1)
		copy(grown, snap.Functions)
		snap.Functions = grown
	}
	for fn := 0; fn <= syncFnRange; fn++ {
		on := active[fn]
		snap.Functions[fn] = on
		r.cache.Set(addr, uint8(fn), on)
	}

	if err := r.redis.StoreLocoCurrentState(ctx, snap, StateTTL); err != nil {
		r.log.WithError(err).WithField("addr", addr).Debug("dcc-bus sync from bus: redis store")
	}
	service.BroadcastLocoState(ctx, r.hub, snap)
	r.log.WithFields(logrus.Fields{
		"addr":    addr,
		"speed":   snap.Speed,
		"forward": snap.Forward,
	}).Debug("dcc-bus synced loco state from command station")
}
