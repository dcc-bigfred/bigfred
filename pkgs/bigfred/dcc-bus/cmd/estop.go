package cmd

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// HandleEStop slams every loco the requesting session subscribes to down to
// speed step 1 and emits a system.estop audit event on the Redis bus.
func (r *Router) HandleEStop(ctx context.Context, actor Actor, resp Responder, p protocol.SystemEStopPayload, _ string) Result {
	r.applyEmergencyForSession(ctx, actor, resp, p.Reason, false)
	return OKResult()
}

func (r *Router) applyEmergencyForSession(ctx context.Context, actor Actor, resp Responder, reason string, movingOnly bool) {
	r.applyEmergencyStop(ctx, actor.UserID, actor.SessionID, resp.SubscribedAddrs(), reason, movingOnly)
}

func (r *Router) isLocoPlacedForward(ctx context.Context, addr uint16) bool {
	cached, ok, err := r.redis.GetLocoCurrentState(ctx, addr)
	if err == nil && ok {
		return cached.Forward
	}
	return true
}

func (r *Router) applyEmergencyStop(ctx context.Context, userID uint, sessionID string, addrs []uint16, reason string, movingOnly bool) {
	affected := make([]uint16, 0, len(addrs))
	for _, addr := range addrs {
		if !r.shouldEmergencyStopLoco(ctx, addr, movingOnly) {
			continue
		}
		forward := r.isLocoPlacedForward(ctx, addr)
		if err := r.station.SetSpeed(commandstation.LocoAddr(addr), 1, forward, uint8(r.speedSteps)); err != nil {
			r.log.WithError(err).WithField("addr", addr).Warn("dcc-bus estop failed")
			continue
		}
		affected = append(affected, addr)
		snap := contract.LocoStateWire{
			Address:            addr,
			Speed:              0,
			Forward:            forward,
			ControlledByUserID: userID,
			Source:             "estop",
			At:                 time.Now().UTC().UnixMilli(),
		}
		if cached, ok, err := r.redis.GetLocoCurrentState(ctx, addr); err == nil && ok {
			snap.Functions = cached.Functions
		}
		if err := r.redis.StoreLocoCurrentState(ctx, snap, StateTTL); err != nil {
			r.log.WithError(err).Debug("dcc-bus estop redis store")
		}
		service.BroadcastLocoState(ctx, r.hub, snap)
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
	cached, ok, err := r.redis.GetLocoCurrentState(ctx, addr)
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
	}
}

func (r *Router) applyEStopTarget(ctx context.Context, addrs []uint16) {
	affected := make([]uint16, 0, len(addrs))
	for _, addr := range addrs {
		if !r.roster.IsOnLayout(addr) {
			continue
		}
		if !r.shouldEmergencyStopLoco(ctx, addr, true) {
			continue
		}
		forward := r.isLocoPlacedForward(ctx, addr)
		if err := r.station.SetSpeed(commandstation.LocoAddr(addr), 1, forward, uint8(r.speedSteps)); err != nil {
			r.log.WithError(err).WithField("addr", addr).Warn("dcc-bus estop target failed")
			continue
		}
		affected = append(affected, addr)
		snap := contract.LocoStateWire{
			Address: addr,
			Speed:   0,
			Forward: forward,
			Source:  "estop",
			At:      time.Now().UTC().UnixMilli(),
		}
		if cached, ok, err := r.redis.GetLocoCurrentState(ctx, addr); err == nil && ok {
			snap.Functions = cached.Functions
			snap.ControlledByUserID = cached.ControlledByUserID
		}
		_ = r.redis.StoreLocoCurrentState(ctx, snap, StateTTL)
		service.BroadcastLocoState(ctx, r.hub, snap)
	}
	if len(affected) == 0 {
		return
	}
	_ = r.redis.Publish(ctx, "system.estop.audit", map[string]any{
		"reason": "estop_target",
		"scope":  "target",
		"addrs":  affected,
		"at":     time.Now().UTC().UnixMilli(),
	})
}

func (r *Router) applyEStopAll(ctx context.Context, reason string) []uint16 {
	addrs := r.roster.AllowedAddrs()
	for _, addr := range addrs {
		forward := r.isLocoPlacedForward(ctx, addr)
		_ = r.station.SetSpeed(commandstation.LocoAddr(addr), 1, forward, uint8(r.speedSteps))
		snap := contract.LocoStateWire{
			Address: addr,
			Speed:   0,
			Forward: forward,
			Source:  "estop",
			At:      time.Now().UTC().UnixMilli(),
		}
		if cached, ok, err := r.redis.GetLocoCurrentState(ctx, addr); err == nil && ok {
			snap.Functions = cached.Functions
		}
		_ = r.redis.StoreLocoCurrentState(ctx, snap, StateTTL)
		service.BroadcastLocoState(ctx, r.hub, snap)
	}
	_ = r.redis.Publish(ctx, "system.estop.audit", map[string]any{
		"reason": reason,
		"scope":  "all",
		"addrs":  addrs,
		"at":     time.Now().UTC().UnixMilli(),
	})
	return addrs
}
