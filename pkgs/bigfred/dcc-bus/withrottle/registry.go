package withrottle

import (
	"net"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes/inbound"
)

// Client is one registered WiThrottle participant.
type Client = inbound.Client

// Registry tracks active TCP participants for one virtual WiThrottle server.
type Registry struct {
	inbound *inbound.ClientRegistry
	wire    *WireState
}

// NewRegistry returns an empty participant map.
func NewRegistry(in *inbound.ClientRegistry, wire *WireState) *Registry {
	if in == nil {
		in = inbound.NewClientRegistry()
	}
	if wire == nil {
		wire = NewWireState()
	}
	return &Registry{inbound: in, wire: wire}
}

// TouchByDeviceId registers or updates a client keyed by HU device id.
func (r *Registry) TouchByDeviceId(deviceID string, conn net.Conn, now time.Time) *Client {
	key := inbound.ClientKey(contract.RemoteProtocolWithrottle, deviceID)
	var remote net.Addr
	if conn != nil {
		remote = conn.RemoteAddr()
	}
	c := r.inbound.TouchByEndpoint(contract.RemoteProtocolWithrottle, deviceID, remote, now)
	r.wire.SetConn(key, conn)
	r.wire.SetDeviceID(key, deviceID)
	return c
}

// SetConn updates the TCP connection after reconnect.
func (r *Registry) SetConn(key string, conn net.Conn) {
	r.wire.SetConn(key, conn)
	if conn != nil {
		if c, ok := r.inbound.Get(key); ok {
			_ = c
			r.inbound.TouchByEndpoint(contract.RemoteProtocolWithrottle, c.Endpoint, conn.RemoteAddr(), time.Now().UTC())
		}
	}
}

// WriteLine sends one WiThrottle line to the client.
func (r *Registry) WriteLine(key, line string) error {
	return r.wire.WriteLine(key, line)
}

// Get returns a client by key.
func (r *Registry) Get(key string) (*Client, bool) {
	return r.inbound.Get(key)
}

// Remove drops one participant.
func (r *Registry) Remove(key string) {
	r.inbound.Remove(key)
	r.wire.Remove(key)
}

// SetPaired stores the active handset session on a client.
func (r *Registry) SetPaired(key string, active *contract.RemoteSessionWire) {
	if active == nil {
		r.inbound.SetSession(key, nil)
		r.wire.ClearPairingBuffer(key)
		return
	}
	r.inbound.SetSession(key, active)
	r.wire.ClearPairingBuffer(key)
}

// Session returns a copy of the active session for key, if any.
func (r *Registry) Session(key string) (*contract.RemoteSessionWire, bool) {
	return r.inbound.Session(key)
}

// SubscribeLoco adds addr to the per-client subscription FIFO.
func (r *Registry) SubscribeLoco(key string, addr uint16) {
	r.inbound.SubscribeLoco(key, addr)
}

// UnsubscribeLoco removes addr from subscriptions (best-effort via resubscribe trim).
func (r *Registry) UnsubscribeLoco(key string, addr uint16) {
	// inbound registry has no explicit unsubscribe; release uses coordinator paths.
	// Loco release clears wire throttle maps; subscriber index is updated on Remove.
	_ = addr
	_ = key
}

// Subscribers returns the client keys subscribed to addr.
func (r *Registry) Subscribers(addr uint16) []string {
	return r.inbound.Subscribers(addr)
}

// Snapshot returns a deep copy of every registered client.
func (r *Registry) Snapshot() []*Client {
	return r.inbound.Snapshot()
}

// IsPaired reports whether key has an active handset session.
func (r *Registry) IsPaired(key string) bool {
	return r.inbound.IsPaired(key)
}

// NeedsSync reports whether the client's session should be reconciled.
func (r *Registry) NeedsSync(key string, stale time.Duration) bool {
	return r.inbound.NeedsSync(key, stale)
}

// MarkSynced records that the client's session was just reconciled.
func (r *Registry) MarkSynced(key string) {
	r.inbound.MarkSynced(key)
}

// MarkSeenDirty stages a lastSeenAt update for batched flush.
func (r *Registry) MarkSeenDirty(key string, ts int64) {
	r.inbound.MarkSeenDirty(key, ts)
}

// IdleBraked reports whether the idle-brake latch is set for key.
func (r *Registry) IdleBraked(key string) bool {
	return r.inbound.IdleBraked(key)
}

// ClearIdleBraked resets the idle-brake latch after handset activity resumes.
func (r *Registry) ClearIdleBraked(key string) {
	r.inbound.ClearIdleBraked(key)
}

// BufferPairingFn records one function-key ON press while pairing.
func (r *Registry) BufferPairingFn(key string, fn int) (code string, ready bool) {
	return r.wire.BufferPairingFn(key, fn)
}

// PairingFnRisingEdge reports whether fn turned on.
func (r *Registry) PairingFnRisingEdge(key string, fn int, on bool) bool {
	return r.wire.PairingFnRisingEdge(key, fn, on)
}

// ClearPairingBuffer resets in-flight pairing entry buffers.
func (r *Registry) ClearPairingBuffer(key string) {
	r.wire.ClearPairingBuffer(key)
}

func (r *Registry) setDeviceName(key, name string) {
	r.wire.SetDeviceName(key, name)
}

func (r *Registry) deviceName(key string) string {
	return r.wire.DeviceName(key)
}

func (r *Registry) setHeartbeatMonitor(key string, on bool) {
	r.wire.SetHeartbeatMonitor(key, on)
}

func (r *Registry) sentinelAcquired(key string) bool {
	return r.wire.SentinelAcquired(key)
}

func (r *Registry) setSentinelAcquired(key string, acquired bool) {
	r.wire.SetSentinelAcquired(key, acquired)
}

func (r *Registry) withThrottle(key string, id byte, fn func(*throttleWire)) {
	r.wire.WithThrottle(key, id, fn)
}

func (r *Registry) findThrottleForAddr(key string, addr uint16) (byte, string, bool) {
	return r.wire.FindThrottleForAddr(key, addr)
}

// Wire returns the WiThrottle wire-state table for cleanup hooks.
func (r *Registry) Wire() *WireState {
	return r.wire
}

// Inbound returns the shared inbound registry backing this view.
func (r *Registry) Inbound() *inbound.ClientRegistry {
	return r.inbound
}

// ClientKeyForDevice formats the registry key for one HU device id.
func ClientKeyForDevice(deviceID string) string {
	return inbound.ClientKey(contract.RemoteProtocolWithrottle, deviceID)
}
