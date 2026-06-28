package cmd

import (
	"context"

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
// publishes them to WS so the UI matches the track after slot reclaim.
func (r *Router) syncLocoStateFromBus(ctx context.Context, addr uint16) {
	speed, forward, err := r.station.GetSpeed(commandstation.LocoAddr(addr))
	if err != nil {
		r.log.WithError(err).WithField("addr", addr).
			Debug("dcc-bus sync from bus: GetSpeed failed")
		return
	}

	prev := r.store.Snapshot(addr)
	fns := append([]bool(nil), prev.Functions...)
	active := map[int]bool{}
	if listed, err := r.station.ListFunctions(commandstation.LocoAddr(addr)); err == nil {
		for _, fn := range listed {
			active[fn] = true
		}
	}
	if len(fns) < syncFnRange+1 {
		grown := make([]bool, syncFnRange+1)
		copy(grown, fns)
		fns = grown
	}
	for fn := 0; fn <= syncFnRange; fn++ {
		on := active[fn]
		fns[fn] = on
		r.cache.Set(addr, uint8(fn), on)
	}

	snap := r.store.SetFromBus(addr, contract.UISpeedFromWire(speed), forward, fns, prev.ControlledByUserID)
	service.BroadcastLocoState(ctx, r.hub, snap)
	r.log.WithFields(logrus.Fields{
		"addr":    addr,
		"speed":   snap.Speed,
		"forward": snap.Forward,
	}).Debug("dcc-bus synced loco state from command station")
}
