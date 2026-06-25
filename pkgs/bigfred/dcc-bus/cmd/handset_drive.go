package cmd

import (
	"context"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
)

// CollectHandsetDriveTargets returns locomotive addresses that should be
// emergency-stopped when a handset goes idle.
func (r *Router) CollectHandsetDriveTargets(
	ctx context.Context,
	userID uint,
	subscribed []uint16,
	scope remotes.DriveScope,
) []uint16 {
	return r.collectHandsetDriveTargets(ctx, userID, subscribed, scope.AllowedAddrs, scope.AllowAllVehicles)
}

func (r *Router) collectHandsetDriveTargets(
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
			if r.isHandsetControlledLoco(ctx, userID, addr) {
				add(&addrs, addr)
			}
		}
		return addrs
	}
	for _, addr := range allowed {
		if r.isHandsetControlledLoco(ctx, userID, addr) {
			add(&addrs, addr)
		}
	}
	return addrs
}

func (r *Router) isHandsetControlledLoco(ctx context.Context, userID uint, addr uint16) bool {
	if r.redis == nil {
		return true
	}
	snap, ok, err := r.redis.GetLocoCurrentState(ctx, addr)
	if err != nil || !ok {
		return false
	}
	return snap.ControlledByUserID == userID && snap.Speed > 0
}

// ApplyHandsetIdleBrake emergency-stops moving locos under one handset.
func (r *Router) ApplyHandsetIdleBrake(ctx context.Context, session remotes.HandsetSession, subscribed []uint16, scope remotes.DriveScope) {
	addrs := r.collectHandsetDriveTargets(ctx, session.UserID, subscribed, scope.AllowedAddrs, scope.AllowAllVehicles)
	if len(addrs) == 0 {
		return
	}
	r.applyEmergencyStop(ctx, session.UserID, remotes.HandsetSessionID(session.ClientKey), addrs, "handset_idle", true)
}

// ApplyHandsetPilotEStop emergency-stops one locomotive from a handset estop.
func (r *Router) ApplyHandsetPilotEStop(ctx context.Context, session remotes.HandsetSession, addr uint16) {
	if addr == 0 {
		return
	}
	r.applyEmergencyStop(ctx, session.UserID, remotes.HandsetSessionID(session.ClientKey), []uint16{addr}, "handset_estop", false)
}

// TriggerLayoutRadioStop publishes the layout-wide radio stop command.
func (r *Router) TriggerLayoutRadioStop(ctx context.Context, userID uint, source string) error {
	if r == nil || r.redis == nil {
		return nil
	}
	return r.redis.PublishLayoutRadioStop(ctx, contract.RadioStopCommandWire{
		TriggeredByUserID: userID,
		TriggeredByLogin:  source,
		At:                time.Now().UTC().UnixMilli(),
	})
}

// AuthorizeHandsetDrive checks handset vehicle scope and roster drive policy.
func (r *Router) AuthorizeHandsetDrive(userID uint, addr uint16, scope remotes.DriveScope) bool {
	return r.authorizeHandsetDrive(userID, addr, scope.AllowedAddrs, scope.AllowAllVehicles)
}

func (r *Router) authorizeHandsetDrive(userID uint, addr uint16, allowedAddrs []uint16, allowAll bool) bool {
	vehicle, onLayout := r.roster.AllowedVehicle(addr)
	if !onLayout {
		return false
	}
	if allowAll {
		return r.drive.CanDrive(userID, vehicle, true).Allowed
	}
	for _, a := range allowedAddrs {
		if a == addr {
			return r.drive.CanDrive(userID, vehicle, true).Allowed
		}
	}
	return false
}
