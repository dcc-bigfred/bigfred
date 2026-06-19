package cmd

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

// Actor identifies the authenticated user behind one WS session.
type Actor struct {
	UserID    uint
	SessionID string
	// ClosingSubscribedAddrs is populated only on session close. The WS layer
	// unregisters the session from the hub before the delayed dead-man fires,
	// so drive-target collection must not rely on the hub still listing this tab.
	ClosingSubscribedAddrs []uint16
}

// Responder sends protocol frames back to one connected client. The WS
// layer implements this port so cmd never imports ws.
type Responder interface {
	Subscribe(addrs ...uint16)
	SubscribedAddrs() []uint16
	SendLocoState(ctx context.Context, snap contract.LocoStateWire) error
	SendLocoError(ctx context.Context, addr uint16, code, detail string) error
	SendAck(ctx context.Context, requestID string, payload protocol.AckPayload) error
}

// SessionView is a snapshot of one live browser session used for fan-out
// and dead-man bookkeeping without importing the ws package.
type SessionView struct {
	ID              string
	UserID          uint
	SubscribedAddrs []uint16
}

// HubPort is the in-memory session registry the router uses for broadcast
// and multi-tab dead-man logic.
type HubPort interface {
	Broadcast(ctx context.Context, addr uint16, env contract.EnvelopeWire)
	SubscribedAddrs() []uint16
	SessionsForUser(userID uint) []SessionView
	UnsubscribeAll(addrs ...uint16)
	Snapshot() []SessionView
}
