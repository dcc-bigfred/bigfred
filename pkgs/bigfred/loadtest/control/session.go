// Package control drives loco-server control-plane WebSocket actions needed
// before the data-plane load test connects through the reverse proxy.
package control

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/loadtest/wsutil"
)

const (
	typeSessionSetCommandStation = "session.setCommandStation"
	typeAck                      = "ack"
	setCommandStationAckTimeout  = 45 * time.Second
)

// StartCommandStation opens /api/v1/ws and sends session.setCommandStation so
// loco-server spawns (or verifies) the matching dcc-bus daemon before the
// load test dials the reverse proxy.
func StartCommandStation(ctx context.Context, httpBase, token string, commandStationID uint, log *logrus.Logger) error {
	if commandStationID == 0 {
		return fmt.Errorf("command-station-id is required")
	}
	if log == nil {
		log = logrus.New()
	}

	wsURL, err := wsutil.ControlWS(httpBase)
	if err != nil {
		return err
	}
	dialURL, err := wsutil.WithToken(wsURL, token)
	if err != nil {
		return err
	}

	conn, _, err := websocket.Dial(ctx, dialURL, nil)
	if err != nil {
		return fmt.Errorf("control websocket dial: %w", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "load test") }()

	requestID := uuid.NewString()
	payload, err := json.Marshal(map[string]uint{"commandStationId": commandStationID})
	if err != nil {
		return err
	}
	frame, err := json.Marshal(map[string]any{
		"type":    typeSessionSetCommandStation,
		"id":      requestID,
		"payload": json.RawMessage(payload),
	})
	if err != nil {
		return err
	}
	if err := conn.Write(ctx, websocket.MessageText, frame); err != nil {
		return fmt.Errorf("send session.setCommandStation: %w", err)
	}

	ackCtx, cancel := context.WithTimeout(ctx, setCommandStationAckTimeout)
	defer cancel()

	for {
		_, data, err := conn.Read(ackCtx)
		if err != nil {
			return fmt.Errorf("wait session.setCommandStation ack: %w", err)
		}
		var env struct {
			Type    string          `json:"type"`
			ID      string          `json:"id"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		if env.Type != typeAck || env.ID != requestID {
			continue
		}
		var ack struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if len(env.Payload) > 0 {
			_ = json.Unmarshal(env.Payload, &ack)
		}
		if !ack.OK {
			if ack.Error != "" {
				return fmt.Errorf("session.setCommandStation: %s", ack.Error)
			}
			return fmt.Errorf("session.setCommandStation rejected")
		}
		log.WithField("commandStationId", commandStationID).Info("command station ready via loco-server")
		return nil
	}
}
