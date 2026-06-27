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

// LocoStateObserver receives locomotive state updates (Observer pattern).
type LocoStateObserver interface {
	OnLocoStateChanged(ctx context.Context, snap contract.LocoStateWire)
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
