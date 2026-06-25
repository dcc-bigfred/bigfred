package z21server

import (
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// Client is one registered Z21 LAN participant (§1.1 implicit login).
type Client struct {
	Addr           net.UDPAddr
	Key            string
	LastSeen       time.Time
	ConnectedAt    time.Time
	IdleBraked     bool
	BroadcastFlags uint32

	Paired *contract.Z21PairingActiveWire

	pairCV3 *int
	pairCV4 *int
	pairFnBuf []string
	pairFnPrevGroup map[byte]byte

	virtualCV map[uint32]byte

	SubscribedLocos []uint16
}

// Registry tracks active UDP participants for one virtual Z21 server.
type Registry struct {
	mu      sync.Mutex
	clients map[string]*Client
}

// NewRegistry returns an empty participant map.
func NewRegistry() *Registry {
	return &Registry{clients: make(map[string]*Client)}
}

// Touch returns the client for addr, creating it on first sight. When
// ipStickiness is set the session key is the client IP only so a UDP
// port change reuses the same registry entry and paired session.
func (r *Registry) Touch(addr *net.UDPAddr, now time.Time, ipStickiness bool) *Client {
	key := sessionKey(addr, ipStickiness)
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		c = &Client{Addr: *addr, Key: key, LastSeen: now, ConnectedAt: now}
		r.clients[key] = c
		return c
	}
	c.Addr = *addr
	c.LastSeen = now
	return c
}

// ClearIdleBraked resets the idle-brake latch after handset activity resumes.
func (r *Registry) ClearIdleBraked(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[key]; ok {
		c.IdleBraked = false
	}
}

// SetIdleBraked marks a client as idle-braked until the next UDP packet.
func (r *Registry) SetIdleBraked(key string, braked bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[key]; ok {
		c.IdleBraked = braked
	}
}

// Get returns a client by key.
func (r *Registry) Get(key string) (*Client, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	return c, ok
}

// Remove drops one participant.
func (r *Registry) Remove(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, key)
}

// SetPaired stores the active handset session on a client.
func (r *Registry) SetPaired(key string, active *contract.Z21PairingActiveWire) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[key]; ok {
		if active == nil {
			c.Paired = nil
		} else {
			copy := *active
			c.Paired = &copy
		}
		c.clearPairingBuffer()
	}
}

func (c *Client) clearPairingBuffer() {
	c.pairCV3 = nil
	c.pairCV4 = nil
	c.pairFnBuf = nil
	c.pairFnPrevGroup = nil
}

// BufferPairingCV records one CV3/CV4 POM value while pairing.
func (c *Client) BufferPairingCV(cvWire int, value int) (cv3, cv4 int, ready bool) {
	switch cvWire {
	case 2:
		c.pairCV3 = &value
	case 3:
		c.pairCV4 = &value
	default:
		return 0, 0, false
	}
	if c.pairCV3 != nil && c.pairCV4 != nil {
		return *c.pairCV3, *c.pairCV4, true
	}
	return 0, 0, false
}

// SubscribeLoco adds addr to the per-client FIFO (max 16 per Z21 spec).
func (c *Client) SubscribeLoco(addr uint16) {
	for _, existing := range c.SubscribedLocos {
		if existing == addr {
			return
		}
	}
	c.SubscribedLocos = append(c.SubscribedLocos, addr)
	if len(c.SubscribedLocos) > 16 {
		c.SubscribedLocos = c.SubscribedLocos[len(c.SubscribedLocos)-16:]
	}
}

// SubscribedTo reports whether addr is in the client's subscription FIFO.
func (c *Client) SubscribedTo(addr uint16) bool {
	for _, existing := range c.SubscribedLocos {
		if existing == addr {
			return true
		}
	}
	return false
}

// Snapshot returns a shallow copy of every registered client.
func (r *Registry) Snapshot() []*Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Client, 0, len(r.clients))
	for _, c := range r.clients {
		cp := *c
		out = append(out, &cp)
	}
	return out
}

// EvictIdle removes clients whose LastSeen is older than cutoff.
func (r *Registry) EvictIdle(cutoff time.Time) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var evicted []string
	for key, c := range r.clients {
		if c.LastSeen.Before(cutoff) {
			delete(r.clients, key)
			evicted = append(evicted, key)
		}
	}
	return evicted
}

// Len reports the number of registered participants.
func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients)
}

func sessionKey(addr *net.UDPAddr, ipStickiness bool) string {
	if addr == nil {
		return ""
	}
	if ipStickiness {
		return addr.IP.String()
	}
	return net.JoinHostPort(addr.IP.String(), strconv.Itoa(addr.Port))
}

// clientKey formats the default session key (IP:port).
func clientKey(addr *net.UDPAddr) string {
	return sessionKey(addr, false)
}
