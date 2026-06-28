package z21server

import (
	"net"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes/inbound"
)

// Client is one registered Z21 LAN participant (§1.1 implicit login).
type Client = inbound.Client

// Registry tracks active UDP participants for one virtual Z21 server.
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

// Touch returns the client for addr, creating it on first sight.
func (r *Registry) Touch(addr *net.UDPAddr, now time.Time, ipStickiness bool) *Client {
	return r.inbound.Touch(contract.RemoteProtocolZ21, addr, now, ipStickiness)
}

// ClearIdleBraked resets the idle-brake latch after handset activity resumes.
func (r *Registry) ClearIdleBraked(key string) {
	r.inbound.ClearIdleBraked(key)
}

// SetIdleBraked marks a client as idle-braked until the next UDP packet.
func (r *Registry) SetIdleBraked(key string, braked bool) {
	r.inbound.SetIdleBraked(key, braked)
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
func (r *Registry) SetPaired(key string, active *contract.Z21PairingActiveWire) {
	if active == nil {
		r.inbound.SetSession(key, nil)
		r.wire.ClearPairingBuffer(key)
		return
	}
	r.inbound.SetSession(key, active)
	r.wire.ClearPairingBuffer(key)
}

// Paired returns a copy of the active session for key, if any.
func (r *Registry) Paired(key string) (*contract.Z21PairingActiveWire, bool) {
	return r.inbound.Session(key)
}

// SetBroadcastFlags stores LAN_SET_BROADCASTFLAGS on a client.
func (r *Registry) SetBroadcastFlags(key string, flags uint32) {
	r.wire.SetBroadcastFlags(key, flags)
}

// SubscribeLoco adds addr to the per-client FIFO (max 16 per Z21 spec).
func (r *Registry) SubscribeLoco(key string, addr uint16) {
	r.inbound.SubscribeLoco(key, addr)
}

// SetLastActiveLoco records the handset's current locomotive.
func (r *Registry) SetLastActiveLoco(key string, addr uint16) {
	r.inbound.SetLastActiveLoco(key, addr)
}

// CurrentLoco returns the pilot's active address for one client key.
func (r *Registry) CurrentLoco(key string) uint16 {
	return r.inbound.CurrentLoco(key)
}

// SubscribedTo reports whether addr is in the client's subscription FIFO.
func (r *Registry) SubscribedTo(key string, addr uint16) bool {
	return r.inbound.SubscribedTo(key, addr)
}

// Subscribers returns the client keys subscribed to addr.
func (r *Registry) Subscribers(addr uint16) []string {
	return r.inbound.Subscribers(addr)
}

// BufferPairingCV records one CV3/CV4 POM value while pairing.
func (r *Registry) BufferPairingCV(key string, cvWire int, value int) (cv3, cv4 int, ready bool) {
	return r.wire.BufferPairingCV(key, cvWire, value)
}

// SetVirtualCV stores a virtual CV value for one client.
func (r *Registry) SetVirtualCV(key string, loco uint16, cvWire int, value byte) {
	r.wire.SetVirtualCV(key, loco, cvWire, value)
}

// GetVirtualCV reads a virtual CV value for one client.
func (r *Registry) GetVirtualCV(key string, loco uint16, cvWire int) (byte, bool) {
	return r.wire.GetVirtualCV(key, loco, cvWire)
}

// Snapshot returns a deep copy of every registered client.
func (r *Registry) Snapshot() []*Client {
	return r.inbound.Snapshot()
}

func (r *Registry) ClearPairingBuffer(key string) {
	r.wire.ClearPairingBuffer(key)
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

// MarkSeenDirty stages a lastSeenAt update for batched flush (WS-1b).
func (r *Registry) MarkSeenDirty(key string, ts int64) {
	r.inbound.MarkSeenDirty(key, ts)
}

// IdleBraked reports whether the idle-brake latch is set for key.
func (r *Registry) IdleBraked(key string) bool {
	return r.inbound.IdleBraked(key)
}

// BufferPairingFn records one function-key ON press while pairing.
func (r *Registry) BufferPairingFn(key string, fn int) (cv3, cv4 int, ready bool) {
	return r.wire.BufferPairingFn(key, fn)
}

// PairingFnRisingEdges returns function numbers that turned on in a group update.
func (r *Registry) PairingFnRisingEdges(key string, group, fnByte byte) []int {
	return r.wire.PairingFnRisingEdges(key, group, fnByte)
}

// BroadcastFlags returns LAN_SET_BROADCASTFLAGS for key.
func (r *Registry) BroadcastFlags(key string) uint32 {
	return r.wire.BroadcastFlags(key)
}

// Wire returns the Z21 wire-state table for cleanup hooks.
func (r *Registry) Wire() *WireState {
	return r.wire
}

// EvictIdle removes clients whose LastSeen is older than cutoff.
//
// Deprecated: use Coordinator.Evict; see inbound.ClientRegistry.EvictIdle.
func (r *Registry) EvictIdle(cutoff time.Time) []string {
	evicted := r.inbound.EvictIdle(cutoff)
	for _, key := range evicted {
		r.wire.Remove(key)
	}
	return evicted
}

// Len reports the number of registered participants.
func (r *Registry) Len() int {
	return r.inbound.Len()
}

// Inbound returns the shared inbound registry backing this Z21 view.
func (r *Registry) Inbound() *inbound.ClientRegistry {
	return r.inbound
}

// clientKey formats the default session key (protocol:IP:port).
func clientKey(addr *net.UDPAddr) string {
	return inbound.ClientKey(contract.RemoteProtocolZ21, inbound.EndpointFromAddr(addr, false))
}
