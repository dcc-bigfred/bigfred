package cmd

import (
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service/station"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

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
