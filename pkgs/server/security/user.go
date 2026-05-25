package security

import (
	"github.com/keskad/loco/pkgs/server/domain"
)

// UserSecurityContext is the stateless policy struct that gates every
// "is the actor allowed to do X to the user catalogue?" decision
// (§7a.3 / §7a.5). It carries no fields and no constructor inputs —
// callers construct it with a zero value: `var sec security.UserSecurityContext`.
//
// The policy never touches the database; it operates purely on the
// already-loaded `domain.User` records the caller passes in.
type UserSecurityContext struct{}

// CanManageUsers gates the entire user-management surface — listing,
// creating, editing, deactivating, deleting users. Only an effective
// admin (permanent or sudo in the active layout, §7a.5 / §7a.7)
// qualifies; signalman and driver grants do not escalate.
func (UserSecurityContext) CanManageUsers(eff domain.EffectiveRoles) Decision {
	if eff.Has(domain.RoleAdmin) {
		return Allow
	}
	return Deny("forbidden")
}

// CanDeactivateSelf prevents an admin from locking themselves out by
// deactivating their own account. The HTTP layer composes this on
// top of CanManageUsers when the actor and target IDs match.
func (UserSecurityContext) CanDeactivateSelf(actor domain.User, target domain.User) Decision {
	if actor.ID == target.ID {
		return Deny("cannot_deactivate_self")
	}
	return Allow
}

// CanDeleteSelf mirrors CanDeactivateSelf for hard deletion. The
// extra guard exists because deactivation and deletion are two
// distinct UI affordances; the matching backend rejection code lets
// the UI render a precise tooltip in either case.
func (UserSecurityContext) CanDeleteSelf(actor domain.User, target domain.User) Decision {
	if actor.ID == target.ID {
		return Deny("cannot_delete_self")
	}
	return Allow
}
