package cmd

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
	"github.com/keskad/loco/pkgs/loco/commandstation"
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

func (r *Router) isHandsetControlledLoco(_ context.Context, userID uint, addr uint16) bool {
	if r.store == nil {
		return true
	}
	snap := r.store.Snapshot(addr)
	if snap.Source == "" && snap.At == 0 && snap.Speed == 0 {
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

// TriggerStationTrackPowerOn turns main-track power on for this daemon's
// command station when the hardware supports it (LocoNet GPON, Z21
// LAN_X_SET_TRACK_POWER_ON). It does not fan out to other stations on the
// same layout; radio stop remains the layout-wide path.
func (r *Router) TriggerStationTrackPowerOn(_ context.Context, userID uint, source string) error {
	if r == nil || r.station == nil {
		return commandstation.ErrTrackPowerUnsupported
	}
	ctrl, ok := r.station.(commandstation.TrackPowerController)
	if !ok {
		return commandstation.ErrTrackPowerUnsupported
	}
	if err := ctrl.SetTrackPower(true); err != nil {
		return err
	}
	if r.log != nil {
		r.log.WithFields(logrus.Fields{
			"userId": userID,
			"source": source,
		}).Info("dcc-bus track power on")
	}
	return nil
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
