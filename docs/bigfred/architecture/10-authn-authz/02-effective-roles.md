### 7a.2 Effective roles

Effective roles are evaluated **in the context of the user's active
layout**, because the `signalman` role is layout-scoped (see §3a.4 and
goal 12). The set is:

```
effective(user, layout) =
        { user.role }                                                       // system role: driver | admin
      ∪ { t.role : t ∈ user.TempRoles, t.expires_at > now() }                // global temporary grants
      ∪ ({ "signalman" } if (user, layout) ∈ LayoutSignalman                   // layout-scoped grant
                          and the matching row has not expired
                          else ∅)
```

Notes:

- `user.role` may be `driver` or `admin`; it is **not** `signalman`.
  The `signalman` role only exists as a layout-scoped grant.
- When `layout` is the system-provided `default`, the rule still
  applies: an admin can grant signalman inside `default` just like in
  any other layout.
- The MCP path passes `layoutID` through the API key context (each key
  is bound to the layout that was active when the key was minted, see
  §7b.1); this keeps role evaluation deterministic for non-interactive
  callers.

`AuthService.Effective(ctx, layoutID)` returns this set; middleware uses
it. Because the layout is picked **on the login form** (§7a.1) and
baked into the JWT, every authenticated request already carries a
`layoutID` and the "anonymous in layout" identity is no longer needed.
The only endpoint that runs without a `layoutID` is the unauthenticated
`GET /api/v1/layouts/login` used to populate the login dropdown
itself; it never inspects roles.
