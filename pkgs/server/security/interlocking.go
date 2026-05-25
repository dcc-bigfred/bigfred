package security

import (
	"time"

	"github.com/keskad/loco/pkgs/server/domain"
)

// InterlockingSecurityContext evaluates interlocking occupation rules
// (§7a.3).
type InterlockingSecurityContext struct{}

// CanOccupy decides whether actor may staff ilk inside layout when
// layoutILK is the whitelist row (non-nil) and current is the active
// session on the box, if any.
func (InterlockingSecurityContext) CanOccupy(
	actor domain.User,
	layoutILK *domain.LayoutInterlocking,
	layoutSignalman *domain.LayoutSignalman,
	current *domain.InterlockingSession,
) Decision {
	if layoutILK == nil {
		return Deny("interlocking_not_in_layout")
	}
	if layoutSignalman == nil {
		return Deny("not_signalman")
	}
	if current != nil && current.SignalmanUserID == actor.ID {
		return Allow
	}
	if current != nil {
		return Deny("interlocking_occupied")
	}
	return Allow
}

// CanDisplace allows a signalman to take over an already-staffed box
// when the client sends force:true.
func (InterlockingSecurityContext) CanDisplace(
	actor domain.User,
	layoutSignalman *domain.LayoutSignalman,
	current *domain.InterlockingSession,
) Decision {
	if layoutSignalman == nil {
		return Deny("not_signalman")
	}
	if current == nil {
		return Allow
	}
	if current.SignalmanUserID == actor.ID {
		return Allow
	}
	return Allow
}

// IsSignalmanGrantActive reports whether the layout grant is valid at
// now.
func IsSignalmanGrantActive(grant domain.LayoutSignalman, now time.Time) bool {
	if grant.ExpiresAt != nil && !grant.ExpiresAt.After(now) {
		return false
	}
	return true
}
