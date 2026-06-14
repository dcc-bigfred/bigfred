### 7a.2 Effective roles

Effective roles are evaluated **in the context of the user's active
layout**, because the `signalman` role is layout-scoped (see §3a.4 and
goal 12) and the `sudo` self-grant (§7a.7) is also layout-scoped.

Roles come from four sources. They flatten into a single set on the
returned struct: a sudo-elevated `admin` is indistinguishable from a
permanent `admin` for every authority check (§7a.7.6). The point of
sudo is to be a **temporary, self-served, full** admin promotion —
not a partial one.

```
sources(user, layout) =
        permanent       : { user.role }                                                     // user.role ∈ { driver, admin }
      + temp_grant      : { t.role : t ∈ user.TempRoles, t.expires_at > now() }              // global, admin-issued (§7a goal 3)
      + layout_signalman: ({ "signalman" } if (user, layout) ∈ LayoutSignalman              // admin-issued OR PIN-self-granted
                            and the matching row has not expired
                            else ∅)                                                         //   (the engineer's-cap icon writes a
                                                                                            //    permanent row here, §7a.7)
      + sudo            : ({ "admin" } if active SudoElevation for (user, layout)            // PIN-gated 2-min admin elevation
                            else ∅)                                                         //   (§7a.7)

effective(user, layout) = permanent ∪ temp_grant ∪ layout_signalman ∪ sudo                  // flat set of roles
```

The struct lives in `pkgs/bigfred/server/domain` because the policy layer
(§7a.3) is allowed to depend only on `pkgs/bigfred/server/domain` and `time`.
It is intentionally minimal — `Has(role)` is the single question
authority checks ask:

```go
// pkgs/bigfred/server/domain/sudo.go
package domain

// EffectiveRoles is the result of AuthService.Effective(ctx, layoutID).
// Permanent role, temporary grants, layout-scoped signalman grants
// and the sudo admin elevation all collapse onto the same set: a
// sudo admin grants the same authority as a permanent admin
// everywhere (§7a.7).
type EffectiveRoles struct {
    // unexported set; constructed by NewEffectiveRoles.
}

// NewEffectiveRoles constructs the membership set out of the
// supplied roles.
func NewEffectiveRoles(roles ...Role) EffectiveRoles

// Has reports whether the role is currently in effect, regardless of
// the source.
func (EffectiveRoles) Has(Role) bool
```

`AuthService.Effective(ctx, user, layoutID)` returns this struct; the
HTTP `RequireRole` middleware and the WebSocket dispatcher consult it
on every authority check.

Notes:

- `user.role` may be `driver` or `admin`; it is **not** `signalman`.
  The `signalman` role only exists as a layout-scoped grant — either
  set by an admin (§7a.5) OR self-granted via the engineer's-cap icon
  using the layout admin PIN (§7a.7). Both paths upsert into
  `LayoutSignalmen`; the icon path simply writes a row with
  `expires_at = NULL` to make the membership permanent inside the
  layout.
- When `layout` is the system-provided `default`, the rule still
  applies: an admin can grant signalman inside `default` just like in
  any other layout, and a user can sudo-elevate to `admin` inside
  `default` just like anywhere else.
- The MCP path passes `layoutID` through the API key context (each key
  is bound to the layout that was active when the key was minted, see
  §7b.1); this keeps role evaluation deterministic for non-interactive
  callers. **Sudo elevations are deliberately ignored for API-key
  callers** — an MCP/REST caller authenticating with a bearer key
  cannot self-elevate, because the PIN dialog is a UI-only affordance
  bound to a real human typing into a browser. Programmatic admin
  capabilities must come from a permanent or admin-granted temporary
  role.
- Because the layout is picked **on the login form** (§7a.1) and
  baked into the JWT, every authenticated request already carries a
  `layoutID` and the "anonymous in layout" identity is no longer
  needed. The only endpoint that runs without a `layoutID` is the
  unauthenticated `GET /api/v1/layouts/login` used to populate the
  login dropdown itself; it never inspects roles.
