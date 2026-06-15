package service

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// CompositeControlHandler delegates control-plane WS frames to the
// first child that handles them.
type CompositeControlHandler struct {
	session  *SessionControlService
	radio    *RadioControlService
	takeover *TakeoverControlService
}

// NewCompositeControlHandler wires session, radio and takeover handlers.
func NewCompositeControlHandler(
	session *SessionControlService,
	radio *RadioControlService,
	takeover *TakeoverControlService,
) *CompositeControlHandler {
	return &CompositeControlHandler{
		session:  session,
		radio:    radio,
		takeover: takeover,
	}
}

func (c *CompositeControlHandler) HandleOpened(ctx context.Context, client *ws.Client) {
	if c.session != nil {
		c.session.HandleOpened(ctx, client)
	}
}

func (c *CompositeControlHandler) HandleClosed(ctx context.Context, client *ws.Client) {
	if c.session != nil {
		c.session.HandleClosed(ctx, client)
	}
}

func (c *CompositeControlHandler) HandleEnvelope(ctx context.Context, client *ws.Client, env ws.Envelope) {
	if c.takeover != nil && c.takeover.HandleEnvelope(ctx, client, env) {
		return
	}
	if c.radio != nil && c.radio.HandleEnvelope(ctx, client, env) {
		return
	}
	if c.session != nil {
		c.session.HandleEnvelope(ctx, client, env)
	}
}
