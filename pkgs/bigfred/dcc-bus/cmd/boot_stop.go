package cmd

import (
	"context"

	"github.com/sirupsen/logrus"
)

// ensureBootStop sends speed 0 to every roster locomotive once per daemon
// lifetime. It runs after the first non-empty allowed_vehicles snapshot —
// either loaded from Redis at NewRouter or received later from loco-server.
func (r *Router) ensureBootStop(ctx context.Context) {
	r.bootStopMu.Lock()
	if r.bootStopDone {
		r.bootStopMu.Unlock()
		return
	}
	addrs := r.roster.AllowedAddrs()
	if len(addrs) == 0 {
		r.bootStopMu.Unlock()
		return
	}
	r.bootStopDone = true
	r.bootStopMu.Unlock()

	if r.log != nil {
		r.log.WithFields(logrus.Fields{
			"layoutId": r.layoutID,
			"addrs":    addrs,
			"count":    len(addrs),
		}).Info("dcc-bus startup — stopping all layout locomotives after daemon restart")
	}
	r.applyEStopAll(ctx, "daemon_startup")
}
