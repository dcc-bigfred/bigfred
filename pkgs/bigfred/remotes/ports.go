// Package remotes defines protocol-agnostic ports for inbound handset
// control. Concrete protocol adapters (e.g. Roco Z21 LAN) live outside
// dcc-bus and server cores and depend on these interfaces only.
package remotes

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// HandsetSession identifies one remote control client.
type HandsetSession struct {
	ClientKey string
	UserID    uint
}

// DriveScope is vehicle authorization for a handset session.
type DriveScope struct {
	AllowedAddrs     []uint16
	AllowAllVehicles bool
}

// HandsetSessionID builds a stable router session key for one handset.
func HandsetSessionID(clientKey string) string {
	return "remote:" + clientKey
}

// HandsetClientKeyFromSession is the inverse of HandsetSessionID: it returns
// the originating handset client key for a router session ID, or "" when the
// session is not a handset (e.g. WS/train). Used by gateways to skip echoing
// a state update back to the handset that just commanded it.
func HandsetClientKeyFromSession(sessionID string) string {
	const prefix = "remote:"
	if len(sessionID) > len(prefix) && sessionID[:len(prefix)] == prefix {
		return sessionID[len(prefix):]
	}
	return ""
}

// LocoStateObserver receives locomotive state updates (Observer pattern).
// originClientKey is the handset client key that triggered the update, or ""
// for non-handset origins (poller, external bus, script); observers use it to
// avoid double-echoing to the commanding handset that already got a direct reply.
type LocoStateObserver interface {
	OnLocoStateChanged(ctx context.Context, snap contract.LocoStateWire, originClientKey string)
}

// ClientsSnapshotPublisher stores and fans out handset presence snapshots.
type ClientsSnapshotPublisher interface {
	PublishClientsSnapshot(ctx context.Context, snap contract.RemoteClientsSnapshotWire) error
}

// InboundGateway is a protocol-specific inbound listener registered by
// the composition root (e.g. dcc-bus daemon).
type InboundGateway interface {
	RemoteProtocol
}
