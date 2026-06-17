package security

import (
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// DCCPoolSecurityContext gates per-user DCC address pool mutations
// (§7a.3). Only an effective admin (permanent or sudo in the active
// layout, §7a.7) may replace or delete pool rows.
type DCCPoolSecurityContext struct{}

// CanManageDCCPool decides whether the caller may replace or delete
// DCC address pool rows for any user.
func (DCCPoolSecurityContext) CanManageDCCPool(eff domain.EffectiveRoles) Decision {
	if eff.Has(domain.RoleAdmin) {
		return Allow
	}
	return Deny(ReasonForbidden)
}
