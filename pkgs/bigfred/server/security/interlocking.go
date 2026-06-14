package security

import (
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// InterlockingSecurityContext evaluates interlocking occupation rules
// (§7a.3).
type InterlockingSecurityContext struct{}

// CanOccupy decides whether the caller may staff ilk inside layout
// when layoutILK is the whitelist row (non-nil) and current is the
// active session on the box, if any. `eff` is the caller's effective
// role set inside the layout (§7a.2) — sudo admin counts the same as
// a permanent admin or layout signalman grant.
func (InterlockingSecurityContext) CanOccupy(
	eff domain.EffectiveRoles,
	actorID uint,
	layoutILK *domain.LayoutInterlocking,
	current *domain.InterlockingSession,
) Decision {
	if layoutILK == nil {
		return Deny("interlocking_not_in_layout")
	}
	if !eff.Has(domain.RoleSignalman) && !eff.Has(domain.RoleAdmin) {
		return Deny("not_signalman")
	}
	if current != nil && current.SignalmanUserID == actorID {
		return Allow
	}
	if current != nil {
		return Deny("interlocking_occupied")
	}
	return Allow
}

// CanDisplace allows a signalman (or admin, including sudo) to take
// over an already-staffed box when the client sends force:true.
func (InterlockingSecurityContext) CanDisplace(
	eff domain.EffectiveRoles,
	current *domain.InterlockingSession,
	actorID uint,
) Decision {
	if !eff.Has(domain.RoleSignalman) && !eff.Has(domain.RoleAdmin) {
		return Deny("not_signalman")
	}
	if current == nil {
		return Allow
	}
	if current.SignalmanUserID == actorID {
		return Allow
	}
	return Allow
}

// CanManageCatalog decides whether the caller may create, update or
// delete rows in the global interlocking catalogue (§7a.3). Only an
// effective admin (permanent or sudo in the active layout, §7a.7)
// qualifies.
func (InterlockingSecurityContext) CanManageCatalog(eff domain.EffectiveRoles) Decision {
	if eff.Has(domain.RoleAdmin) {
		return Allow
	}
	return Deny("forbidden")
}

// IsSignalmanGrantActive reports whether the layout grant is valid at
// now.
func IsSignalmanGrantActive(grant domain.LayoutSignalman, now time.Time) bool {
	if grant.ExpiresAt != nil && !grant.ExpiresAt.After(now) {
		return false
	}
	return true
}
