package domain

import "time"

// SudoElevation is a short-lived (default 2 minutes), layout-scoped
// self-grant of the `admin` role obtained by typing the layout admin
// PIN (§7a.7). The grant is **always for the admin role** — the
// signalman icon next to the padlock writes a permanent
// `LayoutSignalman` row instead, so the sudo path stays single-purpose.
//
// Invariants enforced by the service + DB layer:
//
//   - exactly one ACTIVE row per (UserID, LayoutID); the unique index
//     in the DB and the upsert in the repo guarantee it. A second
//     `Sudo` call while the row is still live simply pushes
//     ExpiresAt forward (the "renew the timer" semantic);
//   - ExpiresAt is set to `now + cfg.SudoTTL`;
//   - the row is created ONLY by `SudoService.Sudo` after a
//     successful PIN verification against `Layout.AdminPINHash`.
//     Sudo is always a self-grant — there is no admin-side
//     "grant sudo to user X" path.
type SudoElevation struct {
	ID        uint
	UserID    uint      `db:"user_id"`
	LayoutID  uint      `db:"layout_id"`
	GrantedAt time.Time `db:"granted_at"`
	ExpiresAt time.Time `db:"expires_at"`
}

// Table tells REL which physical table backs this struct.
func (SudoElevation) Table() string { return "sudo_elevations" }

// IsActive reports whether the row is still in effect at `now`.
func (e SudoElevation) IsActive(now time.Time) bool {
	return e.ExpiresAt.After(now)
}

// EffectiveRoles is the result of computing every role membership
// for `(user, layout)` at a moment in time (§7a.2). The struct is
// flat on purpose: a sudo admin and a permanent admin grant the same
// authority everywhere, so the policy layer asks the single question
// `Has(role)` and never needs to distinguish "where did this role
// come from".
type EffectiveRoles struct {
	roles map[Role]struct{}
}

// NewEffectiveRoles constructs a set out of the supplied roles. The
// service layer is the only intended caller.
func NewEffectiveRoles(roles ...Role) EffectiveRoles {
	out := EffectiveRoles{roles: make(map[Role]struct{}, len(roles))}
	for _, r := range roles {
		if r == "" {
			continue
		}
		out.roles[r] = struct{}{}
	}
	return out
}

// Has reports whether the role is currently effective, regardless of
// the source (permanent, layout-scoped grant, or sudo elevation).
func (e EffectiveRoles) Has(r Role) bool {
	if e.roles == nil {
		return false
	}
	_, ok := e.roles[r]
	return ok
}
