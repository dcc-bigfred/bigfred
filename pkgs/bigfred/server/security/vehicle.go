package security

import (
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// VehicleSecurityContext gates catalogue mutations on domain.Vehicle
// (§7a.3). Callers construct it with a zero value.
type VehicleSecurityContext struct{}

// CanMutateVehicle decides whether actor may update or delete a
// vehicle row. The owner may always mutate their own catalogue entry;
// an effective admin (permanent or sudo in the active layout, §7a.7)
// may mutate any vehicle.
func (VehicleSecurityContext) CanMutateVehicle(
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
