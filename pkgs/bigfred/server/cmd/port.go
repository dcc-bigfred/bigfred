package cmd

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// DCCPoolPort checks whether a DCC address falls inside the owner's
// allocated pool. Implemented by DCCPool.
type DCCPoolPort interface {
	AllowsAddress(ctx context.Context, ownerID uint, addr uint16) (bool, error)
}

// PoolRange is the validated input row of a DCC pool replacement.
type PoolRange struct {
	From uint16
	To   uint16
}

// DCCPoolManagerPort is the subset of DCCPool used by User.
type DCCPoolManagerPort interface {
	List(ctx context.Context, userID uint) ([]domain.DCCAddressRange, error)
	ListAll(ctx context.Context) ([]domain.DCCAddressRange, error)
	Validate(ctx context.Context, userID uint, ranges []PoolRange) error
	Replace(ctx context.Context, eff domain.EffectiveRoles, userID uint, ranges []PoolRange) ([]domain.DCCAddressRange, error)
	DeleteForUser(ctx context.Context, eff domain.EffectiveRoles, userID uint) error
}

// LayoutRosterHubPort fans out layout roster WS events without importing ws.
type LayoutRosterHubPort interface {
	BroadcastVehicleChanged(layoutID uint, vehicleID domain.VehicleID, action string)
	BroadcastTrainChanged(layoutID uint, trainID domain.TrainID, action string)
}

// LayoutRosterSyncPort republishes Redis roster snapshots for dcc-bus.
type LayoutRosterSyncPort interface {
	SyncLayout(ctx context.Context, layoutID uint) error
	SyncForTrain(ctx context.Context, trainID domain.TrainID) error
	SyncForVehicleInTrains(ctx context.Context, vehicleID domain.VehicleID) error
}

// SudoHubPort fans out auth elevation changes without importing ws.
type SudoHubPort interface {
	BroadcastElevationChanged(layoutID, userID uint)
}

// DccBusControlPort is the dcc-bus lifecycle subset used by SessionControl.
type DccBusControlPort interface {
	EnsureRunning(ctx context.Context, layoutID, commandStationID uint) (uint16, string, error)
	PortFor(layoutID, commandStationID uint) uint16
	ProxyEnabled() bool
}

// RadioStopControlPort handles layout-wide Radio Stop from the control WS.
type RadioStopControlPort interface {
	Trigger(ctx context.Context, sess ControlSession) (bool, string)
}

// EStopTargetControlPort handles per-target emergency stop from the control WS.
type EStopTargetControlPort interface {
	Trigger(ctx context.Context, sess ControlSession, target domain.TakeoverTarget, targetID string) (bool, string)
}

// CommandStationDccSyncPort observes command-station catalogue changes.
type CommandStationDccSyncPort interface {
	ObserveCommandStationCatalog(ctx context.Context, commandStationID uint) error
}

// CommandStationSessionPort broadcasts command-station catalogue changes.
type CommandStationSessionPort interface {
	BroadcastCommandStationCatalogChanged(ctx context.Context, commandStation domain.CommandStation)
}

// InterlockingOccupancyAuthPort computes effective layout roles.
type InterlockingOccupancyAuthPort interface {
	Effective(ctx context.Context, user domain.User, layoutID uint) (domain.EffectiveRoles, error)
}

// InterlockingOccupancyHubPort fans out occupant changes.
type InterlockingOccupancyHubPort interface {
	BroadcastOccupantChanged(layoutID, interlockingID uint, occupant *OccupantInfo, reason string)
}

// InterlockingOccupancyPresencePort refreshes layout presence snapshots.
type InterlockingOccupancyPresencePort interface {
	RefreshAndBroadcast(ctx context.Context, layoutID uint)
}

// InterlockingOccupancyTakeoverPort releases active takeovers for a signalman.
type InterlockingOccupancyTakeoverPort interface {
	ReleaseAllForSignalman(ctx context.Context, signalmanUserID uint, reason string) error
}

// PresenceOnlineUser is a deduplicated live WS user.
type PresenceOnlineUser struct {
	UserID uint
	Login  string
}

// PresenceAuthPort computes the role label displayed on presence rows.
type PresenceAuthPort interface {
	EffectiveDisplayRole(ctx context.Context, user domain.User, layoutID uint) (domain.Role, error)
}

// PresenceHubPort supplies online users and fans out presence snapshots.
type PresenceHubPort interface {
	OnlineUsers(layoutID uint) []PresenceOnlineUser
	BroadcastPresenceChanged(layoutID uint, users []domain.PresenceUser)
}
