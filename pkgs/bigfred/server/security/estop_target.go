package security

import "github.com/keskad/loco/pkgs/bigfred/server/domain"

// EStopTargetSecurityContext gates per-target emergency stop (§4.2).
type EStopTargetSecurityContext struct{}

// CanStop reports whether userID may brake the given roster target.
// Authorization passes when the caller is the target owner, an active
// lessee/controller, or the signalman currently occupying an
// interlocking on the layout.
func (EStopTargetSecurityContext) CanStop(
	eff domain.EffectiveRoles,
	userID uint,
	isInterlockingOccupant bool,
	targetOwnerID uint,
	controllerUserIDs []uint,
) Decision {
	if userID == targetOwnerID {
		return Allow
	}
	for _, id := range controllerUserIDs {
		if id == userID {
			return Allow
		}
	}
	if eff.Has(domain.RoleSignalman) && isInterlockingOccupant {
		return Allow
	}
	return Deny(ReasonNotAuthorizedToStop)
}
