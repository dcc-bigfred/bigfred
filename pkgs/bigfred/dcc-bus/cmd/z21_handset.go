package cmd

import (
	"context"
)

// CollectZ21PilotDriveTargets returns locomotive addresses that should be
// emergency-stopped when a Z21 handset goes idle.
func (r *Router) CollectZ21PilotDriveTargets(
	ctx context.Context,
	userID uint,
	subscribed []uint16,
	allowed []uint16,
	allowAll bool,
) []uint16 {
	seen := make(map[uint16]struct{}, 8)
	add := func(out *[]uint16, addr uint16) {
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		*out = append(*out, addr)
	}
	var addrs []uint16
	for _, addr := range subscribed {
		add(&addrs, addr)
	}
	if allowAll {
		for _, addr := range r.roster.AllowedAddrs() {
			if r.isPilotControlledLoco(ctx, userID, addr) {
				add(&addrs, addr)
			}
		}
		return addrs
	}
	for _, addr := range allowed {
		if r.isPilotControlledLoco(ctx, userID, addr) {
			add(&addrs, addr)
		}
	}
	return addrs
}

func (r *Router) isPilotControlledLoco(ctx context.Context, userID uint, addr uint16) bool {
	if r.redis == nil {
		return true
	}
	snap, ok, err := r.redis.GetLocoCurrentState(ctx, addr)
	if err != nil || !ok {
		return false
	}
	return snap.ControlledByUserID == userID && snap.Speed > 0
}

// ApplyZ21HandsetIdleBrake emergency-stops moving locos under one handset.
func (r *Router) ApplyZ21HandsetIdleBrake(ctx context.Context, userID uint, clientKey string, addrs []uint16) {
	if len(addrs) == 0 {
		return
	}
	r.applyEmergencyStop(ctx, userID, "z21:"+clientKey, addrs, "z21_handset_idle", true)
}
