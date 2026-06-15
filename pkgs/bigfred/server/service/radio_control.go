package service

import (
	"context"
	"encoding/json"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// RadioControlService handles radio.send and radio.replay on the
// control-plane WebSocket (§4.2).
type RadioControlService struct {
	radio *RadioService
}

// NewRadioControlService returns a ready WS adapter.
func NewRadioControlService(radio *RadioService) *RadioControlService {
	return &RadioControlService{radio: radio}
}

// HandleEnvelope dispatches radio WS actions.
func (h *RadioControlService) HandleEnvelope(ctx context.Context, c *ws.Client, env ws.Envelope) bool {
	switch env.Type {
	case contract.TypeRadioSend:
		h.handleSend(ctx, c, env)
		return true
	case contract.TypeRadioReplay:
		h.handleReplay(ctx, c, env)
		return true
	default:
		return false
	}
}

func (h *RadioControlService) handleSend(ctx context.Context, c *ws.Client, env ws.Envelope) {
	if h.radio == nil {
		c.SendAck(env.ID, false, "radio_not_configured")
		return
	}
	var p contract.RadioSendPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.SendAck(env.ID, false, "bad_payload")
		return
	}
	sess := c.Session()
	var toUser, toIlk uint
	if p.To.UserID != nil {
		toUser = *p.To.UserID
	}
	if p.To.InterlockingID != nil {
		toIlk = *p.To.InterlockingID
	}
	var ctxVehicle, ctxTrain uint
	if p.Context.VehicleID != nil {
		ctxVehicle = *p.Context.VehicleID
	}
	if p.Context.TrainID != nil {
		ctxTrain = *p.Context.TrainID
	}

	_, err := h.radio.Send(ctx, SendInput{
		LayoutID:         sess.LayoutID,
		FromUserID:       sess.UserID,
		FromLogin:        sess.Login,
		ToUserID:         toUser,
		ToInterlockingID: toIlk,
		ContextVehicleID: ctxVehicle,
		ContextTrainID:   ctxTrain,
		Phrase:           p.Phrase,
		Note:             p.Note,
	})
	if err != nil {
		c.SendAck(env.ID, false, RadioDeniedCode(err))
		return
	}
	c.SendAck(env.ID, true, "")
}

func (h *RadioControlService) handleReplay(ctx context.Context, c *ws.Client, env ws.Envelope) {
	if h.radio == nil {
		c.SendAck(env.ID, false, "radio_not_configured")
		return
	}
	var p contract.RadioReplayPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.SendAck(env.ID, false, "bad_payload")
		return
	}
	sess := c.Session()
	limit := p.Limit
	if limit <= 0 && h.radio.store != nil {
		limit = h.radio.store.ReplayLimit()
	}

	var (
		rows []domain.RadioMessage
		err  error
	)
	switch p.Scope {
	case "interlocking":
		rows, err = h.radio.ReplayInterlocking(ctx, sess.LayoutID, p.InterlockingID, sess.UserID, limit)
	case "user":
		rows, err = h.radio.ReplayUser(ctx, sess.LayoutID, sess.UserID, limit)
	default:
		c.SendAck(env.ID, false, "bad_payload")
		return
	}
	if err != nil {
		c.SendAck(env.ID, false, RadioDeniedCode(err))
		return
	}

	wires := make([]contract.RadioMessageWire, 0, len(rows))
	for _, row := range rows {
		wires = append(wires, contract.MessageWireFromDomain(row))
	}
	c.SendTyped(contract.TypeRadioHistory, contract.RadioHistoryWire{Messages: wires})
	c.SendAck(env.ID, true, "")
}
