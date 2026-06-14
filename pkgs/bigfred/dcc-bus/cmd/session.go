package cmd

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/ws"
)

// HandleSessionClose runs the dead-man's plan and fires audit. Called
// from the WS server when a browser session goes away (any reason).
func (r *Router) HandleSessionClose(ctx context.Context, sess *ws.Session, reason string) {
	if r.isLastSessionForUser(sess) {
		addrs := r.collectDriveTargetsForUser(ctx, sess.UserID)
		r.log.WithFields(logrus.Fields{
			"sessionId": sess.ID,
			"userId":    sess.UserID,
			"reason":    reason,
			"addrs":     addrs,
		}).Info("dcc-bus last user session closed — emergency stop on drive targets")
		r.applyEmergencyStop(ctx, sess.UserID, sess.ID, addrs, reason, true)
		return
	}
	if reason == errors.WsCodeSessionDeadman {
		r.applyEmergencyForSession(ctx, sess, reason, true)
	}
}

// isLastSessionForUser reports whether sess is the only live WS
// session for its user on this daemon (§7e.5 per-daemon rule).
func (r *Router) isLastSessionForUser(sess *ws.Session) bool {
	for _, s := range r.hub.SessionsForUser(sess.UserID) {
		if s.ID != sess.ID {
			return false
		}
	}
	return true
}
