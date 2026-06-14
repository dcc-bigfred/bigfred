package cmd

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/ws"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// HandleEStop slams every loco the requesting session subscribes to
// down to speed step 1 (DCC EMG-stop) and emits a system.estop
// audit event on the Redis bus.
func (r *Router) HandleEStop(ctx context.Context, sess *ws.Session, p protocol.SystemEStopPayload, requestID string) {
	r.applyEmergencyForSession(ctx, sess, p.Reason, false)
	if requestID != "" {
		_ = sess.SendAck(ctx, requestID, true, "")
	}
}

// applyEmergencyForSession brakes every loco the session subscribed
// to. We emit one EMG-stop per address so a partial failure (the
// command station rejected one address) doesn't abort the rest.
func (r *Router) applyEmergencyForSession(ctx context.Context, sess *ws.Session, reason string, movingOnly bool) {
	r.applyEmergencyStop(ctx, sess.UserID, sess.ID, sess.SubscribedAddrs(), reason, movingOnly)
}

// applyEmergencyStop issues EMG-stop for each address and publishes
// the audit frame. Shared by per-session and per-user last-session
// paths.
//
// When movingOnly is true (dead-man's switch paths), a loco is acted
// on only when its cached speed is above 1. When movingOnly is false
// (manual estop), locos already at normal stop (speed 0) are skipped
// so a benign page navigation does not surface wire speed 1 in the UI.
func (r *Router) applyEmergencyStop(ctx context.Context, userID uint, sessionID string, addrs []uint16, reason string, movingOnly bool) {
	affected := make([]uint16, 0, len(addrs))
	for _, addr := range addrs {
		if !r.shouldEmergencyStopLoco(ctx, addr, movingOnly) {
			continue
		}
		if err := r.station.SetSpeed(commandstation.LocoAddr(addr), 1, true, uint8(r.speedSteps)); err != nil {
			r.log.WithError(err).WithField("addr", addr).Warn("dcc-bus estop failed")
			continue
		}
		affected = append(affected, addr)
		snap := contract.LocoStateWire{
			Address:            addr,
			Speed:              1,
			Forward:            true,
			ControlledByUserID: userID,
			Source:             "estop",
			At:                 time.Now().UTC().UnixMilli(),
		}
		if cached, ok, err := r.redis.LoadState(ctx, addr); err == nil && ok {
			snap.Functions = cached.Functions
			snap.Forward = cached.Forward
		}
		if err := r.redis.StoreState(ctx, snap, stateTTL); err != nil {
			r.log.WithError(err).Debug("dcc-bus estop redis store")
		}
		r.broadcastLocoStateToObservers(ctx, snap)
		if v, ok := r.roster.AllowedVehicle(addr); ok {
			r.applyDeadManSwitchForLoco(context.Background(), addr, userID, v)
		}
	}
	if len(affected) == 0 {
		return
	}

	if err := r.redis.Publish(ctx, "system.estop.audit", map[string]any{
		"reason":    reason,
		"userId":    userID,
		"sessionId": sessionID,
		"addrs":     affected,
		"at":        time.Now().UTC().UnixMilli(),
	}); err != nil {
		r.log.WithError(err).Debug("dcc-bus estop publish")
	}
}

func (r *Router) shouldEmergencyStopLoco(ctx context.Context, addr uint16, movingOnly bool) bool {
	cached, ok, err := r.redis.LoadState(ctx, addr)
	if err != nil || !ok {
		return !movingOnly
	}
	if movingOnly {
		return cached.Speed > 1
	}
	return cached.Speed != 0
}

func (r *Router) applyDeadManSwitchForLoco(ctx context.Context, addr uint16, userID uint, v contract.AllowedVehicle) {
	switch v.DeadManSwitchOption {
	case string(domain.DeadManSwitchStopHorn):
		r.setTimedLocoFunctionWithRetry(addr, userID, v.Rp1Function, time.Second, "deadman", 3)
	case string(domain.DeadManSwitchStopHornEmergencyLights):
		r.setTimedLocoFunctionWithRetry(addr, userID, v.Rp1Function, time.Second, "deadman", 3)
		if err := r.setLocoFunction(ctx, addr, userID, v.EmergencyLightsFunction, true, "deadman"); err != nil {
			r.log.WithError(err).WithFields(logrus.Fields{
				"addr":     addr,
				"function": v.EmergencyLightsFunction,
			}).Warn("dcc-bus dead-man emergency lights failed")
		}
	default:
		// "stop" and unknown values: brake only.
	}
}

// applyEStopAll brakes every roster locomotive on this command station.
// Returns the roster addresses included in the halt (for audit ack).
func (r *Router) applyEStopAll(ctx context.Context, reason string) []uint16 {
	addrs := r.roster.AllowedAddrs()
	for _, addr := range addrs {
		_ = r.station.SetSpeed(commandstation.LocoAddr(addr), 1, true, uint8(r.speedSteps))
		snap := contract.LocoStateWire{
			Address: addr,
			Speed:   0,
			Forward: true,
			Source:  "estop",
			At:      time.Now().UTC().UnixMilli(),
		}
		if cached, ok, err := r.redis.LoadState(ctx, addr); err == nil && ok {
			snap.Functions = cached.Functions
		}
		_ = r.redis.StoreState(ctx, snap, stateTTL)
		r.broadcastLocoStateToObservers(ctx, snap)
	}
	_ = r.redis.Publish(ctx, "system.estop.audit", map[string]any{
		"reason": reason,
		"scope":  "all",
		"addrs":  addrs,
		"at":     time.Now().UTC().UnixMilli(),
	})
	return addrs
}
