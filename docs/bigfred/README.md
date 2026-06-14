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
`pkgs/bigfred/server/`: **`http`** (and **`ws`**) terminate transport and
authentication; **`service`** owns validation, orchestration, and
permission checks via **`security`**; see
[§3.1 Backend layer responsibilities](./architecture/04-repository-layout.md#31-backend-layer-responsibilities).

## Table of contents

1. [Terminology](./architecture/00-terminology.md)
2. [Goals](./architecture/01-goals.md)
3. [Technology Stack](./architecture/02-tech-stack.md)
4. [High-Level Architecture](./architecture/03-high-level-architecture.md)
5. [Repository Layout](./architecture/04-repository-layout.md)
6. [Domain Model (REL — Data Mapper)](./architecture/05-domain-model/README.md)
7. [Communication Protocol (REST + WebSocket)](./architecture/06-communication-protocol/README.md)
8. [Backend Components](./architecture/07-backend-components.md)
9. [Frontend Components](./architecture/08-frontend-components.md)
10. [Cross-Cutting Concerns](./architecture/09-cross-cutting.md)
11. [Internationalization (i18n)](./architecture/09a-i18n.md)
12. [Authentication, Roles & Authorization](./architecture/10-authn-authz/README.md)
13. [API Keys & Built-in MCP Server](./architecture/11-api-keys-and-mcp.md)
14. [Makefile Additions](./architecture/12-makefile.md)
15. [Delivery Order (Milestones)](./architecture/13-delivery-order.md)
16. [Acceptance Criteria](./architecture/14-acceptance-criteria/README.md)
17. [Process Supervisor (Supervisord)](./architecture/15-supervisord/README.md)
18. [DCC Bus Daemon (`dcc-bus`)](./architecture/16-dcc-bus/README.md)
