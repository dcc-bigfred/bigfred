### 7a.7 Sudo elevation – temporary `admin` / `signalman` via the layout PIN

This section turns goal 20 ("Sudo elevation – temporary
`admin`/`signalman` powers gated by a layout-scoped PIN") and the
`SudoElevation` / `Layout.AdminPINHash` invariants of §3a.1 / §3a.3
into a concrete, end-to-end flow. The mental model is borrowed
directly from `sudo` on Linux: an authenticated user types a PIN, gets
elevated rights for a short, fixed window, and falls back to their
permanent role when the timer runs out.

The flow has **two icons** on the top `AppBar` (rendered in
`AppShell.tsx`, §6.3b), both gated by the **single** **layout admin
PIN** (§0 Terminology) of the user's active layout:

- a **closed-padlock icon** – self-grants temporary `admin` rights;
- an **engineer's-cap icon** (the signalman / *nastawniczy* icon
  reused from the takeover UI) – self-grants the layout-scoped
  `signalman` role.

Clicking either opens the same modal `<SudoPinDialog>` asking for the
layout admin PIN. On success the icon flips to its **open** variant
with a live MM:SS countdown until the grant expires (default 2 min,
server-configurable in `[1m, 10m]`). When the grant expires the icon
reverts to its closed variant and the user loses the elevated
authority across **every** open session of theirs simultaneously.

#### 7a.7.1 Lifecycle of one elevation

```
  ┌─────────────────────────────────────────────────────────────────┐
  │  user clicks 🔒 / 🧑‍✈️  on the AppBar                           │
  │       │                                                           │
  │       ▼                                                           │
  │  POST /api/v1/layouts/{id}/sudo { target, pin }                  │
  │       │                                                           │
  │       │  PIN ok? ──┐                                              │
  │       │            ▼                                              │
  │       │       upsert SudoElevation { user, layout, target,        │
  │       │                              expiresAt = now + cfg.TTL }  │
  │       │            │                                              │
  │       │            ▼                                              │
  │       │       audit  auth.sudo_granted                            │
  │       │            │                                              │
  │       │            ▼                                              │
  │       │       fan-out auth.elevationChanged { granted:true,       │
  │       │                                       expiresAt }         │
  │       │            │                                              │
  │       │            ▼                                              │
  │       │       UI flips icon to OPEN + countdown                   │
  │       │                                                           │
  │       │  PIN wrong? ─┐                                            │
  │       │              ▼                                            │
  │       │         bump auth:sudo_fail:<userId>:<layoutId>           │
  │       │              auth:sudo_fail:<ip>                          │
  │       │              │                                            │
  │       │              ▼                                            │
  │       │         429 sudo_locked after N attempts                  │
  │       │              + audit auth.sudo_locked                     │
  │       │                                                           │
  │       ▼                                                           │
  │  expiry path:                                                     │
  │    janitor goroutine (every 30 s) finds rows with                 │
  │    ExpiresAt <= now()                                             │
  │       │                                                           │
  │       ▼                                                           │
  │    DELETE the row                                                 │
  │       │                                                           │
  │       ▼                                                           │
  │    audit auth.sudo_expired (actor = system user)                  │
  │       │                                                           │
  │       ▼                                                           │
  │    fan-out auth.elevationChanged { granted:false,                 │
  │                                    reason:"expired" }             │
  │       │                                                           │
  │       ▼                                                           │
  │    UI flips icon back to CLOSED                                   │
  └─────────────────────────────────────────────────────────────────┘
```

The same teardown path also runs on:

- **explicit user revoke** – the user clicks the open padlock or
  engineer-cap a second time (or a "Revoke now" menu item beside it).
  Fires `DELETE /api/v1/layouts/{id}/sudo { target? }`. Idempotent;
  `auth.sudo_revoked { reason:"user_action" }`.
- **logout** – `AuthService.Logout` cascades through every active
  `SudoElevation` row of the calling user, regardless of layout or
  target, in the same transaction as the JWT-blacklist insert. Audit
  rows are written with `reason:"logout"`; `auth.elevationChanged`
  is broadcast to every other live session of the user before the
  current session disconnects.
- **layout deletion** – `LayoutService.Delete` cascades to the
  `SudoElevation` rows pointing at the layout. Audit
  `reason:"layout_deleted"`. In practice deletion is rejected with
  `409 layout_in_use` whenever any session is still pinned to the
  layout, so this branch only fires for layouts no user is currently
  in.

A second click on an already-elevated icon while the row is still
live is treated as a **renewal**, not a duplicate insert: the row's
`ExpiresAt` is bumped to `now() + cfg.SudoTTL` and a fresh
`auth.sudo_granted` audit row is written (`Metadata.reason:"renewed"`
distinguishes it from a first-time grant). The
`auth.elevationChanged` event carries `reason:"renewed"`. This
matches Linux `sudo` semantics, where re-typing the PIN inside the
grace window resets the timer.

#### 7a.7.2 The `AuthService` surface

```go
// pkgs/server/service/auth.go
package service

import (
    "context"
    "time"

    "github.com/keskad/loco/pkgs/server/domain"
)

// SudoTTL is the configured wall-clock window of a sudo elevation.
// Loaded from server config at startup; bounded to [1m, 10m] – a
// value outside that range is a fatal startup error. The default is
// 2 minutes.
type SudoConfig struct {
    TTL          time.Duration // default 2*time.Minute
    FailAttempts int           // default 5; consecutive misses before soft-lock
    LockDuration time.Duration // default 5*time.Minute
}

// Sudo verifies the PIN against Layout.AdminPINHash and, on success,
// upserts a SudoElevation row for (caller, layout, target). On
// mismatch it bumps the per-(userId, layoutId) and per-IP Redis
// counters with exponential back-off (1s, 2s, 4s, …, 60s) and
// returns a typed error the HTTP layer maps to `401 invalid_pin`.
// After cfg.FailAttempts consecutive failures the (userId, layoutId)
// tuple is soft-locked for cfg.LockDuration; the next call returns
// `429 sudo_locked` until the lock window passes.
//
// Sudo is **always a self-grant**: there is no admin-side `Grant
// sudo to user X` path. The actor and the elevated user are the same
// `domain.User`.
func (s *AuthService) Sudo(
    ctx context.Context,
    userID, layoutID uint,
    target domain.SudoTarget,
    pin string,
) (domain.SudoElevation, error)

// RevokeSudo deletes the SudoElevation row(s) for (userID,
// layoutID). When `target` is non-nil only that target is removed;
// otherwise both the admin and the signalman row (if present) are
// removed. Idempotent. Writes the matching `auth.sudo_revoked`
// audit rows and broadcasts `auth.elevationChanged { granted:false,
// reason:"user_action" }` to every live WS session of the user.
func (s *AuthService) RevokeSudo(
    ctx context.Context,
    userID, layoutID uint,
    target *domain.SudoTarget,
) error

// Logout deletes the JWT (blacklist insert) AND every SudoElevation
// row for the caller (any layout, any target) in a single
// transaction. Audit rows are written with reason:"logout" and the
// elevation-cleared fan-out goes to every OTHER live session of the
// user before the current session disconnects.
func (s *AuthService) Logout(ctx context.Context, userID uint) error
```

The PIN itself never leaves `Sudo`: the function argon2id-verifies
`pin` against `Layout.AdminPINHash` (the same column rotated by
`LayoutService.UpdateAdminPIN`, see §7a.7.4), and the plaintext
string is overwritten in memory before the function returns.

#### 7a.7.3 Janitor goroutine

Sudo expiry shares the periodic janitor goroutine introduced in §7
(cross-cutting concern 9 "Time-based grants cleanup"). Once every
30 s the goroutine runs, in addition to its existing lease /
takeover sweeps:

```sql
DELETE FROM sudo_elevations WHERE expires_at <= ?  -- now()
RETURNING id, user_id, layout_id, target, granted_at, expires_at;
```

For every deleted row it:

1. writes `auth.sudo_expired` to the audit log (actor = system user
   id `0`, login `"system"`; `Metadata = { target, grantedAt }`);
2. fan-outs `auth.elevationChanged { target, granted:false,
   reason:"expired" }` over the WS hub to every live session of the
   row's `UserID`, regardless of which layout that session is in –
   the AppBar icons are layout-scoped at render time so the frontend
   simply ignores the event when the user is currently in a different
   layout.

The 30 s tick is intentionally coarse: the indicator countdown in the
UI is driven by the **expected** `expiresAt` timestamp from the last
`auth.elevationChanged` (or `/api/v1/auth/me` on reconnect, §7a.6),
so even when the janitor lags by a few seconds the UI flips back to
"closed" exactly on time. The server-side authority check
(`EffectiveRoles.IsSudoOnly` in the policy layer, §7a.3) re-evaluates
membership on every request and never trusts the cached UI state, so
the small race window between the row's `ExpiresAt` and the janitor's
DELETE is harmless: the policy layer treats a row with
`ExpiresAt <= now()` as if it had already been deleted (the
`AuthService.Effective` query carries an `AndGt("expires_at", now)`
filter, see §7a.2).

#### 7a.7.4 Resetting the layout admin PIN

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
  written. The plaintext is overwritten in memory after hashing. The
  audit row `layout.admin_pin_changed` carries
  `Metadata = { previousHashPrefix }` (first 8 chars of the previous
  hash for forensic correlation – never the plaintext, never the
  full hash).
- The endpoint is gated by `LayoutSecurityContext.CanRotateAdminPIN`
  (§7a.3). The rule is **non-sudo `admin` only** – a sudo-elevated
  admin is rejected with `requires_non_sudo_admin` even though they
  pass `eff.Has(domain.RoleAdmin)`. This is the load-bearing
  asymmetry that prevents the entire sudo concept from
  self-destructing: if a sudo user could rotate the PIN that gated
  their own elevation, they would silently lock the real admin out
  during the 2-minute window.
- The system layout is allowed to rotate its PIN exactly the same
  way: it has no rename / no lock / no station-set edits, but its
  PIN must remain rotatable so the bootstrap one-shot PIN (printed
  to the server log on first boot) can be replaced at first login.

```go
// pkgs/server/service/layout.go (excerpt)
type LayoutUpdateInput struct {
    Name     *string // nil = no change
    AdminPIN *string // nil OR empty string = no change
}

// UpdateAdminPIN is split out from Update for two reasons:
//   1. it has its own audit row (layout.admin_pin_changed) which
//      carries the previous-hash-prefix metadata;
//   2. the policy gate is CanRotateAdminPIN, not CanEditLayout, so
//      the rule layer can refuse a sudo admin without leaking the
//      "no-op when blank" semantic into the security policy.
//
// Returns rel.ErrNotFound when the layout vanished mid-request,
// security.ErrDenied{Reason:"requires_non_sudo_admin"} when the
// caller's admin role is sudo-only, and ErrPinTooWeak when the
// plaintext fails the configured length/format checks.
func (s *LayoutService) UpdateAdminPIN(
    ctx context.Context,
    actor domain.EffectiveRoles, layoutID uint, plaintextPIN string,
) error
```

The HTTP handler behind `PUT /api/v1/layouts/{id}` decomposes the
request body, calls `LayoutService.Update` for the name (when
present) and `LayoutService.UpdateAdminPIN` for the PIN (when present
**and non-empty**), and returns the merged result. The PIN-rotation
audit row is written only when the second branch fires, so a name-only
edit produces a single `layout.updated` row instead of a confusing
pair.

#### 7a.7.5 Frontend: AppBar icons, dialog, countdown

The two AppBar icons live in `AppShell.tsx` next to the existing
account / locale / Throttle controls (§6.3b). Both are rendered for
every authenticated user; the closed-padlock visual is the default
state, the open-padlock visual + a small `MM:SS` countdown badge is
the elevated state.

```tsx
// web/src/components/AppShell.tsx (excerpt)
import LockIcon         from "@mui/icons-material/Lock";
import LockOpenIcon     from "@mui/icons-material/LockOpen";
import EngineeringIcon  from "@mui/icons-material/Engineering";
import EngineeringOutlinedIcon from "@mui/icons-material/EngineeringOutlined";

import { useElevation } from "../hooks/useElevation";
import { SudoPinDialog } from "./SudoPinDialog";

function SudoIndicator({ target }: { target: "admin"|"signalman" }) {
  const { active, expiresAt, request, revoke } = useElevation(target);
  const remaining = useCountdown(expiresAt); // returns "01:23" or null
  const [dialogOpen, setDialogOpen] = useState(false);
  const { t } = useTranslation("sudo");

  const ClosedIcon = target === "admin" ? LockIcon            : EngineeringOutlinedIcon;
  const OpenIcon   = target === "admin" ? LockOpenIcon        : EngineeringIcon;

  return (
    <>
      <Tooltip title={active
          ? t(`tooltip.${target}.active`, { remaining })
          : t(`tooltip.${target}.idle`)}>
        <IconButton
          color={active ? "warning" : "inherit"}
          aria-pressed={active}
          aria-label={t(`aria.${target}.${active ? "active" : "idle"}`)}
          onClick={() => active ? revoke() : setDialogOpen(true)}
        >
          {active ? <OpenIcon /> : <ClosedIcon />}
          {active && <Badge badgeContent={remaining} color="warning" />}
        </IconButton>
      </Tooltip>
      <SudoPinDialog
        open={dialogOpen}
        target={target}
        onCancel={() => setDialogOpen(false)}
        onSubmit={async (pin) => {
          await request(pin);          // POST .../sudo
          setDialogOpen(false);
        }}
      />
    </>
  );
}
```

`useElevation(target)` is a small Zustand-backed hook that:

- seeds its initial state from `useMe()` (`/api/v1/auth/me` returns
  the active sudo rows; §6 REST table);
- subscribes to the WS `auth.elevationChanged` events for the
  matching `target` and rewrites the cached `expiresAt`;
- exposes `request(pin)` which `POST`s to
  `/api/v1/layouts/{layoutId}/sudo` and `revoke()` which `DELETE`s.

Because `auth.elevationChanged` is broadcast to **every** live WS
session of the user (§4.2), starting sudo on the desktop instantly
flips the indicator on the phone, and the auto-expiry fan-out reaches
both UIs in the same code path. There is no per-tab state to
reconcile.

`<SudoPinDialog>` reuses the same numeric-only input affordances as
the login PIN (auto-focus, on-screen number pad on touch devices,
masked digits, "show" eye toggle). On `429 sudo_locked` it disables
the OK button and renders a localized "Locked until HH:MM:SS" line
sourced from the `Retry-After` header. The dialog never logs the
plaintext PIN to the console.

#### 7a.7.6 Where sudo lives in the policy layer

The structural distinction between a permanent `admin` role and a
sudo-elevated `admin` role lives entirely in `domain.EffectiveRoles`
(§7a.2) and the `LayoutSecurityContext` family of methods (§7a.3).
The HTTP and WS layers never inspect `SudoElevation` rows directly –
they hand `EffectiveRoles` to the relevant policy method and translate
`Decision.Reason` into a status code. The single rule-of-thumb is:

| Operation kind                                | Policy gate                  |
|-----------------------------------------------|------------------------------|
| Operational admin work (DCC pool override, register a guest loco, grant a temporary role, read the audit log) | `eff.Has(domain.RoleAdmin)` – sudo admins pass |
| Organisational admin work (rename layout, lock/unlock, attach/detach stations, manage signalmen, manage interlocking whitelist, **rotate the admin PIN**, delete the layout) | `eff.HasNonSudo(domain.RoleAdmin)` – sudo-only admins are denied with `requires_non_sudo_admin` |
| Cross-layout admin work (manage users, DCC pools globally, view audit log) | unchanged from existing rules; sudo admins pass |
| Operational signalman work (occupy interlocking, request takeover, add interlocking to whitelist) | `eff.Has(domain.RoleSignalman)` – sudo signalmen pass |
| Driving authority | unchanged: §7a.3 `LocoSecurityContext.CanDriveLoco` does not look at sudo at all (the `admin` role does not grant the right to drive in the first place) |

The asymmetry is intentional and is the single most important
property of this section: a sudo user can do **operational** admin
work in a club room ("oh, this guest brought a loco, register it
outside my pool real quick") but cannot make **organisational**
changes to the layout itself ("rename it, lock me out, switch the
command station set"). The 2-minute window matches that scope: long
enough to register a vehicle and grant a one-shot temporary role,
short enough that an unattended browser tab cannot be hijacked into
a permanent admin session.

#### 7a.7.7 Configuration surface

A single configuration block in the server config drives the whole
flow:

```yaml
# server.yaml (excerpt)
auth:
  sudo:
    ttl:               2m   # default; bounds [1m, 10m] enforced at startup
    fail_attempts:     5    # consecutive misses before soft lock
    lock_duration:     5m   # how long the (userId, layoutId) tuple stays locked
    pin_min_length:    6    # validated by LayoutService.UpdateAdminPIN
    pin_max_length:    12
```

`pin_min_length` / `pin_max_length` ALSO gate the *initial* PIN set
on `POST /api/v1/layouts` and the rotation on
`PUT /api/v1/layouts/{id}`. A PIN that fails the bounds is rejected
with `pin_too_weak` (numeric-only, length in `[min, max]`), which the
frontend's layout-settings form pre-validates so the user gets an
inline error before the request leaves the browser.

#### 7a.7.8 i18n: the new `sudo.json` namespace

Following the i18n contract (§7c.4), every user-visible string this
flow introduces lands in a new `sudo.json` namespace, mirrored across
`pl/` and `en/`:

| Key                                         | pl (canonical)                                                                 | en                                                              |
|---------------------------------------------|--------------------------------------------------------------------------------|-----------------------------------------------------------------|
| `dialog.title`                              | „Awans tymczasowy"                                                             | "Temporary elevation"                                           |
| `dialog.description.admin`                  | „Wpisz PIN administracyjny makiety, aby zyskać uprawnienia administratora na {{minutes}} min." | "Enter the layout admin PIN to gain administrator powers for {{minutes}} min." |
| `dialog.description.signalman`              | „Wpisz PIN administracyjny makiety, aby zostać nastawniczym tej makiety na {{minutes}} min." | "Enter the layout admin PIN to become a signalman of this layout for {{minutes}} min." |
| `dialog.pin.label`                          | „PIN administracyjny makiety"                                                  | "Layout admin PIN"                                              |
| `dialog.submit`                             | „Zatwierdź"                                                                    | "Elevate"                                                       |
| `dialog.cancel`                             | „Anuluj"                                                                       | "Cancel"                                                        |
| `tooltip.admin.idle`                        | „Awansuj do administratora makiety"                                            | "Elevate to layout admin"                                       |
| `tooltip.admin.active`                      | „Administrator makiety – pozostało {{remaining}}. Kliknij, aby zwolnić."        | "Layout admin – {{remaining}} left. Click to revoke."           |
| `tooltip.signalman.idle`                    | „Awansuj do nastawniczego makiety"                                             | "Elevate to layout signalman"                                   |
| `tooltip.signalman.active`                  | „Nastawniczy makiety – pozostało {{remaining}}. Kliknij, aby zwolnić."          | "Layout signalman – {{remaining}} left. Click to revoke."       |
| `aria.admin.idle` / `.active`               | „Kłódka uprawnień administratora (zamknięta / otwarta, {{remaining}})"          | "Admin lock (closed / open, {{remaining}})"                     |
| `aria.signalman.idle` / `.active`           | „Czapka nastawniczego (nieaktywna / aktywna, {{remaining}})"                    | "Signalman cap (idle / active, {{remaining}})"                  |
| `toast.granted.admin` / `.signalman`        | „Otrzymano uprawnienia administratora / nastawniczego ({{minutes}} min)"        | "Admin / signalman granted ({{minutes}} min)"                   |
| `toast.expired`                             | „Uprawnienia tymczasowe wygasły"                                               | "Temporary elevation expired"                                   |
| `toast.locked`                              | „Zbyt wiele błędnych prób. Spróbuj ponownie za {{minutes}} min."                | "Too many failed attempts. Try again in {{minutes}} min."       |
| `settings.pin.title`                        | „PIN administracyjny makiety"                                                  | "Layout admin PIN"                                              |
| `settings.pin.helper`                       | „Pozostawienie pustego pola NIE zmienia PIN-u."                                | "Leaving this field blank does NOT change the PIN."             |
| `settings.pin.changed`                      | „PIN administracyjny makiety został zmieniony."                                | "Layout admin PIN updated."                                     |

New error codes added to `errors.json` in the same PR:

| Code                          | pl                                              | en                                                 |
|-------------------------------|-------------------------------------------------|----------------------------------------------------|
| `invalid_pin`                 | „Nieprawidłowy PIN administracyjny makiety."     | "Incorrect layout admin PIN."                      |
| `sudo_locked`                 | „Zbyt wiele błędnych prób PIN-u. Spróbuj później." | "Too many failed PIN attempts. Try again later." |
| `pin_missing`                 | „Wpisz PIN administracyjny makiety."            | "Please enter the layout admin PIN."               |
| `pin_too_weak`                | „PIN musi zawierać {{min}}–{{max}} cyfr."       | "PIN must be {{min}}–{{max}} digits."              |
| `requires_non_sudo_admin`     | „Operacja wymaga stałych uprawnień administratora (nie tymczasowych)." | "This action requires permanent admin rights (not a sudo elevation)." |
| `layout_mismatch`             | „Żądanie dotyczy innej makiety niż Twoja sesja." | "The request targets a different layout than your session." |

The `audit.json` namespace gains the four new actions
(`auth.sudo_granted`, `auth.sudo_revoked`, `auth.sudo_expired`,
`auth.sudo_locked`, `layout.admin_pin_changed`) keyed under
`audit:action.<action_with_dots_to_underscores>` per the §7c.2
mapping rule.

Note that `sudo.json` is added to the namespace list in the i18n
bootstrap (`web/src/i18n/index.ts`) and to the namespaces enumerated
in §7c.4; it is the only frontend wiring that lands outside the
`AppShell` / settings-page changes.
