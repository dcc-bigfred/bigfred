package ws

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/cmd"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

// RouterAdapter bridges cmd.Router to the ws.Router interface.
type RouterAdapter struct {
	inner *cmd.Router
}

// NewRouterAdapter wraps a command router for the WS server.
func NewRouterAdapter(inner *cmd.Router) *RouterAdapter {
	return &RouterAdapter{inner: inner}
}

func (a *RouterAdapter) HandleSubscribe(ctx context.Context, sess *Session, payload protocol.LocoSubscribePayload, requestID string) Outcome {
	return deliver(ctx, sess, requestID, a.inner.HandleSubscribe(ctx, actor(sess), NewResponder(sess), payload, requestID))
}

func (a *RouterAdapter) HandleSetSpeed(ctx context.Context, sess *Session, payload contract.LocoSetSpeedWire, requestID string) Outcome {
	return deliver(ctx, sess, requestID, a.inner.HandleSetSpeed(ctx, actor(sess), NewResponder(sess), payload, requestID))
}

func (a *RouterAdapter) HandleTrainSetSpeed(ctx context.Context, sess *Session, payload contract.TrainSetSpeedWire, requestID string) Outcome {
	return deliver(ctx, sess, requestID, a.inner.HandleTrainSetSpeed(ctx, actor(sess), NewResponder(sess), payload, requestID))
}

func (a *RouterAdapter) HandleSetFunction(ctx context.Context, sess *Session, payload contract.LocoSetFunctionWire, requestID string) Outcome {
	return deliver(ctx, sess, requestID, a.inner.HandleSetFunction(ctx, actor(sess), NewResponder(sess), payload, requestID))
}

func (a *RouterAdapter) HandleEStop(ctx context.Context, sess *Session, payload protocol.SystemEStopPayload, requestID string) Outcome {
	return deliver(ctx, sess, requestID, a.inner.HandleEStop(ctx, actor(sess), NewResponder(sess), payload, requestID))
}

func (a *RouterAdapter) HandleSessionClose(ctx context.Context, sess *Session, reason string) {
	a.inner.HandleSessionClose(ctx, actor(sess), reason)
}

func actor(sess *Session) cmd.Actor {
	return cmd.Actor{UserID: sess.UserID, SessionID: sess.ID}
}

func deliver(ctx context.Context, sess *Session, requestID string, res cmd.Result) Outcome {
	if len(res.Members) > 0 && requestID != "" {
		ack := protocol.AckPayload{OK: res.OK, Error: res.Code, Members: res.Members}
		if err := NewResponder(sess).SendAck(ctx, requestID, ack); err != nil {
			return Fail(errors.WsCodeSendFailed)
		}
	} else if requestID != "" {
		if err := sess.SendAck(ctx, requestID, res.OK, res.Code); err != nil {
			return Fail(errors.WsCodeSendFailed)
		}
	}
	if res.OK {
		return OK()
	}
	if res.Code == "" {
		return OK()
	}
	return Fail(res.Code)
}
