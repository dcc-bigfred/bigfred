package cmd

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
)

// HandleSessionClose runs the dead-man's plan when a browser session ends.
func (r *Router) HandleSessionClose(ctx context.Context, actor Actor, reason string) {
	if r.leaser != nil {
		r.leaser.ReleaseSession(actor.SessionID)
	}
	if r.isLastSessionForUser(actor) {
		addrs := r.collectDriveTargetsForUser(ctx, actor.UserID, actor.ClosingSubscribedAddrs)
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

// ReleaseHandsetSession drops every slot lease held by a remote handset
// (Z21 / WiThrottle) when the client disconnects or is evicted.
func (r *Router) ReleaseHandsetSession(sessionID string) {
	if r == nil || r.leaser == nil || sessionID == "" {
		return
	}
	r.leaser.ReleaseSession(sessionID)
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

func (r *Router) collectDriveTargetsForUser(ctx context.Context, userID uint, extraAddrs []uint16) []uint16 {
	seen := make(map[uint16]struct{}, 8)
	add := func(out *[]uint16, addr uint16) {
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		*out = append(*out, addr)
	}
	var addrs []uint16
	for _, addr := range extraAddrs {
		add(&addrs, addr)
	}
	for _, s := range r.hub.SessionsForUser(userID) {
		for _, addr := range s.SubscribedAddrs {
			add(&addrs, addr)
		}
	}
	for _, addr := range r.roster.AllowedAddrs() {
		snap := r.store.Snapshot(addr)
		if snap.ControlledByUserID != userID {
			continue
		}
		add(&addrs, addr)
	}
	return addrs
}
