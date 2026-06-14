package cmd

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// HandleRadioStop runs the hybrid halt (§4.6.1a): roster-wide DCC
// emergency stop on this command station, then each connected driver's
// dead-man's-switch plan against their drive targets on this daemon.
func (r *Router) HandleRadioStop(ctx context.Context) {
	addrs := r.applyEStopAll(ctx, "radio_stop")

	seen := make(map[uint]struct{}, 4)
	for _, sess := range r.hub.Snapshot() {
		if _, ok := seen[sess.UserID]; ok {
			continue
		}
		seen[sess.UserID] = struct{}{}
		targets := r.collectDriveTargetsForUser(ctx, sess.UserID)
		if len(targets) == 0 {
			continue
		}
		r.applyEmergencyStop(ctx, sess.UserID, "", targets, "radio_stop", true)
	}

	_ = r.redis.Publish(ctx, "system.radio_stop.ack", contract.RadioStopAckWire{
		CommandStationID: r.commandStationID,
		Addrs:            addrs,
		At:               time.Now().UTC().UnixMilli(),
	})
}

// HandleLayoutRadioStopMessage decodes a payload from the layout-wide
// radio_stop pub/sub channel and runs the local halt.
func (r *Router) HandleLayoutRadioStopMessage(ctx context.Context, raw []byte) {
	var cmd contract.RadioStopCommandWire
	if err := json.Unmarshal(raw, &cmd); err != nil {
		r.log.WithError(err).Debug("dcc-bus radio stop: bad payload")
		return
	}
	r.log.WithFields(logrus.Fields{
		"triggeredByUserId": cmd.TriggeredByUserID,
		"triggeredByLogin":  cmd.TriggeredByLogin,
	}).Info("dcc-bus radio stop received")
	r.HandleRadioStop(ctx)
}
