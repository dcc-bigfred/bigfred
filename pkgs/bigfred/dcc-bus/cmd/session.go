package cmd

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
)

// HandleSessionClose runs the dead-man's plan when a browser session ends.
func (r *Router) HandleSessionClose(ctx context.Context, actor Actor, reason string) {
	if r.isLastSessionForUser(actor) {
		addrs := r.collectDriveTargetsForUser(ctx, actor.UserID)
		r.log.WithFields(logrus.Fields{
			"sessionId": actor.SessionID,
			"userId":    actor.UserID,
			"reason":    reason,
			"addrs":     addrs,
		}).Info("dcc-bus last user session closed — emergency stop on drive targets")
		r.applyEmergencyStop(ctx, actor.UserID, actor.SessionID, addrs, reason, true)
		return
	}
	if reason == errors.WsCodeSessionDeadman {
		addrs := r.collectSessionAddrs(actor.SessionID)
		r.applyEmergencyStop(ctx, actor.UserID, actor.SessionID, addrs, reason, true)
	}
}

func (r *Router) isLastSessionForUser(actor Actor) bool {
	for _, s := range r.hub.SessionsForUser(actor.UserID) {
		if s.ID != actor.SessionID {
			return false
		}
	}
	return true
}

func (r *Router) collectSessionAddrs(sessionID string) []uint16 {
	for _, s := range r.hub.Snapshot() {
		if s.ID == sessionID {
			return append([]uint16(nil), s.SubscribedAddrs...)
		}
	}
	return nil
}

func (r *Router) collectDriveTargetsForUser(ctx context.Context, userID uint) []uint16 {
	seen := make(map[uint16]struct{}, 8)
	add := func(out *[]uint16, addr uint16) {
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		*out = append(*out, addr)
	}
	var addrs []uint16
	for _, s := range r.hub.SessionsForUser(userID) {
		for _, addr := range s.SubscribedAddrs {
			add(&addrs, addr)
		}
	}
	for _, addr := range r.roster.AllowedAddrs() {
		snap, ok, err := r.redis.LoadState(ctx, addr)
		if err != nil || !ok || snap.ControlledByUserID != userID {
			continue
		}
		add(&addrs, addr)
	}
	return addrs
}
