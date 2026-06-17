package security

import (
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// LayoutSecurityContext is the stateless policy struct that gates
// layout-scoped decisions (§7a.3). Callers construct it with a zero
// value: `var sec security.LayoutSecurityContext`.
type LayoutSecurityContext struct{}

// CanRemoveVehicleFromRoster decides whether actor may detach vehicle
// from the layout roster. The owner may always remove their own
// vehicle; an effective admin (permanent or sudo, §7a.7) may remove
// any vehicle from the roster.
func (LayoutSecurityContext) CanRemoveVehicleFromRoster(
	eff domain.EffectiveRoles,
	actorID, ownerUserID uint,
) Decision {
	if eff.Has(domain.RoleAdmin) {
		return Allow
	}
	if actorID == ownerUserID {
		return Allow
	}
	return Deny(ReasonVehicleNotOwned)
}

// CanRemoveTrainFromRoster is the train-shaped sibling of
// CanRemoveVehicleFromRoster.
func (LayoutSecurityContext) CanRemoveTrainFromRoster(
	eff domain.EffectiveRoles,
	actorID, ownerUserID uint,
) Decision {
	if eff.Has(domain.RoleAdmin) {
		return Allow
	}
	if actorID == ownerUserID {
		return Allow
	}
	return Deny(ReasonTrainNotOwned)
}

// CanGrantSignalmanToUser allows an effective admin (permanent or
// sudo, §7a.7) to grant the layout-scoped signalman role to another
// user.
func (LayoutSecurityContext) CanGrantSignalmanToUser(eff domain.EffectiveRoles) Decision {
	if eff.Has(domain.RoleAdmin) {
		return Allow
	}
	return Deny(ReasonForbidden)
}

// CanManageLayouts decides whether the caller may create, update or
// delete layout rows (§7a.3). Only an effective admin (permanent or
// sudo in the active layout, §7a.7) qualifies.
func (LayoutSecurityContext) CanManageLayouts(eff domain.EffectiveRoles) Decision {
	if eff.Has(domain.RoleAdmin) {
		return Allow
	}
	return Deny(ReasonForbidden)
}
