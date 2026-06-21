package cmd

import (
	"context"
	"encoding/json"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

type TakeoverControlPort interface {
	Request(ctx context.Context, layoutID uint, signalman domain.User, target domain.TakeoverTarget, targetID string) (domain.TakeoverRequest, error)
	Reject(ctx context.Context, requestID, driverID uint) error
	Cancel(ctx context.Context, requestID, signalmanID uint) error
	Release(ctx context.Context, requestID, signalmanID uint, reason string) error
}

type TakeoverDeniedCodeFunc func(error) string

// TakeoverControl handles takeover.* on the control-plane WS.
type TakeoverControl struct {
	takeover   TakeoverControlPort
	deniedCode TakeoverDeniedCodeFunc
}

func NewTakeoverControl(takeover TakeoverControlPort, deniedCode TakeoverDeniedCodeFunc) *TakeoverControl {
	return &TakeoverControl{takeover: takeover, deniedCode: deniedCode}
}

func (h *TakeoverControl) HandleEnvelope(ctx context.Context, c ControlClient, env ControlEnvelope) bool {
	switch env.Type {
	case contract.TypeTakeoverRequest:
		h.handleRequest(ctx, c, env)
		return true
	case contract.TypeTakeoverReject:
		h.handleReject(ctx, c, env)
		return true
	case contract.TypeTakeoverCancel:
		h.handleCancel(ctx, c, env)
		return true
	case contract.TypeTakeoverRelease:
		h.handleRelease(ctx, c, env)
		return true
	default:
		return false
	}
}

func (h *TakeoverControl) handleRequest(ctx context.Context, c ControlClient, env ControlEnvelope) {
	if h.takeover == nil {
		c.SendAck(env.ID, false, "takeover_not_configured")
		return
	}
	var p contract.TakeoverRequestPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.SendAck(env.ID, false, "bad_payload")
		return
	}
	sess := c.Session()
	actor := domain.User{ID: sess.UserID(), Login: sess.Login()}
	_, err := h.takeover.Request(ctx, sess.LayoutID(), actor, p.Target, p.TargetID)
	if err != nil {
		c.SendAck(env.ID, false, h.errorCode(err))
		return
	}
	c.SendAck(env.ID, true, "")
}

func (h *TakeoverControl) handleReject(ctx context.Context, c ControlClient, env ControlEnvelope) {
	if h.takeover == nil {
		c.SendAck(env.ID, false, "takeover_not_configured")
		return
	}
	var p contract.TakeoverRequestIDPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.SendAck(env.ID, false, "bad_payload")
		return
	}
	if err := h.takeover.Reject(ctx, p.RequestID, c.Session().UserID()); err != nil {
		c.SendAck(env.ID, false, h.errorCode(err))
		return
	}
	c.SendAck(env.ID, true, "")
}

func (h *TakeoverControl) handleCancel(ctx context.Context, c ControlClient, env ControlEnvelope) {
	if h.takeover == nil {
		c.SendAck(env.ID, false, "takeover_not_configured")
		return
	}
	var p contract.TakeoverRequestIDPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.SendAck(env.ID, false, "bad_payload")
		return
	}
	if err := h.takeover.Cancel(ctx, p.RequestID, c.Session().UserID()); err != nil {
		c.SendAck(env.ID, false, h.errorCode(err))
		return
	}
	c.SendAck(env.ID, true, "")
}

func (h *TakeoverControl) handleRelease(ctx context.Context, c ControlClient, env ControlEnvelope) {
	if h.takeover == nil {
		c.SendAck(env.ID, false, "takeover_not_configured")
		return
	}
	var p contract.TakeoverRequestIDPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.SendAck(env.ID, false, "bad_payload")
		return
	}
	if err := h.takeover.Release(ctx, p.RequestID, c.Session().UserID(), "signalman_released"); err != nil {
		c.SendAck(env.ID, false, h.errorCode(err))
		return
	}
	c.SendAck(env.ID, true, "")
}

func (h *TakeoverControl) errorCode(err error) string {
	if h.deniedCode == nil {
		return "internal_error"
	}
	return h.deniedCode(err)
}
