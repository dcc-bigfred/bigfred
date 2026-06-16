package cmd

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// HandleRadioStop runs the hybrid halt (§4.6.1a).
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

// HandleLayoutRadioStopMessage decodes a layout-wide radio_stop pub/sub payload.
func (r *Router) HandleLayoutRadioStopMessage(ctx context.Context, raw []byte) {
	var cmdWire contract.RadioStopCommandWire
	if err := json.Unmarshal(raw, &cmdWire); err != nil {
		r.log.WithError(err).Debug("dcc-bus radio stop: bad payload")
		return
	}
	r.log.WithFields(logrus.Fields{
		"triggeredByUserId": cmdWire.TriggeredByUserID,
		"triggeredByLogin":  cmdWire.TriggeredByLogin,
	}).Info("dcc-bus radio stop received")
	r.HandleRadioStop(ctx)
}
