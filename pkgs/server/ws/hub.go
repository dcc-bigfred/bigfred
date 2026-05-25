package ws

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/keskad/loco/pkgs/server/domain"
)

// onlineUser tracks one connected user inside a layout. Multiple WS
// tabs for the same login collapse to a single PresenceUser row.
type onlineUser struct {
	UserID uint
	Login  string
}

// PresenceRefresher rebuilds and broadcasts the layout presence
// snapshot. Injected at wire time to break the Hub ↔ service cycle.
type PresenceRefresher interface {
	RefreshAndBroadcast(ctx context.Context, layoutID uint)
}

// Hub is the central registry of live WebSocket drive sessions (§5.1).
type Hub struct {
	mu sync.RWMutex

	clients  map[*Client]struct{}
	sessions map[string]*DriveSession // session ID → session
	// layoutID → userID → aggregated online user (deduped tabs)
	online map[uint]map[uint]onlineUser

	register   chan *Client
	unregister chan *Client

	presence PresenceRefresher
}

// NewHub constructs a Hub. Call Run before accepting connections.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		sessions:   make(map[string]*DriveSession),
		online:     make(map[uint]map[uint]onlineUser),
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
	}
}

// SetPresenceRefresher wires the callback invoked after every
// register/unregister so the dashboard stays live.
func (h *Hub) SetPresenceRefresher(p PresenceRefresher) {
	h.presence = p
}

// Run processes register/unregister until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-h.register:
			h.addClient(c)
		case c := <-h.unregister:
			h.removeClient(c)
		}
	}
}

func (h *Hub) addClient(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.sessions[c.session.ID] = c.session
	if h.online[c.session.LayoutID] == nil {
		h.online[c.session.LayoutID] = make(map[uint]onlineUser)
	}
	h.online[c.session.LayoutID][c.session.UserID] = onlineUser{
		UserID: c.session.UserID,
		Login:  c.session.Login,
	}
	layoutID := c.session.LayoutID
	refresher := h.presence
	h.mu.Unlock()

	if refresher != nil {
		refresher.RefreshAndBroadcast(context.Background(), layoutID)
	}
}

func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	delete(h.sessions, c.session.ID)

	layoutID := c.session.LayoutID
	userID := c.session.UserID
	stillOnline := false
	for client := range h.clients {
		if client.session.LayoutID == layoutID && client.session.UserID == userID {
			stillOnline = true
			break
		}
	}
	if !stillOnline {
		if users, ok := h.online[layoutID]; ok {
			delete(users, userID)
			if len(users) == 0 {
				delete(h.online, layoutID)
			}
		}
	}
	refresher := h.presence
	h.mu.Unlock()

	close(c.send)

	if refresher != nil {
		refresher.RefreshAndBroadcast(context.Background(), layoutID)
	}
}

// Register enqueues a client for admission. Called from the HTTP
// upgrade goroutine.
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// OnlineUsers returns the deduplicated set of users connected to a
// layout. Safe for concurrent readers.
func (h *Hub) OnlineUsers(layoutID uint) []onlineUser {
	h.mu.RLock()
	defer h.mu.RUnlock()
	users := h.online[layoutID]
	out := make([]onlineUser, 0, len(users))
	for _, u := range users {
		out = append(out, u)
	}
	return out
}

// BroadcastToLayout sends an envelope to every client pinned to
// layoutID.
func (h *Hub) BroadcastToLayout(layoutID uint, eventType string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	env := Envelope{Type: eventType, Payload: data}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.session.LayoutID != layoutID {
			continue
		}
		select {
		case c.send <- env:
		default:
		}
	}
}

// BroadcastToUserInLayout sends an envelope to every WS session of
// `userID` that is pinned to `layoutID`. Used by the sudo flow to
// notify all open tabs of the same login that their effective role
// just changed (§7a.7). Other users in the same layout don't see
// the event — sudo state is private to its owner.
func (h *Hub) BroadcastToUserInLayout(layoutID, userID uint, eventType string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	env := Envelope{Type: eventType, Payload: data}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.session.LayoutID != layoutID {
			continue
		}
		if c.session.UserID != userID {
			continue
		}
		select {
		case c.send <- env:
		default:
		}
	}
}

// NewDriveSession allocates a session for a freshly upgraded client.
func NewDriveSession(userID uint, login string, layoutID uint) *DriveSession {
	return &DriveSession{
		ID:       uuid.NewString(),
		UserID:   userID,
		Login:    login,
		LayoutID: layoutID,
		OpenedAt: time.Now().UTC(),
	}
}

// DriveSession is the in-memory handle created on every WS upgrade
// (§4.5.1). Throttle fields land in later milestones.
type DriveSession struct {
	ID       string
	UserID   uint
	Login    string
	LayoutID uint
	OpenedAt time.Time
}

// Envelope is the common wire format for every WS frame (§4.2).
type Envelope struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// PresenceChangedPayload is the server → client layout.presenceChanged
// event body.
type PresenceChangedPayload struct {
	LayoutID uint                  `json:"layoutId"`
	Users    []domain.PresenceUser `json:"users"`
}

// OccupantChangedPayload is the server → client
// interlocking.occupantChanged event body.
type OccupantChangedPayload struct {
	InterlockingID uint             `json:"interlockingId"`
	Occupant       *OccupantPayload `json:"occupant,omitempty"`
	Reason         string           `json:"reason,omitempty"`
}

// OccupantPayload identifies the user staffing a box.
type OccupantPayload struct {
	UserID uint   `json:"userId"`
	Login  string `json:"login"`
}

// ElevationChangedPayload is the server → client
// `auth.elevationChanged` event body (§7a.7). The client refetches
// `/api/v1/auth/me` on receipt to pick up the fresh
// effectiveRole / sudo set; we deliberately don't ship the new role
// inline so the wire shape stays small and the source of truth
// remains the REST endpoint.
type ElevationChangedPayload struct {
	LayoutID uint `json:"layoutId"`
	UserID   uint `json:"userId"`
}
