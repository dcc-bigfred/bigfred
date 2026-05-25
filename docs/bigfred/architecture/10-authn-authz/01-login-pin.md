### 7a.1 Login + PIN + layout picker

- A user logs in with three inputs presented together on the login
  screen: their **`login`**, a numeric **PIN** of configurable length
  (default 6 digits), and a **layout selector** (`makieta`). The
  selector is a dropdown right next to the login and PIN fields, so
  the user picks the layout they want to enter **at the moment of
  authentication**, never after. There is no post-login layout-list
  screen.
- The dropdown is populated by the **unauthenticated**
  `GET /api/v1/layouts/login` endpoint, called by the frontend before
  the user submits the form. It returns only layouts that are
  **currently selectable**: every row with `Locked = false`. The
  system-provided **system layout** (`IsSystem = true`, stored
  `Name = "default"`, cannot be locked) is always in the response and
  is rendered in the UI as **"Domyślna (warsztat)"** in Polish /
  **"Default (workshop)"** in English via the i18n key
  `layout:system_default_label`. It is also the dropdown's default
  pre-selected entry on first paint, so a user who never touches the
  selector simply lands in the system layout.
- PINs are hashed with **argon2id** with per-record salt and stored
  only as `pin_hash` in the `users` table. Plaintext PINs never leave
  `AuthService.Login`.
- Failed attempts are rate-limited **per `login`** *and* **per IP** in
  Redis (`auth:fail:<login>` and `auth:fail:<ip>`), with exponential
  back-off (1 s, 2 s, 4 s, … up to 60 s). After N consecutive failures
  the account is temporarily soft-locked.
- `POST /api/v1/auth/login { login, pin, layoutId }` runs in this
  order:
  1. lookup user by `login`, verify `pin_hash`; on mismatch return
     `401 invalid_credentials` and bump the rate-limit counters;
  2. lookup `Layout` by `layoutId`; if not found return
     `422 layout_not_found`;
  3. if `Layout.Locked == true` reject with `422 layout_locked` — this
     branch is defensive (the dropdown never offered the row in the
     first place, but a hand-crafted request could still hit the
     endpoint);
  4. on success, issue a signed session token (JWT, 24 h TTL) that
     carries `{ userId, layoutId }` and deliver it as an `HttpOnly`,
     `Secure`, `SameSite=Strict` cookie for REST; the same token is
     accepted as `?token=` for the WS upgrade.
- The `layoutId` baked into the JWT is **immutable for the lifetime of
  the token**: the WS upgrade reads it directly from the token, writes
  it once to `DriveSession.LayoutID`, and the user cannot change layout
  without logging out and logging back in. The frontend exposes a "log
  out and switch layout" affordance in `AppShell.tsx` next to the
  account menu so this is a one-click flow.
- API keys (§7b) inherit the same layout binding: an API key is minted
  while a user is logged into layout L, so the key permanently
  authenticates `{ userId, layoutId: L }`. Revoking and re-minting is
  the only way to point an API key at a different layout.
