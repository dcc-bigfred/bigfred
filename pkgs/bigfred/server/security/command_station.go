package security

import "github.com/keskad/loco/pkgs/bigfred/server/domain"

// CommandStationSecurityContext evaluates catalogue CRUD authority
// (§4.1 / §7a.3 — same surface as interlockings: permanent admin or
// sudo admin).
type CommandStationSecurityContext struct{}

// CanManageCatalog decides whether the caller may create, update or
// delete command_stations rows.
func (CommandStationSecurityContext) CanManageCatalog(eff domain.EffectiveRoles) Decision {
	if eff.Has(domain.RoleAdmin) {
		return Allow
	}
	return Deny("forbidden")
}
