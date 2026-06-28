package remotes

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// RemoteProtocol is a protocol-specific inbound handset listener.
type RemoteProtocol interface {
	Name() string
	LocoStateObserver
	Run(ctx context.Context) error
}

// PairingStrategy creates and completes protocol-specific pairing flows.
// TODO(withrottle): implement and route cmd.Remote.StartPairing through
// registered strategies instead of the manual protocol switch. Z21's
// strategy generates CV3/CV4; WiThrottle's would pair on device id (`HU`).
type PairingStrategy interface {
	Protocol() string
	CreatePending(ctx context.Context, store *remotepairing.Store, in CreatePendingParams) (contract.RemotePendingWire, error)
}

// CreatePendingParams carries user scope for a new pending pairing request.
type CreatePendingParams struct {
	LayoutID         uint
	CommandStationID uint
	UserID           uint
	VehicleIDs       []string
	AllowedAddrs     []uint16
	AllowAllVehicles bool
	HandsetBrakeSecs uint
}

// CVReader reads locomotive CV values for virtual programming track flows.
// TODO(withrottle): used by Z21 virtual-CV POM intercept; not applicable to
// WiThrottle — keep behind the Z21 adapter, not the generic remotes port.
type CVReader interface {
	ReadLocoCV(addr uint16, cvNum commandstation.CVNum) (int, error)
}

// HandsetDriveWithCV combines handset drive and CV read for inbound adapters.
type HandsetDriveWithCV interface {
	HandsetDrivePort
	CVReader
}
