package service

import (
	"context"
	"encoding/json"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// TakeoverControlService handles takeover.* on the control-plane WS.
type TakeoverControlService struct {
	takeover *TakeoverService
}

// NewTakeoverControlService returns a ready WS adapter.
func NewTakeoverControlService(takeover *TakeoverService) *TakeoverControlService {
	return &TakeoverControlService{takeover: takeover}
}

// HandleEnvelope dispatches takeover WS actions.
func (h *TakeoverControlService) HandleEnvelope(ctx context.Context, c *ws.Client, env ws.Envelope) bool {
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

func (h *TakeoverControlService) handleRequest(ctx context.Context, c *ws.Client, env ws.Envelope) {
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
	actor := domain.User{ID: sess.UserID, Login: sess.Login}
	_, err := h.takeover.Request(ctx, sess.LayoutID, actor, p.Target, p.TargetID)
	if err != nil {
		c.SendAck(env.ID, false, TakeoverDeniedCode(err))
		return
	}
	c.SendAck(env.ID, true, "")
}

func (h *TakeoverControlService) handleReject(ctx context.Context, c *ws.Client, env ws.Envelope) {
	if h.takeover == nil {
		c.SendAck(env.ID, false, "takeover_not_configured")
		return
	}
	var p contract.TakeoverRequestIDPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.SendAck(env.ID, false, "bad_payload")
		return
	}
	if err := h.takeover.Reject(ctx, p.RequestID, c.Session().UserID); err != nil {
		c.SendAck(env.ID, false, TakeoverDeniedCode(err))
		return
	}
	c.SendAck(env.ID, true, "")
}

func (h *TakeoverControlService) handleCancel(ctx context.Context, c *ws.Client, env ws.Envelope) {
	if h.takeover == nil {
		c.SendAck(env.ID, false, "takeover_not_configured")
		return
	}
	var p contract.TakeoverRequestIDPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.SendAck(env.ID, false, "bad_payload")
		return
	}
	if err := h.takeover.Cancel(ctx, p.RequestID, c.Session().UserID); err != nil {
		c.SendAck(env.ID, false, TakeoverDeniedCode(err))
		return
	}
	c.SendAck(env.ID, true, "")
}

func (h *TakeoverControlService) handleRelease(ctx context.Context, c *ws.Client, env ws.Envelope) {
	if h.takeover == nil {
		c.SendAck(env.ID, false, "takeover_not_configured")
		return
	}
	var p contract.TakeoverRequestIDPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.SendAck(env.ID, false, "bad_payload")
		return
	}
	if err := h.takeover.Release(ctx, p.RequestID, c.Session().UserID, "signalman_released"); err != nil {
		c.SendAck(env.ID, false, TakeoverDeniedCode(err))
		return
	}
	c.SendAck(env.ID, true, "")
}
