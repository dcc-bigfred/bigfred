// Package ws is the dcc-bus daemon's WebSocket transport.
package ws

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/auth"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

const (
	wsSendQueueCap = 64
	wsWriteTimeout = 2 * time.Second
)

// Session is one connected user (one browser tab). Multiple sessions
// per (userID, layoutID, commandStationId) are allowed — the daemon
// fans state events out to every matching tab.
type Session struct {
	ID       string
	UserID   uint
	LayoutID uint
	Login    string
	OpenedAt time.Time

	conn *websocket.Conn

	mu          sync.Mutex
	sendCh      chan contract.EnvelopeWire
	sendDrop    atomic.Uint64
	subscribed  map[uint16]struct{}
	lastBeat    time.Time
	closed      bool
	closeReason string
}

// NewSession allocates a new client handle from a verified Identity.
func NewSession(id auth.Identity, conn *websocket.Conn) *Session {
	s := &Session{
		ID:         uuid.NewString(),
		UserID:     id.UserID,
		LayoutID:   id.LayoutID,
		Login:      id.Login,
		OpenedAt:   time.Now().UTC(),
		conn:       conn,
		sendCh:     make(chan contract.EnvelopeWire, wsSendQueueCap),
		subscribed: make(map[uint16]struct{}, 8),
		lastBeat:   time.Now().UTC(),
	}
	go s.writeLoop()
	return s
}

func (s *Session) writeLoop() {
	for env := range s.sendCh {
		raw, err := json.Marshal(env)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), wsWriteTimeout)
		err = s.conn.Write(ctx, websocket.MessageText, raw)
		cancel()
		if err != nil {
			s.Close("write failed: " + err.Error())
			for range s.sendCh {
			}
			return
		}
	}
}

// SendDrop returns how many outbound frames were dropped (oldest-first)
// because the per-client send queue was saturated.
func (s *Session) SendDrop() uint64 {
	return s.sendDrop.Load()
}

// Subscribe records interest in additional locomotive addresses.
func (s *Session) Subscribe(addrs ...uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range addrs {
		s.subscribed[a] = struct{}{}
	}
}

// Unsubscribe drops interest in the given locomotive addresses.
func (s *Session) Unsubscribe(addrs ...uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range addrs {
		delete(s.subscribed, a)
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

// Send enqueues env for the session write loop. It is non-blocking:
// when the queue is full the oldest frame is dropped.
func (s *Session) Send(ctx context.Context, env contract.EnvelopeWire) error {
	s.mu.Lock()
	if s.closed || s.sendCh == nil {
		s.mu.Unlock()
		return websocket.CloseError{Code: websocket.StatusGoingAway}
	}
	ch := s.sendCh
	s.mu.Unlock()

	select {
	case ch <- env:
	default:
		select {
		case <-ch:
		default:
		}
		s.sendDrop.Add(1)
		ch <- env
	}
	return nil
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
	return s.SendAckData(ctx, requestID, protocol.AckPayload{OK: ok, Error: errCode})
}

// SendAckData emits an ack frame with a full payload body.
func (s *Session) SendAckData(ctx context.Context, requestID string, payload protocol.AckPayload) error {
	env, err := protocol.FrameWithID(protocol.TypeAck, requestID, payload)
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
	if s.closeReason == "" {
		s.closeReason = reason
	}
	conn := s.conn
	ch := s.sendCh
	s.sendCh = nil
	s.mu.Unlock()
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, reason)
	}
	if ch != nil {
		close(ch)
	}
}

// CloseReason returns the first reason passed to Close, or "" when the
// session is still open.
func (s *Session) CloseReason() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeReason
}

// IsClosed reports whether Close has already been called.
func (s *Session) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}
