package ws

import (
	"context"
	"sync"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// Hub is the in-memory registry of every Session currently connected
// to one dcc-bus daemon. The struct is tiny on purpose — the daemon
// keeps state in Redis, the Hub only fans events out to live WS
// connections.
type Hub struct {
	mu       sync.RWMutex
	sessions map[*Session]struct{}
}

// NewHub returns an empty Hub.
func NewHub() *Hub {
	return &Hub{sessions: make(map[*Session]struct{}, 4)}
}

// Register adds a session to the live set.
func (h *Hub) Register(s *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[s] = struct{}{}
}

// Unregister removes a session from the live set. Safe to call
// multiple times.
func (h *Hub) Unregister(s *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, s)
}

// Snapshot returns a copy of the live session set so callers can
// iterate without holding the lock.
func (h *Hub) Snapshot() []*Session {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*Session, 0, len(h.sessions))
	for s := range h.sessions {
		out = append(out, s)
	}
	return out
}

// Count returns the number of live sessions. Cheap; called by the
// daemon health loop.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}

// UnsubscribeAll removes locomotive subscriptions from every live session.
func (h *Hub) UnsubscribeAll(addrs ...uint16) {
	if len(addrs) == 0 {
		return
	}
	for _, s := range h.Snapshot() {
		s.Unsubscribe(addrs...)
	}
}

// SendDropsTotal sums per-session send-queue drops across live sessions.
func (h *Hub) SendDropsTotal() uint64 {
	var n uint64
	for _, s := range h.Snapshot() {
		n += s.SendDrop()
	}
	return n
}

// Broadcast sends env to every session that has subscribed to addr,
// or to every session when addr == 0 (used for `dcc-bus.opened`
// echoes and `system.estop` broadcasts). Errors are swallowed so a
// stuck client cannot block the fan-out path.
func (h *Hub) Broadcast(ctx context.Context, addr uint16, env contract.EnvelopeWire) {
	for _, s := range h.Snapshot() {
		if addr != 0 && !s.IsSubscribed(addr) {
			continue
		}
		_ = s.Send(ctx, env)
	}
}

// SubscribedAddrs returns the union of locomotive addresses that any
// live session currently subscribes to. The state-feed poller uses it
// to limit DCC traffic to addresses someone is actually watching.
func (h *Hub) SubscribedAddrs() []uint16 {
	seen := make(map[uint16]struct{}, 8)
	for _, s := range h.Snapshot() {
		for _, a := range s.SubscribedAddrs() {
			seen[a] = struct{}{}
		}
	}
	out := make([]uint16, 0, len(seen))
	for a := range seen {
		out = append(out, a)
	}
	return out
}

// SessionsForUser returns every live session belonging to userID.
// Used by takeover handling to selectively close one user's tabs.
func (h *Hub) SessionsForUser(userID uint) []*Session {
	out := make([]*Session, 0, 2)
	for _, s := range h.Snapshot() {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out
}
