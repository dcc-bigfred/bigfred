package service

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

// StateBroadcaster fans loco.state envelopes to live WS sessions.
type StateBroadcaster interface {
	Broadcast(ctx context.Context, addr uint16, env contract.EnvelopeWire)
}

// BroadcastLocoState fans one snapshot to every subscribed WS session.
func BroadcastLocoState(ctx context.Context, hub StateBroadcaster, snap contract.LocoStateWire) {
	if hub == nil {
		return
	}
	env, err := protocol.Frame(protocol.TypeLocoState, snap)
	if err != nil {
		return
	}
	hub.Broadcast(ctx, snap.Address, env)
}
