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
	// Source is the lease holder source: "ws" for browser throttle, or the
	// inbound protocol name ("z21", "withrottle") for remote handsets.
	Source string
	// ClosingSubscribedAddrs is populated only on session close. The WS layer
	// unregisters the session from the hub before the delayed dead-man fires,
	// so drive-target collection must not rely on the hub still listing this tab.
	ClosingSubscribedAddrs []uint16
}

// LeaseSource returns the slot-lease holder source for this actor.
func (a Actor) LeaseSource() string {
	if a.Source == "" {
		return "ws"
	}
	return a.Source
}

// Responder sends protocol frames back to one connected client. The WS
// layer implements this port so cmd never imports ws.
type Responder interface {
	Subscribe(addrs ...uint16)
	Unsubscribe(addrs ...uint16)
	SubscribedAddrs() []uint16
	OldestSubscribed() (uint16, bool)
	SelectedAddr() uint16
	SetSelected(addr uint16)
	ClearSelected()
	SendLocoState(ctx context.Context, snap contract.LocoStateWire) error
	SendLocoError(ctx context.Context, addr uint16, code, detail string) error
	SendLocoErrorPayload(ctx context.Context, p protocol.LocoErrorPayload) error
	SendAck(ctx context.Context, requestID string, payload protocol.AckPayload) error
}

// noopResponder satisfies Responder for commands that arrive without a
// client to answer — the Redis control channel is fire-and-forget.
type noopResponder struct{}

func (noopResponder) Subscribe(...uint16)              {}
func (noopResponder) Unsubscribe(...uint16)            {}
func (noopResponder) SubscribedAddrs() []uint16        { return nil }
func (noopResponder) OldestSubscribed() (uint16, bool) { return 0, false }
func (noopResponder) SelectedAddr() uint16             { return 0 }
func (noopResponder) SetSelected(uint16)               {}
func (noopResponder) ClearSelected()                   {}

func (noopResponder) SendLocoState(context.Context, contract.LocoStateWire) error { return nil }
func (noopResponder) SendLocoError(context.Context, uint16, string, string) error { return nil }
func (noopResponder) SendLocoErrorPayload(context.Context, protocol.LocoErrorPayload) error {
	return nil
}
func (noopResponder) SendAck(context.Context, string, protocol.AckPayload) error { return nil }

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
	IsSubscribed(addr uint16) bool
	SessionsForUser(userID uint) []SessionView
	UnsubscribeAll(addrs ...uint16)
	Snapshot() []SessionView
}
