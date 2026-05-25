## 7a. Authentication, Roles & Authorization

This section turns the functional goals about users, roles, leases and
interlockings into concrete code-level rules.

## Subsections

1. [Login + PIN](./01-login-pin.md)
2. [Effective roles](./02-effective-roles.md) — `effective(user, layout)` computation
3. [Domain Policy Layer (`pkgs/server/security`)](./03-security-policy-layer.md) — `*SecurityContext` per aggregate
4. [Middleware – using the policies](./04-middleware.md)
5. [Permission matrix](./05-permission-matrix.md)
6. [Session reconciliation on WS connect](./06-session-reconciliation.md)
7. [Sudo elevation – temporary `admin` / `signalman` via the layout PIN](./07-sudo-elevation.md)
