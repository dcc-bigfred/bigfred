package cmd

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service/station"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// HandleSessionClose runs the dead-man's plan when a browser session ends.
func (r *Router) HandleSessionClose(ctx context.Context, actor Actor, reason string) {
	if r.isLastSessionForUser(actor) {
		addrs := r.collectDriveTargetsForUser(ctx, actor.UserID, actor.ClosingSubscribedAddrs)
		r.log.WithFields(logrus.Fields{
			"sessionId": actor.SessionID,
			"userId":    actor.UserID,
			"reason":    reason,
			"addrs":     addrs,
		}).Info("dcc-bus last user session closed — emergency stop on drive targets")
		r.applyEmergencyStop(ctx, actor.UserID, actor.SessionID, addrs, reason, true)
		r.releaseUnusedSlots(addrs)
		return
	}
	if reason == errors.WsCodeSessionDeadman {
		addrs := r.collectSessionAddrs(actor.SessionID)
		r.applyEmergencyStop(ctx, actor.UserID, actor.SessionID, addrs, reason, true)
	}
}

// releaseUnusedSlots relinquishes the command-station slot for each loco the
// departing user was driving, so a slot is not held while nobody is connected.
// A slot is kept only when another live session is still subscribed to that
// loco — slots are owned per-locomotive by the server, so a co-driver's session
// must retain ownership. Released slots return to COMMON and can be reclaimed by
// a physical throttle or re-acquired on the next subscribe.
//
// Best-effort and asynchronous: each ReleaseSlot is a command-station write, so
// it must not block session teardown. Drivers without slots (e.g. Z21) do not
// implement SlotManager and are skipped.
func (r *Router) releaseUnusedSlots(addrs []uint16) {
	if len(addrs) == 0 {
		return
	}
	sm, ok := station.AsSlotManager(r.station)
	if !ok {
		return
	}
	targets := make([]uint16, 0, len(addrs))
	for _, addr := range addrs {
		if r.addrStillSubscribed(addr) {
			continue
		}
		targets = append(targets, addr)
	}
	if len(targets) == 0 {
		return
	}
	go func() {
		for _, addr := range targets {
			if err := sm.ReleaseSlot(commandstation.LocoAddr(addr)); err != nil {
				r.log.WithError(err).WithField("addr", addr).
					Debug("dcc-bus session close: slot release failed")
				continue
			}
			r.log.WithField("addr", addr).
				Info("dcc-bus released slot: no remaining sessions for loco")
		}
	}()
}

// addrStillSubscribed reports whether any live session is subscribed to addr.
func (r *Router) addrStillSubscribed(addr uint16) bool {
	for _, s := range r.hub.Snapshot() {
		for _, a := range s.SubscribedAddrs {
			if a == addr {
				return true
			}
		}
	}
	return false
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
		snap, ok, err := r.redis.GetLocoCurrentState(ctx, addr)
		if err != nil || !ok || snap.ControlledByUserID != userID {
			continue
		}
		add(&addrs, addr)
	}
	return addrs
}
