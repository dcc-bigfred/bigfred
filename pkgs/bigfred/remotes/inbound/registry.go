package inbound

import (
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// Client is one registered inbound handset participant.
type Client struct {
	Key             string
	Protocol        string
	Endpoint        string
	Addr            net.UDPAddr
	LastSeen        time.Time
	ConnectedAt     time.Time
	IdleBraked      bool
	SubscribedLocos []uint16
	LastActiveLoco  uint16
	Session         *contract.RemoteSessionWire
	// syncedAt is the last time the session was reconciled against Redis.
	// Per-packet sync is gated on its staleness so the hot path avoids a
	// Redis GET; event-driven sync (WS-1) and the first packet after
	// (re)connect refresh it. Zero means "never synced" → needs sync.
	syncedAt time.Time
	// seenDirty marks a pending lastSeenAt update to be flushed to Redis
	// in batch by the coordinator (WS-1b), replacing a per-packet SET.
	seenDirty   bool
	pendingSeen int64
	// HeartbeatMonitor is set by WiThrottle *+ / *- (dead-man E-stop only when on).
	HeartbeatMonitor bool
}

// ClientRegistry tracks active inbound participants for one command station.
type ClientRegistry struct {
	mu          sync.Mutex
	clients     map[string]*Client
	subscribers map[uint16]map[string]struct{}
}

// NewClientRegistry returns an empty participant map.
func NewClientRegistry() *ClientRegistry {
	return &ClientRegistry{
		clients:     make(map[string]*Client),
		subscribers: make(map[uint16]map[string]struct{}),
	}
}

// Touch returns the client for addr, creating it on first sight.
func (r *ClientRegistry) Touch(protocol string, addr *net.UDPAddr, now time.Time, ipStickiness bool) *Client {
	endpoint := EndpointFromAddr(addr, ipStickiness)
	key := ClientKey(protocol, endpoint)
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		c = &Client{
			Key:         key,
			Protocol:    protocol,
			Endpoint:    endpoint,
			Addr:        *addr,
			LastSeen:    now,
			ConnectedAt: now,
		}
		r.clients[key] = c
		return c
	}
	c.Addr = *addr
	c.LastSeen = now
	return c
}

// TouchByEndpoint registers or updates a client keyed by protocol and an
// opaque endpoint (e.g. WiThrottle HU device id), not IP:port.
func (r *ClientRegistry) TouchByEndpoint(protocol, endpoint string, remote net.Addr, now time.Time) *Client {
	key := ClientKey(protocol, endpoint)
	udp := udpAddrFromNet(remote)
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		c = &Client{
			Key:         key,
			Protocol:    protocol,
			Endpoint:    endpoint,
			Addr:        udp,
			LastSeen:    now,
			ConnectedAt: now,
		}
		r.clients[key] = c
		return c
	}
	if remote != nil {
		c.Addr = udpAddrFromNet(remote)
	}
	c.LastSeen = now
	return c
}

func udpAddrFromNet(a net.Addr) net.UDPAddr {
	if a == nil {
		return net.UDPAddr{}
	}
	if ua, ok := a.(*net.UDPAddr); ok {
		return *ua
	}
	if ta, ok := a.(*net.TCPAddr); ok {
		return net.UDPAddr{IP: ta.IP, Port: ta.Port, Zone: ta.Zone}
	}
	host, port, err := net.SplitHostPort(a.String())
	if err != nil {
		return net.UDPAddr{}
	}
	p, _ := strconv.Atoi(port)
	return net.UDPAddr{IP: net.ParseIP(host), Port: p}
}

// ClearIdleBraked resets the idle-brake latch after handset activity resumes.
func (r *ClientRegistry) ClearIdleBraked(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[key]; ok {
		c.IdleBraked = false
	}
}

// SetIdleBraked marks a client as idle-braked until the next packet.
func (r *ClientRegistry) SetIdleBraked(key string, braked bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[key]; ok {
		c.IdleBraked = braked
	}
}

// Get returns a client by key.
func (r *ClientRegistry) Get(key string) (*Client, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	return c, ok
}

// Remove drops one participant.
func (r *ClientRegistry) Remove(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return
	}
	for _, addr := range c.SubscribedLocos {
		r.removeSubscriberLocked(addr, key)
	}
	c.SubscribedLocos = nil
	delete(r.clients, key)
}

// SetSession stores the active handset session on a client.
func (r *ClientRegistry) SetSession(key string, session *contract.RemoteSessionWire) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[key]; ok {
		if session == nil {
			c.Session = nil
		} else {
			copy := *session
			c.Session = &copy
		}
	}
}

// Session returns a copy of the active session for key, if any.
func (r *ClientRegistry) Session(key string) (*contract.RemoteSessionWire, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok || c.Session == nil {
		return nil, false
	}
	copy := *c.Session
	return &copy, true
}

// SetHeartbeatMonitor toggles WiThrottle dead-man monitoring for one client.
func (r *ClientRegistry) SetHeartbeatMonitor(key string, on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[key]; ok {
		c.HeartbeatMonitor = on
	}
}

// UnsubscribeLoco removes addr from the client's subscription FIFO and index.
func (r *ClientRegistry) UnsubscribeLoco(key string, addr uint16) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return
	}
	kept := c.SubscribedLocos[:0]
	for _, a := range c.SubscribedLocos {
		if a != addr {
			kept = append(kept, a)
		}
	}
	c.SubscribedLocos = kept
	r.removeSubscriberLocked(addr, key)
}

// SubscribeLoco adds addr to the per-client FIFO (max 16 per Z21 spec).
func (r *ClientRegistry) SubscribeLoco(key string, addr uint16) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return
	}
	trimmed, didTrim := subscribeLocoLocked(c, addr)
	r.addSubscriberLocked(addr, key)
	if didTrim {
		r.removeSubscriberLocked(trimmed, key)
	}
}

// SetLastActiveLoco records the handset's current locomotive.
func (r *ClientRegistry) SetLastActiveLoco(key string, addr uint16) {
	if addr == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return
	}
	c.LastActiveLoco = addr
	trimmed, didTrim := subscribeLocoLocked(c, addr)
	r.addSubscriberLocked(addr, key)
	if didTrim {
		r.removeSubscriberLocked(trimmed, key)
	}
}

// CurrentLoco returns the pilot's active address for one client key.
func (r *ClientRegistry) CurrentLoco(key string) uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return 0
	}
	return c.currentLocoLocked()
}

// SubscribedTo reports whether addr is in the client's subscription FIFO.
func (r *ClientRegistry) SubscribedTo(key string, addr uint16) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return false
	}
	return c.subscribedToLocked(addr)
}

// Snapshot returns a deep copy of every registered client.
func (r *ClientRegistry) Snapshot() []*Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Client, 0, len(r.clients))
	for _, c := range r.clients {
		out = append(out, copyClient(c))
	}
	return out
}

// IsPaired reports whether key has an active handset session.
func (r *ClientRegistry) IsPaired(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	return ok && c.Session != nil
}

// NeedsSync reports whether the client's session should be reconciled
// against Redis (first sight, or the safety window has elapsed).
func (r *ClientRegistry) NeedsSync(key string, stale time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return false
	}
	return c.syncedAt.IsZero() || time.Since(c.syncedAt) >= stale
}

// MarkSynced records that the client's session was just reconciled.
func (r *ClientRegistry) MarkSynced(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[key]; ok {
		c.syncedAt = time.Now().UTC()
	}
}

// MarkSeenDirty stages a lastSeenAt update for batched flush (WS-1b)
// instead of a per-packet Redis SET. Only the latest timestamp per
// client matters; the coordinator flusher drains it periodically.
func (r *ClientRegistry) MarkSeenDirty(key string, ts int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[key]; ok {
		c.seenDirty = true
		c.pendingSeen = ts
	}
}

// DrainSeenDirty returns and clears all pending lastSeenAt updates.
// The returned map is clientKey → lastSeenAt (unix ms).
func (r *ClientRegistry) DrainSeenDirty() map[string]int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.clients) == 0 {
		return nil
	}
	out := make(map[string]int64, len(r.clients))
	for key, c := range r.clients {
		if c.seenDirty {
			out[key] = c.pendingSeen
			c.seenDirty = false
		}
	}
	return out
}

// IdleBraked reports whether the idle-brake latch is set for key.
func (r *ClientRegistry) IdleBraked(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	return ok && c.IdleBraked
}

// EvictIdle removes clients whose LastSeen is older than cutoff.
//
// Deprecated: the Coordinator sweep evicts via EvictClient (which also
// unpairs in Redis and runs onEvict hooks). This method is kept on the
// raw registry for direct-registry callers and future protocols that may
// want a batch evict without the store; it maintains the subscriber index.
func (r *ClientRegistry) EvictIdle(cutoff time.Time) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var evicted []string
	for key, c := range r.clients {
		if c.LastSeen.Before(cutoff) {
			for _, addr := range c.SubscribedLocos {
				r.removeSubscriberLocked(addr, key)
			}
			c.SubscribedLocos = nil
			delete(r.clients, key)
			evicted = append(evicted, key)
		}
	}
	return evicted
}

// Subscribers returns the client keys subscribed to addr. The slice is a
// snapshot copy and safe to iterate without holding the registry lock.
func (r *ClientRegistry) Subscribers(addr uint16) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	set, ok := r.subscribers[addr]
	if !ok || len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	return out
}

// Len reports the number of registered participants.
func (r *ClientRegistry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients)
}

// subscribeLocoLocked adds addr to the per-client FIFO (max 16 per Z21
// spec). It returns the address that was evicted from the FIFO (when the
// cap was exceeded) so the caller can update the subscriber index.
func subscribeLocoLocked(c *Client, addr uint16) (uint16, bool) {
	for _, existing := range c.SubscribedLocos {
		if existing == addr {
			return 0, false
		}
	}
	c.SubscribedLocos = append(c.SubscribedLocos, addr)
	if len(c.SubscribedLocos) > 16 {
		evicted := c.SubscribedLocos[0]
		c.SubscribedLocos = c.SubscribedLocos[1:]
		return evicted, true
	}
	return 0, false
}

func (c *Client) currentLocoLocked() uint16 {
	if c.LastActiveLoco != 0 {
		return c.LastActiveLoco
	}
	if n := len(c.SubscribedLocos); n > 0 {
		return c.SubscribedLocos[n-1]
	}
	return 0
}

func (c *Client) subscribedToLocked(addr uint16) bool {
	for _, existing := range c.SubscribedLocos {
		if existing == addr {
			return true
		}
	}
	return false
}

func (r *ClientRegistry) addSubscriberLocked(addr uint16, key string) {
	set, ok := r.subscribers[addr]
	if !ok {
		set = make(map[string]struct{})
		r.subscribers[addr] = set
	}
	set[key] = struct{}{}
}

func (r *ClientRegistry) removeSubscriberLocked(addr uint16, key string) {
	set, ok := r.subscribers[addr]
	if !ok {
		return
	}
	delete(set, key)
	if len(set) == 0 {
		delete(r.subscribers, addr)
	}
}

// CurrentLoco returns the active loco on a detached client copy.
func (c *Client) CurrentLoco() uint16 {
	return c.currentLocoLocked()
}

// SubscribedTo reports subscription membership on a detached client copy.
func (c *Client) SubscribedTo(addr uint16) bool {
	return c.subscribedToLocked(addr)
}

func copyClient(c *Client) *Client {
	cp := *c
	if c.Session != nil {
		s := *c.Session
		cp.Session = &s
	}
	if len(c.SubscribedLocos) > 0 {
		cp.SubscribedLocos = append([]uint16(nil), c.SubscribedLocos...)
	}
	return &cp
}
