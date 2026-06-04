# BigFred – Web Application Architecture Plan

This document describes the proposed architecture for a web application that
controls model railroad locomotives. It builds on top of the existing
`pkgs/loco` and `pkgs/rb` packages (Go core), which already provide a clean
`LocoApp` controller layer, a `Station` interface (Z21, LocoNet) and SQLite
access via `modernc.org/sqlite` (pure Go, `CGO_ENABLED=0`).

The architecture is split across multiple files under
[`./architecture/`](./architecture/). For an orientation read start at
the [architecture index](./architecture/README.md); for a specific
topic, jump directly to one of the sections below.

Section numbering used in the prose (`§3a.4`, `§4.5`, `§7b.1`, …) is
preserved verbatim in the headings of the split files, so existing
cross-references inside the text still work via `Ctrl+F`.

On the Go server, responsibilities are split across three packages under
`pkgs/server/`: **`http`** (and **`ws`**) terminate transport and
authentication; **`service`** owns validation, orchestration, and
permission checks via **`security`**; see
[§3.1 Backend layer responsibilities](./architecture/04-repository-layout.md#31-backend-layer-responsibilities).

## Table of contents

1. [Terminology](./architecture/00-terminology.md) — authoritative
   vocabulary (driver, signalman, vehicle, train, interlocking,
   takeover, lease, radio, layout, command station, function, vehicle template,
   script, throttle mode, dashboard, layout vehicle roster).
2. [Goals](./architecture/01-goals.md) — platform goals and the 19
   functional goals that drive everything else.
3. [Technology Stack](./architecture/02-tech-stack.md) — Go (chi, REL,
   Goja, mark3labs/mcp-go, …) and React (Vite, MUI, TanStack Query,
   Zustand, Monaco, …).
4. [High-Level Architecture](./architecture/03-high-level-architecture.md)
   — the diagram with both `server` and `scripts-executor` processes.
5. [Repository Layout](./architecture/04-repository-layout.md) — every
   directory under `pkgs/server/`, `pkgs/scripts-executor/` and `web/`;
   **§3.1** documents how `http`, `service`, and `security` divide work.
6. [Domain Model (REL — Data Mapper)](./architecture/05-domain-model/README.md)
   — entities, invariants, addressing rules, audit log, functions /
   templates / scripts, [function icon catalogue](./architecture/05-domain-model/08-function-icon-catalogue.md).
7. [Communication Protocol (REST + WebSocket)](./architecture/06-communication-protocol/README.md)
   — REST endpoints, WS actions / events, takeover state machine,
   radio delivery, drive session + dead-man's switch.
8. [Backend Components](./architecture/07-backend-components.md) — Hub,
   Client, dispatch, LocoService, poller, Redis roles, chi wiring.
9. [Frontend Components](./architecture/08-frontend-components.md) —
   `useSocket`, Zustand store, `LocoControl`, `TrainControl`, throttle
   mode overlay, vehicle catalogue & function editor (§6.3e), layout
   dashboard, interlocking view, MUI setup, script buttons & console.
10. [Cross-Cutting Concerns](./architecture/09-cross-cutting.md) —
    single binary, WS backpressure, reconnect, audit discipline,
    `scripts-executor` supervision.
11. [Internationalization (i18n)](./architecture/09a-i18n.md) —
    `react-i18next` with bundled JSON namespaces; backend stays
    language-neutral; what is translated vs. rendered verbatim;
    locale selection and persistence; type-safe message keys.
12. [Authentication, Roles & Authorization](./architecture/10-authn-authz/README.md)
    — login + PIN, effective roles, the `pkgs/server/security`
    policy layer, middleware, permission matrix.
13. [API Keys & Built-in MCP Server](./architecture/11-api-keys-and-mcp.md)
    — per-user keys (≤365 days), MCP tool surface mounted next to chi.
14. [Makefile Additions](./architecture/12-makefile.md) — build targets
    for `server` and `scripts-executor`.
15. [Delivery Order (Milestones)](./architecture/13-delivery-order.md)
    — M1..M8, incrementally shippable.
16. [Acceptance Criteria](./architecture/14-acceptance-criteria/README.md)
    — externally observable behaviours each milestone must demonstrate.
17. [Process Supervisor (Supervisord)](./architecture/15-supervisord/README.md)
    — non-root supervisord integration: process groups, Go templates,
    config regeneration, hot reload (`reread`+`update`), health observation.
18. [DCC Bus Daemon (`dcc-bus`)](./architecture/16-dcc-bus/README.md)
    — per-`(layout × command station)` sibling daemon owning the DCC
    bus and the throttle WebSocket; session-aware authorization via
    `pkgs/server/security`; Redis state cache; supervised by §7d.
