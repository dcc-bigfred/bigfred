### 7a.2 Effective roles

Effective roles are evaluated **in the context of the user's active
layout**, because the `signalman` role is layout-scoped (see §3a.4 and
goal 12) and the `sudo` self-grants (§7a.7) are also layout-scoped.

Roles come from four independent sources, which the authorization layer
must keep distinguishable so policy decisions can refuse one source
while accepting another (the canonical example: a sudo-elevated
`admin` is allowed to drive operationally but is **not** allowed to
edit layout settings, §7a.7):

```
sources(user, layout) =
        permanent       : { user.role }                                                     // user.role ∈ { driver, admin }
      + temp_grant      : { t.role : t ∈ user.TempRoles, t.expires_at > now() }              // global, admin-issued (§7a goal 3)
      + layout_signalman: ({ "signalman" } if (user, layout) ∈ LayoutSignalman             // admin-issued grant inside this layout
                            and the matching row has not expired
                            else ∅)
      + sudo            : { e.target : e ∈ SudoElevation                                    // self-grant via the layout admin PIN
                            where e.user_id = user.id
                            and e.layout_id = layout.id
                            and e.expires_at > now() }                                       // §7a.7

effective(user, layout) =
        permanent ∪ temp_grant ∪ layout_signalman ∪ sudo                                    // flattened set of roles
```

The four sources do not collapse into a flat `[]Role` set – the
authorization layer carries them as a richer struct so policies can
ask "is this `admin` membership sudo-only?" without re-querying the
database. Because the policy layer (§7a.3) is allowed to depend
**only** on `pkgs/server/domain` and `time`, the struct lives in
`pkgs/server/domain`:

```go
// pkgs/server/domain/effective_roles.go
package domain

// EffectiveRoles is the structured result of
// AuthService.Effective(ctx, layoutID). The four maps are disjoint
// by source – the same role may appear in multiple maps when the
// user holds it through multiple paths (e.g. permanent admin AND a
// sudo admin elevation simultaneously).
type EffectiveRoles struct {
    Permanent       Role                 // user.Role
    TempGrants      map[Role]struct{}    // active TemporaryRole rows
    LayoutSignalman map[Role]struct{}    // ∈ {{}, { "signalman" }}
    Sudo            map[Role]struct{}    // active SudoElevation rows for this layout
}

// Has returns true when the role is held through ANY source.
func (e EffectiveRoles) Has(r Role) bool

// HasNonSudo returns true when the role is held through a source
// OTHER than `sudo`. Policies that should refuse sudo-elevated
// callers (layout settings edits, admin PIN rotation, …) ask this.
func (e EffectiveRoles) HasNonSudo(r Role) bool

// IsSudoOnly returns true when the role is in the effective set AND
// every source that supplies it is `sudo`. Symmetric with HasNonSudo:
//   e.Has(r) == e.HasNonSudo(r) || e.IsSudoOnly(r)
func (e EffectiveRoles) IsSudoOnly(r Role) bool
```

`AuthService.Effective(ctx, layoutID)` returns this struct; the HTTP
middleware and the WebSocket dispatcher store it on `auth.Identity`
and pass it to every policy method that needs the discrimination (see
§7a.3).

Notes:

- `user.role` may be `driver` or `admin`; it is **not** `signalman`.
  The `signalman` role only exists as a layout-scoped grant or a
  sudo self-grant.
- When `layout` is the system-provided `default`, the rule still
  applies: an admin can grant signalman inside `default` just like in
  any other layout, and a user can sudo-elevate to `admin` or
  `signalman` inside `default` just like anywhere else.
- The MCP path passes `layoutID` through the API key context (each key
  is bound to the layout that was active when the key was minted, see
  §7b.1); this keeps role evaluation deterministic for non-interactive
  callers. **Sudo elevations are deliberately ignored for API-key
  callers** – an MCP/REST caller authenticating with a bearer key
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
