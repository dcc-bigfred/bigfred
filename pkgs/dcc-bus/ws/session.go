// Package ws is the dcc-bus daemon's WebSocket transport.
package ws

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/keskad/loco/pkgs/dcc-bus/auth"
	"github.com/keskad/loco/pkgs/dcc-bus/protocol"
)

// Session is one connected user (one browser tab). Multiple sessions
// per (userID, layoutID, commandStationID) are allowed — the daemon
// fans state events out to every matching tab.
type Session struct {
	ID       string
	UserID   uint
	LayoutID uint
	Login    string
	OpenedAt time.Time

	conn *websocket.Conn

	mu          sync.Mutex
	subscribed  map[uint16]struct{}
	lastBeat    time.Time
	closed      bool
}

// NewSession allocates a new client handle from a verified Identity.
func NewSession(id auth.Identity, conn *websocket.Conn) *Session {
	return &Session{
		ID:         uuid.NewString(),
		UserID:     id.UserID,
		LayoutID:   id.LayoutID,
		Login:      id.Login,
		OpenedAt:   time.Now().UTC(),
		conn:       conn,
		subscribed: make(map[uint16]struct{}, 8),
		lastBeat:   time.Now().UTC(),
	}
}

// Subscribe records interest in additional locomotive addresses.
func (s *Session) Subscribe(addrs ...uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range addrs {
		s.subscribed[a] = struct{}{}
	}
}

// SubscribedAddrs returns the current subscription set.
func (s *Session) SubscribedAddrs() []uint16 {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]uint16, 0, len(s.subscribed))
	for a := range s.subscribed {
		out = append(out, a)
	}
	return out
}

// IsSubscribed reports whether this session asked for updates about
// the given DCC address.
func (s *Session) IsSubscribed(addr uint16) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.subscribed[addr]
	return ok
}

// Touch resets the dead-man's switch timer.
func (s *Session) Touch() {
	s.mu.Lock()
	s.lastBeat = time.Now().UTC()
	s.mu.Unlock()
}

// IdleFor returns how long it has been since the last inbound frame.
func (s *Session) IdleFor() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.lastBeat)
}

// Send marshals the envelope and writes it to the underlying WS.
// Safe to call from multiple goroutines.
func (s *Session) Send(ctx context.Context, env protocol.Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return websocket.CloseError{Code: websocket.StatusGoingAway}
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return s.conn.Write(ctx, websocket.MessageText, raw)
}

// SendTyped is the typed convenience wrapper around Send. It is the
// preferred way for handlers to emit events.
func (s *Session) SendTyped(ctx context.Context, eventType string, payload any) error {
	env, err := protocol.Frame(eventType, payload)
	if err != nil {
		return err
	}
	return s.Send(ctx, env)
}

// SendAck emits an ack frame correlated with the request ID.
func (s *Session) SendAck(ctx context.Context, requestID string, ok bool, errCode string) error {
	env, err := protocol.FrameWithID(protocol.TypeAck, requestID, protocol.AckPayload{OK: ok, Error: errCode})
	if err != nil {
		return err
	}
	return s.Send(ctx, env)
}

// Close marks the session as closed and shuts the underlying WS
// down cleanly. Idempotent.
func (s *Session) Close(reason string) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	conn := s.conn
	s.mu.Unlock()
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, reason)
	}
}

// IsClosed reports whether Close has already been called.
func (s *Session) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}
