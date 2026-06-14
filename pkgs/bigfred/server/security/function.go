package security

import "github.com/keskad/loco/pkgs/bigfred/server/domain"

// FunctionSecurityContext gates function-definition mutations (§7a.3).
type FunctionSecurityContext struct{}

// CanEditVehicleFunctions allows only the vehicle owner (not lessees,
// not signalmen with takeover).
func (FunctionSecurityContext) CanEditVehicleFunctions(actorID, ownerUserID uint) Decision {
	if actorID == ownerUserID {
		return Allow
	}
	return Deny("only_owner_can_edit")
}

// CanEditTemplateFunctions allows the template owner or an effective admin.
func (FunctionSecurityContext) CanEditTemplateFunctions(
	eff domain.EffectiveRoles,
	actorID, ownerUserID uint,
) Decision {
	if eff.Has(domain.RoleAdmin) {
		return Allow
	}
	if actorID == ownerUserID {
		return Allow
	}
	return Deny("template_not_owned")
}
