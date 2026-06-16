package ws

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/cmd"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

type sessionResponder struct {
	sess *Session
}

// NewResponder adapts a Session to cmd.Responder.
func NewResponder(sess *Session) cmd.Responder {
	return &sessionResponder{sess: sess}
}

func (r *sessionResponder) Subscribe(addrs ...uint16) {
	r.sess.Subscribe(addrs...)
}

func (r *sessionResponder) SubscribedAddrs() []uint16 {
	return r.sess.SubscribedAddrs()
}

func (r *sessionResponder) SendLocoState(ctx context.Context, snap contract.LocoStateWire) error {
	return r.sess.SendTyped(ctx, protocol.TypeLocoState, snap)
}

func (r *sessionResponder) SendLocoError(ctx context.Context, addr uint16, code, detail string) error {
	return r.sess.SendTyped(ctx, protocol.TypeLocoError, protocol.LocoErrorPayload{
		Address: addr,
		Code:    code,
		Detail:  detail,
	})
}

func (r *sessionResponder) SendAck(ctx context.Context, requestID string, payload protocol.AckPayload) error {
	return r.sess.SendAckData(ctx, requestID, payload)
}
