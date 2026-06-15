package security

import "github.com/keskad/loco/pkgs/bigfred/server/domain"

// RadioSecurityContext gates walkie-talkie send and replay (§4.4).
type RadioSecurityContext struct{}

// CanSend reports whether the caller may emit a radio message with the
// given target. Signalmen address drivers; drivers address interlockings.
func (RadioSecurityContext) CanSend(eff domain.EffectiveRoles, toUserID, toInterlockingID uint) Decision {
	if toUserID != 0 {
		if eff.Has(domain.RoleSignalman) {
			return Allow
		}
		return Deny("not_signalman")
	}
	if toInterlockingID != 0 {
		return Allow
	}
	return Deny("radio_invalid_target")
}

// CanReplayInterlocking reports whether the caller may read the group
// chat history for interlockingID. Only the active occupant qualifies.
func (RadioSecurityContext) CanReplayInterlocking(
	eff domain.EffectiveRoles,
	interlockingID uint,
	occupantUserID uint,
	callerUserID uint,
) Decision {
	if interlockingID == 0 {
		return Deny("invalid_interlocking")
	}
	if !eff.Has(domain.RoleSignalman) {
		return Deny("not_signalman")
	}
	if occupantUserID == 0 || occupantUserID != callerUserID {
		return Deny("not_interlocking_occupant")
	}
	return Allow
}

// CanReplayUser allows every authenticated user to read their own stream.
func (RadioSecurityContext) CanReplayUser() Decision {
	return Allow
}
