package ws

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/cmd"
)

type hubAdapter struct {
	h *Hub
}

// HubPort wraps a Hub as cmd.HubPort for the command router.
func HubPort(h *Hub) cmd.HubPort {
	return &hubAdapter{h: h}
}

func (a *hubAdapter) Broadcast(ctx context.Context, addr uint16, env contract.EnvelopeWire) {
	a.h.Broadcast(ctx, addr, env)
}

func (a *hubAdapter) SubscribedAddrs() []uint16 {
	return a.h.SubscribedAddrs()
}

func (a *hubAdapter) IsSubscribed(addr uint16) bool {
	return a.h.IsSubscribed(addr)
}

func (a *hubAdapter) SessionsForUser(userID uint) []cmd.SessionView {
	return sessionViews(a.h.SessionsForUser(userID))
}

func (a *hubAdapter) UnsubscribeAll(addrs ...uint16) {
	a.h.UnsubscribeAll(addrs...)
}

func (a *hubAdapter) Snapshot() []cmd.SessionView {
	return sessionViews(a.h.Snapshot())
}

func sessionViews(sessions []*Session) []cmd.SessionView {
	out := make([]cmd.SessionView, len(sessions))
	for i, s := range sessions {
		out[i] = cmd.SessionView{
			ID:              s.ID,
			UserID:          s.UserID,
			SubscribedAddrs: s.SubscribedAddrs(),
		}
	}
	return out
}
