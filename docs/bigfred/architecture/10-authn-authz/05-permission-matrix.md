### 7a.5 Permission matrix

| Capability                                  | driver (own) | driver (leased) | signalman (idle) | signalman (active takeover) | admin (permanent) | admin (sudo) |
|---------------------------------------------|:------------:|:---------------:|:----------------:|:---------------------------:|:-----------------:|:------------:|
| Drive vehicle / train                       | ✅            | ✅               | ❌                | ✅                           | ❌¹                | ❌¹           |
| Trigger Radio Stop (layout-wide halt, §4.6)   | ✅            | ✅               | ❌                | ✅                           | ❌¹                | ❌¹           |
| Edit vehicle metadata, write CV             | ✅            | ❌               | ❌                | ❌                           | ❌¹                | ❌¹           |
| Register vehicle (within own DCC pool)      | ✅            | n/a             | n/a              | n/a                         | ❌¹                | ❌¹           |
| Register vehicle outside the user's DCC pool³ | ❌            | n/a             | n/a              | n/a                         | ✅                 | ✅            |
| Create / edit train                         | ✅            | ❌               | ❌                | ❌                           | ❌¹                | ❌¹           |
| Lease out a vehicle / train                 | ✅            | ❌               | ❌                | ❌                           | ❌¹                | ❌¹           |
| Occupy an interlocking                      | ❌            | ❌               | ✅                | ✅                           | ❌¹                | ❌¹           |
| Request takeover                            | ❌            | ❌               | ❌                | ✅²                          | ❌¹                | ❌¹           |
| Add an interlocking to the layout whitelist  | ❌            | ❌               | ✅                | ✅                           | ✅                 | ✅            |
| Manage users, roles, DCC pools              | ❌            | ❌               | ❌                | ❌                           | ✅                 | ✅            |
| Edit layout settings⁴                        | ❌            | ❌               | ❌                | ❌                           | ✅                 | ✅            |
| Rotate the layout admin PIN                  | ❌            | ❌               | ❌                | ❌                           | ✅                 | ✅            |
| Lock / unlock layout                        | ❌            | ❌               | ❌                | ❌                           | ✅                 | ✅            |
| Attach / detach command stations on layout   | ❌            | ❌               | ❌                | ❌                           | ✅                 | ✅            |
| Grant / revoke layout-scoped signalmen       | ❌            | ❌               | ❌                | ❌                           | ✅                 | ✅            |
| Delete layout                                | ❌            | ❌               | ❌                | ❌                           | ✅                 | ✅            |
| Read audit log                               | ❌            | ❌               | ❌                | ❌                           | ✅                 | ✅            |
| Self-elevate via layout admin PIN (`sudo`)   | ✅            | ✅               | ✅                | ✅                           | ✅                 | n/a          |
| Self-grant permanent signalman via PIN      | ✅            | ✅               | n/a              | n/a                         | ✅                 | ✅            |

¹ `admin` is a management role only; if an admin also needs to drive,
   they must additionally hold the `driver` role (permanent or
   temporary).
² Takeover is only available to the signalman currently occupying an
   interlocking; idle signalmen do not have this power.
³ A permanent `admin` (or a sudo-elevated one) may register a vehicle
   that falls **outside** any DCC-pool – this is a deliberate
   operational override for troubleshooting (e.g. registering a guest
   loco mid-session). `LocoSecurityContext.CanRegisterLoco` accepts
   `domain.EffectiveRoles` and short-circuits to `Allow` when the
   actor `Has(domain.RoleAdmin)`.
⁴ "Layout settings" covers the operations gated by
   `LayoutSecurityContext.CanEditLayout` and friends in §7a.3:
   rename, lock/unlock, command-station attach/detach, layout-scoped
   signalmen list, interlocking whitelist removal, layout deletion
   and admin-PIN rotation. A sudo admin grants the same authority as
   a permanent admin everywhere — the 2-minute window plus the
   rate-limiter on the PIN dialog are the only guard rails (§7a.7).

The signalman icon next to the padlock (§7a.7) is **not** a sudo
elevation: it writes a permanent `LayoutSignalman` row with
`expires_at = NULL`, so a user that promotes themselves keeps the
signalman role inside that layout until they (or an admin) revoke
it. The matrix row "signalman (idle)" / "signalman (active takeover)"
applies to such a user from the moment the row is persisted.
