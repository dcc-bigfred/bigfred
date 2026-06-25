package cmd

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
)

var _ remotes.InboundDrivePort = (*Router)(nil)

func toCommandResult(r Result) remotes.CommandResult {
	return remotes.CommandResult{OK: r.OK, Code: r.Code}
}

func (r *Router) AuthorizeDrive(userID uint, addr uint16, scope remotes.DriveScope) bool {
	return r.AuthorizeHandsetDrive(userID, addr, scope)
}

func (r *Router) SetSpeed(ctx context.Context, actor remotes.ThrottleActor, resp remotes.ThrottleResponder, req contract.LocoSetSpeedWire) remotes.CommandResult {
	return toCommandResult(r.HandleSetSpeed(ctx, throttleActor(actor), bridgeResponder(resp), req, ""))
}

func (r *Router) SetFunction(ctx context.Context, actor remotes.ThrottleActor, resp remotes.ThrottleResponder, req contract.LocoSetFunctionWire) remotes.CommandResult {
	return toCommandResult(r.HandleSetFunction(ctx, throttleActor(actor), bridgeResponder(resp), req, ""))
}

func (r *Router) Subscribe(ctx context.Context, actor remotes.ThrottleActor, resp remotes.ThrottleResponder, addrs []uint16) remotes.CommandResult {
	return toCommandResult(r.HandleSubscribe(ctx, throttleActor(actor), bridgeResponder(resp), protocol.LocoSubscribePayload{
		Addresses: addrs,
	}, ""))
}

func throttleActor(actor remotes.ThrottleActor) Actor {
	return Actor{UserID: actor.UserID, SessionID: actor.SessionID}
}

type throttleResponderBridge struct {
	remotes.ThrottleResponder
}

func bridgeResponder(resp remotes.ThrottleResponder) Responder {
	return throttleResponderBridge{ThrottleResponder: resp}
}

func (throttleResponderBridge) SendAck(ctx context.Context, requestID string, payload protocol.AckPayload) error {
	_ = ctx
	_ = requestID
	_ = payload
	return nil
}
