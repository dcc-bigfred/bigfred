### 7a.2 Effective roles

Effective roles are evaluated **in the context of the user's active
party**, because the `signalman` role is party-scoped (see §3a.4 and
goal 12). The set is:

```
effective(user, party) =
        { user.role }                                                       // system role: driver | admin
      ∪ { t.role : t ∈ user.TempRoles, t.expires_at > now() }                // global temporary grants
      ∪ ({ "signalman" } if (user, party) ∈ PartySignalman                   // party-scoped grant
                          and the matching row has not expired
                          else ∅)
```

Notes:

- `user.role` may be `driver` or `admin`; it is **not** `signalman`.
  The `signalman` role only exists as a party-scoped grant.
- When `party` is the system-provided `default`, the rule still
  applies: an admin can grant signalman inside `default` just like in
  any other party.
- The MCP path passes `partyID` through the API key context (each key
  is bound to the party that was active when the key was minted, see
  §7b.1); this keeps role evaluation deterministic for non-interactive
  callers.

`AuthService.Effective(ctx, partyID)` returns this set; middleware uses
it. Callers that do not yet have a party (the login screen and the
party-list endpoint) get an "anonymous in party" identity that only
includes the system role plus global temporary grants.
