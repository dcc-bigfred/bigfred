package cmd

import (
	"context"
	"encoding/json"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

type RadioControlPort interface {
	Send(ctx context.Context, in RadioSendInput) (domain.RadioMessage, error)
	ReplayInterlocking(ctx context.Context, layoutID, interlockingID, callerUserID uint, limit int) ([]domain.RadioMessage, error)
	ReplayUser(ctx context.Context, layoutID, userID uint, limit int) ([]domain.RadioMessage, error)
	ReplayLimit() int
}

type RadioDeniedCodeFunc func(error) string

type RadioSendInput struct {
	LayoutID         uint
	FromUserID       uint
	FromLogin        string
	ToUserID         uint
	ToInterlockingID uint
	ContextVehicleID domain.VehicleID
	ContextTrainID   domain.TrainID
	Phrase           domain.RadioPhrase
	Note             string
}

// RadioControl handles radio.send and radio.replay on the control-plane WS.
type RadioControl struct {
	radio      RadioControlPort
	deniedCode RadioDeniedCodeFunc
}

func NewRadioControl(radio RadioControlPort, deniedCode RadioDeniedCodeFunc) *RadioControl {
	return &RadioControl{radio: radio, deniedCode: deniedCode}
}

func (h *RadioControl) HandleEnvelope(ctx context.Context, c ControlClient, env ControlEnvelope) bool {
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

func (h *RadioControl) handleSend(ctx context.Context, c ControlClient, env ControlEnvelope) {
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
	var ctxVehicle domain.VehicleID
	var ctxTrain domain.TrainID
	if p.Context.VehicleID != nil {
		if id, ok := domain.ParseVehicleID(*p.Context.VehicleID); ok {
			ctxVehicle = id
		}
	}
	if p.Context.TrainID != nil {
		if id, ok := domain.ParseTrainID(*p.Context.TrainID); ok {
			ctxTrain = id
		}
	}

	_, err := h.radio.Send(ctx, RadioSendInput{
		LayoutID:         sess.LayoutID(),
		FromUserID:       sess.UserID(),
		FromLogin:        sess.Login(),
		ToUserID:         toUser,
		ToInterlockingID: toIlk,
		ContextVehicleID: ctxVehicle,
		ContextTrainID:   ctxTrain,
		Phrase:           p.Phrase,
		Note:             p.Note,
	})
	if err != nil {
		c.SendAck(env.ID, false, h.errorCode(err))
		return
	}
	c.SendAck(env.ID, true, "")
}

func (h *RadioControl) handleReplay(ctx context.Context, c ControlClient, env ControlEnvelope) {
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
	if limit <= 0 {
		limit = h.radio.ReplayLimit()
	}

	var (
		rows []domain.RadioMessage
		err  error
	)
	switch p.Scope {
	case "interlocking":
		rows, err = h.radio.ReplayInterlocking(ctx, sess.LayoutID(), p.InterlockingID, sess.UserID(), limit)
	case "user":
		rows, err = h.radio.ReplayUser(ctx, sess.LayoutID(), sess.UserID(), limit)
	default:
		c.SendAck(env.ID, false, "bad_payload")
		return
	}
	if err != nil {
		c.SendAck(env.ID, false, h.errorCode(err))
		return
	}

	wires := make([]contract.RadioMessageWire, 0, len(rows))
	for _, row := range rows {
		wires = append(wires, contract.MessageWireFromDomain(row))
	}
	c.SendTyped(contract.TypeRadioHistory, contract.RadioHistoryWire{Messages: wires})
	c.SendAck(env.ID, true, "")
}

func (h *RadioControl) errorCode(err error) string {
	if h.deniedCode == nil {
		return "internal_error"
	}
	return h.deniedCode(err)
}
