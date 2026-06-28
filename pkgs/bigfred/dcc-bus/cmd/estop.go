package cmd

import (
	"context"
	stderrors "errors"
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

func (r *Router) isLocoPlacedForward(_ context.Context, addr uint16) bool {
	return r.store.Snapshot(addr).Forward
}

func (r *Router) applyEmergencyStop(ctx context.Context, userID uint, sessionID string, addrs []uint16, reason string, movingOnly bool) {
	affected := make([]uint16, 0, len(addrs))
	for _, addr := range addrs {
		if !r.shouldEmergencyStopLoco(addr, movingOnly) {
			continue
		}
		forward := r.isLocoPlacedForward(ctx, addr)
		if err := emergencyStopLoco(r.station, addr, forward, uint8(r.speedSteps)); err != nil {
			if stderrors.Is(err, commandstation.ErrSpeedSuperseded) {
				continue
			}
			r.log.WithError(err).WithField("addr", addr).Warn("dcc-bus estop failed")
			continue
		}
		affected = append(affected, addr)
		snap := r.store.SetSpeed(addr, 0, forward, userID, "estop")
		r.store.FlushNow(ctx, addr)
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

func (r *Router) shouldEmergencyStopLoco(addr uint16, movingOnly bool) bool {
	cached := r.store.Snapshot(addr)
	if cached.Source == "" && cached.At == 0 && cached.Speed == 0 {
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
		if !r.shouldEmergencyStopLoco(addr, true) {
			continue
		}
		forward := r.isLocoPlacedForward(ctx, addr)
		if err := emergencyStopLoco(r.station, addr, forward, uint8(r.speedSteps)); err != nil {
			if stderrors.Is(err, commandstation.ErrSpeedSuperseded) {
				continue
			}
			r.log.WithError(err).WithField("addr", addr).Warn("dcc-bus estop target failed")
			continue
		}
		affected = append(affected, addr)
		snap := r.store.SetSpeedPreservingUser(addr, 0, forward, "estop")
		r.store.FlushNow(ctx, addr)
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
		if err := emergencyStopLoco(r.station, addr, forward, uint8(r.speedSteps)); err != nil {
			if stderrors.Is(err, commandstation.ErrSpeedSuperseded) {
				continue
			}
		}
		snap := r.store.SetSpeedPreservingUser(addr, 0, forward, "estop")
		r.store.FlushNow(ctx, addr)
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

func emergencyStopLoco(station commandstation.Station, addr uint16, forward bool, speedSteps uint8) error {
	la := commandstation.LocoAddr(addr)
	if estopper, ok := station.(commandstation.EmergencyStopper); ok {
		return estopper.EmergencyStop(la, forward)
	}
	return station.SetSpeed(la, 1, forward, speedSteps)
}
