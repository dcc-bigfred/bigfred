### 7a.1 Login + PIN

- A user logs in with their **`login`** and a numeric **PIN** of
  configurable length (default 6 digits). PINs are deliberately short so
  they are usable on a phone during a layout session.
- PINs are hashed with **argon2id** with per-record salt and stored only
  as `pin_hash` in the `users` table. Plaintext PINs never leave
  `AuthService.Login`.
- Failed attempts are rate-limited **per `login`** *and* **per IP** in
  Redis (`auth:fail:<login>` and `auth:fail:<ip>`), with exponential
  back-off (1 s, 2 s, 4 s, … up to 60 s). After N consecutive failures
  the account is temporarily soft-locked.
- On success, the server issues a signed session token (JWT, 24 h TTL)
  delivered as an `HttpOnly`, `Secure`, `SameSite=Strict` cookie for
  REST and accepted as `?token=` for the WS upgrade.
