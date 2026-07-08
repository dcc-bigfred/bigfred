package remotes

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// ThrottleActor identifies the user behind one remote handset session.
type ThrottleActor struct {
	UserID    uint
	SessionID string
	// Source is the remote protocol ("z21" or "withrottle") when the actor
	// originates from an inbound handset; empty for internal callers.
	Source string
}

// ThrottleResponder receives per-loco replies for one handset.
type ThrottleResponder interface {
	Subscribe(addrs ...uint16)
	Unsubscribe(addrs ...uint16)
	SubscribedAddrs() []uint16
	OldestSubscribed() (uint16, bool)
	SendLocoState(ctx context.Context, snap contract.LocoStateWire) error
	SendLocoError(ctx context.Context, addr uint16, code, detail string) error
}

// CommandResult is the outcome of a handset throttle action.
type CommandResult struct {
	OK   bool
	Code string
}

// HandsetDrivePort covers handset-specific safety and authorization.
type HandsetDrivePort interface {
	AuthorizeDrive(userID uint, addr uint16, scope DriveScope) bool
	CollectHandsetDriveTargets(ctx context.Context, userID uint, subscribed []uint16, scope DriveScope) []uint16
	ApplyHandsetIdleBrake(ctx context.Context, session HandsetSession, subscribed []uint16, scope DriveScope)
	ApplyHandsetPilotEStop(ctx context.Context, session HandsetSession, addr uint16)
	TriggerLayoutRadioStop(ctx context.Context, userID uint, source string) error
	ReadLocoCV(addr uint16, cvNum commandstation.CVNum) (int, error)
}

// InboundDrivePort is the full drive surface used by inbound remotes.
type InboundDrivePort interface {
	HandsetDrivePort
	SetSpeed(ctx context.Context, actor ThrottleActor, resp ThrottleResponder, req contract.LocoSetSpeedWire) CommandResult
	SetFunction(ctx context.Context, actor ThrottleActor, resp ThrottleResponder, req contract.LocoSetFunctionWire) CommandResult
	Subscribe(ctx context.Context, actor ThrottleActor, resp ThrottleResponder, addrs []uint16) CommandResult
	// LocoSnapshot returns the current merged state for an address so a
	// gateway can echo it straight back to the commanding handset.
	LocoSnapshot(addr uint16) contract.LocoStateWire
}
