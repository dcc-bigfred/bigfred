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
	LastActiveLoco  uint16
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

// Paired returns a copy of the active session for key, if any.
func (r *Registry) Paired(key string) (*contract.Z21PairingActiveWire, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok || c.Paired == nil {
		return nil, false
	}
	copy := *c.Paired
	return &copy, true
}

// SetBroadcastFlags stores LAN_SET_BROADCASTFLAGS on a client.
func (r *Registry) SetBroadcastFlags(key string, flags uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[key]; ok {
		c.BroadcastFlags = flags
	}
}

// SubscribeLoco adds addr to the per-client FIFO (max 16 per Z21 spec).
func (r *Registry) SubscribeLoco(key string, addr uint16) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return
	}
	subscribeLocoLocked(c, addr)
}

// SetLastActiveLoco records the handset's current locomotive and keeps the
// subscription FIFO in sync (used for per-pilot estop).
func (r *Registry) SetLastActiveLoco(key string, addr uint16) {
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
	subscribeLocoLocked(c, addr)
}

// CurrentLoco returns the pilot's active address for one client key.
func (r *Registry) CurrentLoco(key string) uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return 0
	}
	return c.currentLocoLocked()
}

// SubscribedTo reports whether addr is in the client's subscription FIFO.
func (r *Registry) SubscribedTo(key string, addr uint16) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return false
	}
	return c.subscribedToLocked(addr)
}

// BufferPairingCV records one CV3/CV4 POM value while pairing.
func (r *Registry) BufferPairingCV(key string, cvWire int, value int) (cv3, cv4 int, ready bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return 0, 0, false
	}
	return c.bufferPairingCVLocked(cvWire, value)
}

// SetVirtualCV stores a virtual CV value for one client.
func (r *Registry) SetVirtualCV(key string, loco uint16, cvWire int, value byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return
	}
	c.setVirtualCVLocked(loco, cvWire, value)
}

// GetVirtualCV reads a virtual CV value for one client.
func (r *Registry) GetVirtualCV(key string, loco uint16, cvWire int) (byte, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return 0, false
	}
	return c.getVirtualCVLocked(loco, cvWire)
}

func subscribeLocoLocked(c *Client, addr uint16) {
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

func (c *Client) bufferPairingCVLocked(cvWire int, value int) (cv3, cv4 int, ready bool) {
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

func (c *Client) setVirtualCVLocked(loco uint16, cvWire int, value byte) {
	if c.virtualCV == nil {
		c.virtualCV = make(map[uint32]byte)
	}
	c.virtualCV[virtualCVKey(loco, cvWire)] = value
}

func (c *Client) getVirtualCVLocked(loco uint16, cvWire int) (byte, bool) {
	if c.virtualCV == nil {
		return 0, false
	}
	v, ok := c.virtualCV[virtualCVKey(loco, cvWire)]
	return v, ok
}

// Snapshot returns a deep copy of every registered client.
func (r *Registry) Snapshot() []*Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Client, 0, len(r.clients))
	for _, c := range r.clients {
		out = append(out, copyClient(c))
	}
	return out
}

func copyClient(c *Client) *Client {
	cp := *c
	if c.Paired != nil {
		p := *c.Paired
		cp.Paired = &p
	}
	if len(c.SubscribedLocos) > 0 {
		cp.SubscribedLocos = append([]uint16(nil), c.SubscribedLocos...)
	}
	if len(c.virtualCV) > 0 {
		cp.virtualCV = make(map[uint32]byte, len(c.virtualCV))
		for k, v := range c.virtualCV {
			cp.virtualCV[k] = v
		}
	}
	return &cp
}

func (r *Registry) ClearPairingBuffer(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[key]; ok {
		c.clearPairingBuffer()
	}
}

// IsPaired reports whether key has an active handset session.
func (r *Registry) IsPaired(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	return ok && c.Paired != nil
}

// IdleBraked reports whether the idle-brake latch is set for key.
func (r *Registry) IdleBraked(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	return ok && c.IdleBraked
}

// BufferPairingFn records one function-key ON press while pairing.
func (r *Registry) BufferPairingFn(key string, fn int) (cv3, cv4 int, ready bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return 0, 0, false
	}
	return c.BufferPairingFn(fn)
}

// PairingFnRisingEdges returns function numbers that turned on in a group update.
func (r *Registry) PairingFnRisingEdges(key string, group, fnByte byte) []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		return nil
	}
	return c.pairingFnRisingEdges(group, fnByte)
}

func (c *Client) clearPairingBuffer() {
	c.pairCV3 = nil
	c.pairCV4 = nil
	c.pairFnBuf = nil
	c.pairFnPrevGroup = nil
}

// CurrentLoco returns the active loco on a detached client copy (e.g. from Snapshot).
func (c *Client) CurrentLoco() uint16 {
	return c.currentLocoLocked()
}

// SubscribedTo reports subscription membership on a detached client copy.
func (c *Client) SubscribedTo(addr uint16) bool {
	return c.subscribedToLocked(addr)
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
