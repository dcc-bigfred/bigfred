package cmd

import "context"

// enforceSingleVehicleControl stops the user's other moving vehicles (speed > 1)
// when single-vehicle control is enabled. activeAddrs are the addresses of the
// vehicle or consist the user is taking control of; they are not braked.
func (r *Router) enforceSingleVehicleControl(ctx context.Context, actor Actor, activeAddrs ...uint16) {
	if r == nil || !r.singleVehicleControl {
		return
	}
	skip := make(map[uint16]struct{}, len(activeAddrs))
	for _, addr := range activeAddrs {
		skip[addr] = struct{}{}
	}
	for _, addr := range r.roster.AllowedAddrs() {
		if _, ok := skip[addr]; ok {
			continue
		}
		snap := r.store.Snapshot(addr)
		if snap.ControlledByUserID != actor.UserID || snap.Speed <= 1 {
			continue
		}
		if err := r.applyMemberSetSpeed(ctx, actor, addr, 0, snap.Forward, false, "single_vehicle_control", ""); err != nil {
			r.log.WithError(err).WithField("addr", addr).Warn("dcc-bus single vehicle control brake failed")
		}
	}
}
