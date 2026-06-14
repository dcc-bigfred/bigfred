### 7a.7 Sudo elevation – temporary `admin` and permanent self-granted `signalman` via the layout PIN

This section turns goal 20 ("Sudo elevation – temporary admin powers
gated by a layout-scoped PIN") and the `SudoElevation` /
`Layout.AdminPINHash` invariants of §3a.1 / §3a.3 into a concrete,
end-to-end flow. The mental model is borrowed directly from `sudo` on
Linux: an authenticated user types a PIN and gets elevated rights for
a short, fixed window.

The flow exposes **two icons** on the top `AppBar` (rendered in
`AppShell.tsx`, §6.3b), both gated by the **single** **layout admin
PIN** (§0 Terminology) of the user's active layout:

- a **closed-padlock icon** – self-grants temporary `admin` rights
  for a short window (default 2 min, server-configurable in
  `[1m, 10m]`). Click again to revoke immediately;
- an **engineer's-cap icon** (the signalman / *nastawniczy* icon
  reused from the takeover UI) – **permanent** self-grant of the
  layout-scoped `signalman` role. Same PIN gate, but the resulting
  membership has no TTL — it lives in `layout_signalmen` exactly
  like an admin-issued grant (§7a.5). Click the active icon to
  step down.

The two flows share the PIN dialog (`<SudoPinDialog>`) and the
backend rate-limiter, but live on **separate persistent rows** so
the two domains don't entangle:

| Icon                  | Storage                                | Lifetime          |
|-----------------------|----------------------------------------|-------------------|
| 🔒 padlock (admin)     | `sudo_elevations(user_id, layout_id)`  | `cfg.SudoTTL` (2m) |
| 🧑‍✈️ engineer's cap     | `layout_signalmen(layout_id, user_id)` with `expires_at = NULL` | permanent until DELETE |

#### 7a.7.1 Lifecycle of an admin elevation

```
  ┌─────────────────────────────────────────────────────────────────┐
  │  user clicks 🔒 on the AppBar                                    │
  │       │                                                           │
  │       ▼                                                           │
  │  POST /api/v1/layouts/{id}/sudo { pin }                          │
  │       │                                                           │
  │       │  PIN ok? ──┐                                              │
  │       │            ▼                                              │
  │       │       upsert SudoElevation { user, layout,                │
  │       │                              expiresAt = now + cfg.TTL }  │
  │       │            │                                              │
  │       │            ▼                                              │
  │       │       audit  auth.sudo_granted                            │
  │       │            │                                              │
  │       │            ▼                                              │
  │       │       fan-out auth.elevationChanged                       │
  │       │            │                                              │
  │       │            ▼                                              │
  │       │       UI flips icon to OPEN + countdown                   │
  │       │                                                           │
  │       │  PIN wrong? ─┐                                            │
  │       │              ▼                                            │
  │       │         bump in-memory rolling-window failure counter     │
  │       │              │                                            │
  │       │              ▼                                            │
  │       │         429 sudo_locked after N attempts                  │
  │       │              + audit auth.sudo_locked                     │
  │       │                                                           │
  │       ▼                                                           │
  │  expiry path:                                                     │
  │    janitor goroutine (every 10 s) finds rows with                 │
  │    ExpiresAt <= now()                                             │
  │       │                                                           │
  │       ▼                                                           │
  │    DELETE the row                                                 │
  │       │                                                           │
  │       ▼                                                           │
  │    fan-out auth.elevationChanged                                  │
  │       │                                                           │
  │       ▼                                                           │
  │    UI flips icon back to CLOSED                                   │
  └─────────────────────────────────────────────────────────────────┘
```

The same teardown path also runs on:

- **explicit user revoke** – the user clicks the open padlock or
  fires `DELETE /api/v1/layouts/{id}/sudo`. Idempotent;
  `auth.sudo_revoked { reason:"user_action" }`.
- **logout** – `AuthService.Logout` deletes the caller's sudo row
  (if any) for the JWT-pinned layout in the same transaction as the
  cookie clearing. `auth.elevationChanged` is broadcast to every
  other live session of the user before the current session
  disconnects.
- **layout deletion** – `LayoutService.Delete` cascades to the
  `SudoElevation` rows pointing at the layout. In practice deletion
  is rejected with `409 layout_in_use` whenever any session is still
  pinned to the layout, so this branch only fires for layouts no
  user is currently in.

A second click on an already-elevated padlock while the row is still
live is treated as a **renewal**, not a duplicate insert: the row's
`ExpiresAt` is bumped to `now() + cfg.SudoTTL`. This matches Linux
`sudo` semantics, where re-typing the PIN inside the grace window
resets the timer.

#### 7a.7.2 Lifecycle of a signalman self-grant

```
  user clicks 🧑‍✈️ on the AppBar (idle state)
       │
       ▼
  POST /api/v1/layouts/{id}/signalman { pin }
       │
       │  PIN ok? ──► upsert layout_signalmen { user, layout,
       │                expires_at = NULL, granted_by = user.id }
       │              audit + auth.elevationChanged
       │
       │  PIN wrong? ─► same rate-limiter as the padlock flow
       ▼
  user clicks 🧑‍✈️ again (active state)
       │
       ▼
  DELETE /api/v1/layouts/{id}/signalman
       │
       ▼
  drop the row + audit + auth.elevationChanged
```

Because the row is permanent, an admin-side revoke
(`DELETE /api/v1/layouts/{id}/signalmen/{userId}`, §4.1) drops the
self-granted row exactly the same way as an admin-issued one. There
is no separate "self-revoke" privilege check — the icon simply
targets the row keyed by the caller's own `user_id`.

#### 7a.7.3 The `SudoService` surface

```go
// pkgs/bigfred/server/service/sudo.go
package service

// SudoConfig groups the few knobs the service exposes. Defaults
// match §7a.7 of the spec.
type SudoConfig struct {
    TTL             time.Duration // default 2*time.Minute
    FailWindow      time.Duration // rolling window for the failure counter
    MaxFailures     int           // default 5; trips the lockout
    LockDuration    time.Duration // how long a tripped lockout lasts
    JanitorInterval time.Duration // how often the reap loop wakes up
}

// Sudo verifies the PIN against Layout.AdminPINHash and, on success,
// upserts a SudoElevation row for (caller, layout). On mismatch it
// bumps the per-(userId, layoutId) failure counter; after
// cfg.MaxFailures failures inside cfg.FailWindow the tuple is
// soft-locked for cfg.LockDuration. Returns the persisted row so
// the HTTP layer can echo `expiresAt`.
//
// Sudo is **always a self-grant**: there is no admin-side `Grant
// sudo to user X` path. The actor and the elevated user are the
// same `domain.User`.
func (s *SudoService) Sudo(
    ctx context.Context, userID, layoutID uint, pin string,
) (domain.SudoElevation, error)

// Revoke deletes the SudoElevation row for (userID, layoutID).
// Idempotent.
func (s *SudoService) Revoke(ctx context.Context, userID, layoutID uint) error

// GrantSignalman verifies the layout admin PIN and persists a
// PERMANENT signalman grant by upserting a `layout_signalmen` row
// with `ExpiresAt = nil`. Same PIN-rate-limiter as Sudo.
func (s *SudoService) GrantSignalman(
    ctx context.Context, userID, layoutID uint, pin string,
) error

// RevokeSignalman drops the user's signalman grant in the layout.
// Idempotent.
func (s *SudoService) RevokeSignalman(ctx context.Context, userID, layoutID uint) error
```

The PIN itself never leaves `Sudo` / `GrantSignalman`: each function
argon2id-verifies `pin` against `Layout.AdminPINHash` (the same
column rotated by `LayoutService.UpdateAdminPIN`, see §7a.7.5), and
the plaintext is overwritten in memory before the function returns.

#### 7a.7.4 Janitor goroutine

Sudo expiry shares the periodic janitor goroutine introduced in §7
(cross-cutting concern 9 "Time-based grants cleanup"). Every
`cfg.JanitorInterval` (default 10 s) the goroutine runs, in addition
to its existing lease / takeover sweeps:

```sql
DELETE FROM sudo_elevations WHERE expires_at <= ?  -- now()
RETURNING id, user_id, layout_id, granted_at, expires_at;
```

For every deleted row it broadcasts
`auth.elevationChanged` over the WS hub to every live session of the
row's `UserID`. The signalman path has no janitor — its rows are
permanent.

The 10 s tick is intentionally coarse: the indicator countdown in the
UI is driven by the **expected** `expiresAt` timestamp from the last
`auth.elevationChanged` (or `/api/v1/auth/me` on reconnect, §7a.6),
so even when the janitor lags by a few seconds the UI flips back to
"closed" exactly on time. The server-side authority check
(`AuthService.Effective` → `EffectiveRoles.Has`, §7a.2) re-evaluates
membership on every request and never trusts the cached UI state, so
the small race window between the row's `ExpiresAt` and the
janitor's DELETE is harmless.

#### 7a.7.5 Resetting the layout admin PIN

The PIN is **resettable from the layout settings page only**, never
from the sudo dialog itself. The contract is the one already pinned
down in §3a.3:

- `PUT /api/v1/layouts/{id}` with body `{ name?, adminPin? }` is the
  single endpoint that rotates the PIN. Both fields are independent;
  a request with one field doesn't touch the other.
- **An empty or missing `adminPin` field is a no-op for the PIN.**
  This matches the user-visible contract from goal 20: "*not entering
  anything causes the PIN to remain unchanged*". The frontend's
  layout-settings form models this as a single text field with an
  explicit "Save" button – submitting with the field blank changes
  only the rest of the form (e.g. the layout name) and the page
  doesn't even fire the PIN-rotation request when the field is
  empty.
- **A non-empty `adminPin` is argon2id-hashed in
  `LayoutService.UpdateAdminPIN`** with a per-row salt before being
  written. The plaintext is overwritten in memory after hashing.
  PIN rotation writes the `layout.admin_pin_changed` audit row.
- The endpoint requires `eff.Has(domain.RoleAdmin)`. **Sudo admins
  pass the same gate as permanent admins** — there is no
  "non-sudo-only" carve-out. The whole point of the 2-minute admin
  elevation is to be a fully equivalent admin promotion; the Linux
  `sudo` analogy holds end-to-end. (The asymmetry that earlier
  drafts of this section described — a sudo admin being unable to
  rotate the PIN that bootstrapped their own elevation — was
  removed once we standardised on "sudo == admin everywhere"; the
  rate-limiter and the 2-minute timer remain the load-bearing
  defenses against PIN harvesting.)
- The system layout is allowed to rotate its PIN exactly the same
  way: it has no rename / no lock / no station-set edits, but its
  PIN must remain rotatable so the bootstrap one-shot PIN (printed
  to the server log on first boot) can be replaced at first login.

#### 7a.7.6 Where sudo lives in the policy layer

`AuthService.Effective(ctx, user, layoutID)` returns a flat
`domain.EffectiveRoles` (§7a.2). Permanent role, layout signalman
grant and the sudo admin elevation collapse onto the same set: the
policy layer asks `eff.Has(domain.RoleAdmin)` (or
`eff.Has(domain.RoleSignalman)`, etc.) and never branches on the
*source* of the membership. Concretely:

| Operation kind                                                                   | Policy gate                       |
|----------------------------------------------------------------------------------|-----------------------------------|
| Any admin action (rename layout, lock/unlock, attach/detach stations, manage signalmen, manage interlocking whitelist, **rotate the admin PIN**, delete the layout, manage users, view audit log) | `eff.Has(domain.RoleAdmin)`       |
| Operational signalman work (occupy interlocking, request takeover, add interlocking to whitelist) | `eff.Has(domain.RoleSignalman)`   |
| Driving authority                                                                | unchanged: §7a.3 `LocoSecurityContext.CanDriveLoco` does not look at sudo at all (the `admin` role does not grant the right to drive in the first place) |

The single cross-cutting rule is: a sudo admin grants the same
authority as a permanent admin **everywhere**. The 2-minute window
is the *only* guard rail.

#### 7a.7.7 Configuration surface

A single configuration block in the server config drives the whole
flow:

```yaml
# server.yaml (excerpt)
auth:
  sudo:
    ttl:               2m   # default; bounds [1m, 10m] enforced at startup
    fail_window:       1m   # rolling window for the failure counter
    max_failures:      5    # consecutive misses before soft lock
    lock_duration:     1m   # how long the (userId, layoutId) tuple stays locked
    janitor_interval:  10s  # how often the reap loop runs
    pin_min_length:    4    # validated by LayoutService.UpdateAdminPIN
    pin_max_length:    8
```

`pin_min_length` / `pin_max_length` ALSO gate the *initial* PIN set
on `POST /api/v1/layouts` and the rotation on
`PUT /api/v1/layouts/{id}`. A PIN that fails the bounds is rejected
with `layout_admin_pin_invalid`, which the frontend's layout-settings
form pre-validates so the user gets an inline error before the
request leaves the browser.

#### 7a.7.8 i18n: the new `sudo.json` namespace

Following the i18n contract (§7c.4), every user-visible string this
flow introduces lands in a new `sudo.json` namespace, mirrored across
`pl/` and `en/`. The padlock keys keep the countdown placeholder
(`{{remaining}}`); the engineer's-cap keys do not, because the
signalman membership is permanent — there is no timer to render.

| Key                                         | pl (canonical)                                                                  | en                                                              |
|---------------------------------------------|---------------------------------------------------------------------------------|-----------------------------------------------------------------|
| `tooltip.admin.idle`                        | „Aktywuj uprawnienia administratora makiety (sudo)"                              | "Elevate to layout administrator (sudo)"                        |
| `tooltip.admin.active`                      | „Aktywne sudo: administrator — pozostało {{remaining}}"                          | "Sudo active: administrator — {{remaining}} remaining"          |
| `tooltip.signalman.idle`                    | „Awansuj się na nastawniczego w tej makiecie"                                    | "Promote yourself to signalman in this layout"                  |
| `tooltip.signalman.active`                  | „Jesteś nastawniczym w tej makiecie — kliknij, aby zrezygnować"                   | "You are a signalman in this layout — click to step down"       |
| `aria.admin.idle` / `.active`               | „Aktywuj sudo administratora" / „Wyłącz sudo administratora"                     | "Activate admin sudo" / "Revoke admin sudo"                     |
| `aria.signalman.idle` / `.active`           | „Zostań nastawniczym" / „Zrezygnuj z roli nastawniczego"                          | "Become signalman" / "Step down from signalman"                  |
| `dialog.title.admin` / `.signalman`         | „PIN administratora makiety"                                                    | "Layout admin PIN"                                              |
| `dialog.description.admin`                  | „Wpisz PIN administracyjny makiety, aby uzyskać uprawnienia administratora na 2 minuty." | "Enter the layout admin PIN to gain administrator powers for 2 minutes." |
| `dialog.description.signalman`              | „Wpisz PIN administracyjny makiety, aby otrzymać rolę nastawniczego na stałe w tej makiecie." | "Enter the layout admin PIN to permanently take the signalman role in this layout." |
| `dialog.pinLabel` / `.submit` / `.cancel`   | „PIN" / „Aktywuj" / „Anuluj"                                                    | "PIN" / "Elevate" / "Cancel"                                    |
| `settings.pinLabel` / `.pinHelp`            | „PIN administratora makiety" / „Pozostaw puste, aby zachować obecny PIN. Wymagane 4–8 cyfr." | "Layout admin PIN" / "Leave blank to keep the current PIN. 4–8 digits required." |

New error codes added to `errors.json` in the same PR:

| Code                          | pl                                              | en                                                 |
|-------------------------------|-------------------------------------------------|----------------------------------------------------|
| `sudo_invalid_pin`            | „Nieprawidłowy PIN administracyjny makiety."     | "Wrong layout admin PIN."                          |
| `sudo_layout_mismatch`        | „Sesja jest powiązana z inną makietą."          | "Your session is bound to a different layout."     |
| `sudo_locked`                 | „Zbyt wiele nieudanych prób — spróbuj ponownie za chwilę." | "Too many failed attempts — try again in a moment." |
| `layout_admin_pin_invalid`    | „PIN administracyjny musi zawierać 4–8 cyfr."   | "The admin PIN must contain 4–8 digits."           |
| `layout_admin_pin_unset`      | „Makieta nie ma ustawionego PIN-u administracyjnego — poproś administratora o jego ustawienie." | "This layout has no admin PIN — ask the administrator to set one first." |

Note that `sudo.json` is added to the namespace list in the i18n
bootstrap (`web/src/i18n/index.ts`) and to the namespaces enumerated
in §7c.4; it is the only frontend wiring that lands outside the
`AppShell` / settings-page changes.
