package security

import "github.com/keskad/loco/pkgs/bigfred/contract"

// RadioStopSecurityContext gates layout-wide Radio Stop (§4.6.2).
type RadioStopSecurityContext struct{}

// CanTrigger reports whether userID may fire Radio Stop on the layout.
// The user must appear in ControllerUserIDs of at least one drivable
// roster vehicle (owner, lessee, or temporary driver grant).
func (RadioStopSecurityContext) CanTrigger(userID uint, roster contract.AllowedVehicles) Decision {
	for _, v := range roster.Vehicles {
		for _, id := range v.ControllerUserIDs {
			if id == userID {
				return Allow
			}
		}
	}
	return Deny("not_authorized_to_drive")
}
