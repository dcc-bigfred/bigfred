package cmd

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

type stubHub struct {
	sessions []SessionView
}

func (h *stubHub) Broadcast(context.Context, uint16, contract.EnvelopeWire) {}
func (h *stubHub) SubscribedAddrs() []uint16                               { return nil }
func (h *stubHub) UnsubscribeAll(...uint16)                                {}
func (h *stubHub) Snapshot() []SessionView                                 { return append([]SessionView(nil), h.sessions...) }
func (h *stubHub) SessionsForUser(userID uint) []SessionView {
	out := make([]SessionView, 0, len(h.sessions))
	for _, s := range h.sessions {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out
}

func TestIsLastSessionForUser_reconnectSkipsDMS(t *testing.T) {
	t.Parallel()
	r := &Router{hub: &stubHub{}}
	old := Actor{UserID: 42, SessionID: "old-tab"}

	if !r.isLastSessionForUser(old) {
		t.Fatal("no live sessions: closing tab should be last for user")
	}

	r.hub = &stubHub{sessions: []SessionView{
		{ID: "new-tab", UserID: 42},
	}}
	if r.isLastSessionForUser(old) {
		t.Fatal("reconnected tab: old session close must not trigger layout-wide DMS")
	}
}
