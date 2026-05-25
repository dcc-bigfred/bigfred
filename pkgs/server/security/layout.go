package security

import (
	"github.com/keskad/loco/pkgs/server/domain"
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
	return Deny("vehicle_not_owned")
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
	return Deny("train_not_owned")
}
