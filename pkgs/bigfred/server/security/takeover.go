package security

import "github.com/keskad/loco/pkgs/bigfred/server/domain"

// TakeoverSecurityContext gates takeover.request (§4.3).
type TakeoverSecurityContext struct{}

// CanRequest reports whether the caller may request takeover of a target
// while occupying interlockingID.
func (TakeoverSecurityContext) CanRequest(
	eff domain.EffectiveRoles,
	occupantUserID uint,
	callerUserID uint,
) Decision {
	if !eff.Has(domain.RoleSignalman) {
		return Deny("not_signalman")
	}
	if occupantUserID == 0 || occupantUserID != callerUserID {
		return Deny("not_interlocking_occupant")
	}
	return Allow
}
