package security

import (
	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// RadioStopSecurityContext gates layout-wide Radio Stop (§4.6.2).
type RadioStopSecurityContext struct{}

// CanTrigger reports whether userID may fire Radio Stop on the layout.
// Authorization passes on either drive scope (userID appears in
// controllerUserIds of a roster vehicle) or the layout-scoped signalman
// role. Permanent admin alone is not sufficient (§4.6.2).
func (RadioStopSecurityContext) CanTrigger(eff domain.EffectiveRoles, userID uint, roster contract.AllowedVehicles) Decision {
	if eff.Has(domain.RoleSignalman) {
		return Allow
	}
	for _, v := range roster.Vehicles {
		for _, id := range v.ControllerUserIDs {
			if id == userID {
				return Allow
			}
		}
	}
	return Deny(ReasonNotAuthorizedToDrive)
}
