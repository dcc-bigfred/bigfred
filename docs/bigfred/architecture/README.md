# BigFred – Architecture (split documents)

This folder holds the BigFred architecture proposal, split into one file per
top-level section. The parent file [`../architecture.md`](../architecture.md)
is the thin entry point with the full table of contents.

Section numbering (`§3a.4`, `§7b.1`, …) is preserved in heading text so
cross-references inside the prose still work via `Ctrl+F`. Large sections
that grew beyond ~250 lines are further split into subfolders; each subfolder
carries its own `README.md` with its sub-TOC.

## Sections

1. [Terminology](./00-terminology.md)
2. [Goals](./01-goals.md)
3. [Technology Stack](./02-tech-stack.md)
4. [High-Level Architecture](./03-high-level-architecture.md)
5. [Repository Layout](./04-repository-layout.md)
6. [Domain Model (REL — Data Mapper)](./05-domain-model/README.md)
7. [Communication Protocol (REST + WebSocket)](./06-communication-protocol/README.md)
8. [Backend Components](./07-backend-components.md)
9. [Frontend Components](./08-frontend-components.md)
10. [Cross-Cutting Concerns](./09-cross-cutting.md)
11. [Internationalization (i18n)](./09a-i18n.md)
12. [Authentication, Roles & Authorization](./10-authn-authz/README.md)
13. [API Keys & Built-in MCP Server](./11-api-keys-and-mcp.md)
14. [Makefile Additions](./12-makefile.md)
15. [Delivery Order (Milestones)](./13-delivery-order.md)
16. [Acceptance Criteria](./14-acceptance-criteria/README.md)
17. [Process Supervisor (Supervisord)](./15-supervisord/README.md) — non-root
    process groups, Go templates, hot reload, daemon lifecycle
18. [DCC Bus Daemon (`dcc-bus`)](./16-dcc-bus/README.md) — per-
    `(layout × command station)` sibling daemon for the throttle
    data plane; session-aware, security-policy-driven, Redis-cached
