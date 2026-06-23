package cmd

import (
	"context"
	"time"
)

// Shutdown stops every locomotive on the layout, cancels pending train
// consist commands, and releases command-station resources. It must run
// before the process exits (SIGTERM, supervisord stop, or daemon Close).
//
// LocoNet: emergency stop is sent for each roster loco, then OPC_SLOT_STAT1
// COMMON for every cached slot before the transport closes.
func (r *Router) Shutdown() {
	r.shutdownOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if r.trainSpeed != nil {
			r.trainSpeed.CancelAll()
		}

		if r.log != nil {
			r.log.Info("dcc-bus shutting down — emergency stop on all layout locomotives")
		}
		r.applyEStopAll(ctx, "daemon_shutdown")

		if err := r.station.CleanUp(); err != nil && r.log != nil {
			r.log.WithError(err).Warn("dcc-bus shutdown: station cleanup failed")
		}
	})
}
