package z21server

import (
	"net"
	"strconv"
	"sync"
	"time"
)

// Client is one registered Z21 LAN participant (§1.1 implicit login).
type Client struct {
	Addr           net.UDPAddr
	Key            string
	LastSeen       time.Time
	BroadcastFlags uint32
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

// Touch returns the client for addr, creating it on first sight.
func (r *Registry) Touch(addr *net.UDPAddr, now time.Time) *Client {
	key := clientKey(addr)
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[key]
	if !ok {
		c = &Client{Addr: *addr, Key: key, LastSeen: now}
		r.clients[key] = c
		return c
	}
	c.LastSeen = now
	return c
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

func clientKey(addr *net.UDPAddr) string {
	if addr == nil {
		return ""
	}
	return net.JoinHostPort(addr.IP.String(), strconv.Itoa(addr.Port))
}
