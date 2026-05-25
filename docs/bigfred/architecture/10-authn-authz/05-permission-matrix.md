### 7a.5 Permission matrix

| Capability                                  | driver (own) | driver (leased) | signalman (idle) | signalman (active takeover) | admin (permanent) | admin (sudo) | signalman (sudo) |
|---------------------------------------------|:------------:|:---------------:|:----------------:|:---------------------------:|:-----------------:|:------------:|:----------------:|
| Drive vehicle / train                       | ✅            | ✅               | ❌                | ✅                           | ❌¹                | ❌¹           | ❌                |
| Edit vehicle metadata, write CV             | ✅            | ❌               | ❌                | ❌                           | ❌¹                | ❌¹           | ❌                |
| Register vehicle (within own DCC pool)      | ✅            | n/a             | n/a              | n/a                         | ❌¹                | ❌¹           | n/a              |
| Register vehicle outside the user's DCC pool³ | ❌            | n/a             | n/a              | n/a                         | ✅                 | ✅            | n/a              |
| Create / edit train                         | ✅            | ❌               | ❌                | ❌                           | ❌¹                | ❌¹           | ❌                |
| Lease out a vehicle / train                 | ✅            | ❌               | ❌                | ❌                           | ❌¹                | ❌¹           | ❌                |
| Occupy an interlocking                      | ❌            | ❌               | ✅                | ✅                           | ❌¹                | ❌¹           | ✅                |
| Request takeover                            | ❌            | ❌               | ❌                | ✅²                          | ❌¹                | ❌¹           | ✅²               |
| Add an interlocking to the layout whitelist  | ❌            | ❌               | ✅                | ✅                           | ✅                 | ✅            | ✅                |
| Manage users, roles, DCC pools              | ❌            | ❌               | ❌                | ❌                           | ✅                 | ✅            | ❌                |
| **Edit layout settings⁴**                    | ❌            | ❌               | ❌                | ❌                           | ✅                 | **❌⁵**       | ❌                |
| **Rotate the layout admin PIN**              | ❌            | ❌               | ❌                | ❌                           | ✅                 | **❌⁵**       | ❌                |
| Lock / unlock layout                        | ❌            | ❌               | ❌                | ❌                           | ✅                 | **❌⁵**       | ❌                |
| Attach / detach command stations on layout   | ❌            | ❌               | ❌                | ❌                           | ✅                 | **❌⁵**       | ❌                |
| Grant / revoke layout-scoped signalmen       | ❌            | ❌               | ❌                | ❌                           | ✅                 | **❌⁵**       | ❌                |
| Delete layout                                | ❌            | ❌               | ❌                | ❌                           | ✅                 | **❌⁵**       | ❌                |
| Read audit log                               | ❌            | ❌               | ❌                | ❌                           | ✅                 | ✅            | ❌                |
| Self-elevate via layout admin PIN (`sudo`)   | ✅            | ✅               | ✅                | ✅                           | ✅                 | n/a          | n/a              |

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
   AND admin-PIN rotation. The first six are organisational decisions
   that should outlast the 2-minute sudo window; the seventh is the
   "lock the real admin out" trap that motivates the entire exception.
⁵ Denied with `requires_non_sudo_admin`. This is the single
   asymmetry between permanent and sudo `admin` and is the entire
   point of the sudo concept (§7a.7): a sudo user can do **operational
   admin work** in a club room (register a guest loco, grant a
   one-shot temporary role, …) but cannot make **organisational**
   changes to the layout itself.
