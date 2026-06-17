---
name: review-docs-first
description: >-
  Orients on BigFred project documentation under docs/ before implementing or
  answering architecture questions in this repository. Use at the start of
  every new conversation in this repo, when the user asks about domain behavior,
  protocols, auth, DCC bus, milestones, or project conventions.
---

# Review project documentation first

## When to apply

At the **start of every new conversation** in this repository — before writing code, proposing designs, or answering questions about how the system should behave.

Re-read targeted sections when the task touches terminology, REST/WebSocket protocols, auth, DCC bus, supervisord, delivery milestones, or acceptance criteria.

## Workflow

1. **Discover** — List `docs/content/` and open the entry points below.
2. **Orient** — Read `docs/content/bigfred/README.md` and skim `docs/content/bigfred/architecture/README.md` for the table of contents.
3. **Focus** — Read section READMEs and files that match the user's first message (do not read all 60+ files unless the task is broad).
4. **Apply** — Use terminology, invariants, and protocols from the docs; treat them as authoritative for intended behavior.
5. **Reconcile** — When something does not line up, stop and ask the user how to proceed (do not guess):
   - **User request vs documentation** — Briefly cite the relevant doc section, explain the conflict, and ask what to do (e.g. follow the docs, follow the request, update the docs, or a scoped exception).
   - **Documentation vs code** — Note both sides and ask whether to match existing code, implement per the docs, or update one of them.

## Entry points

| Purpose | Path |
|--------|------|
| Project overview & full TOC | `docs/content/bigfred/README.md` |
| Architecture index | `docs/content/bigfred/architecture/README.md` |
| Terminology (required vocabulary) | `docs/content/bigfred/architecture/00-terminology.md` |
| Repository layout | `docs/content/bigfred/architecture/04-repository-layout.md` |
| REST + WebSocket | `docs/content/bigfred/architecture/06-communication-protocol/README.md` |
| Backend / frontend components | `docs/content/bigfred/architecture/07-backend-components.md`, `08-frontend-components.md` |
| Auth & permissions | `docs/content/bigfred/architecture/10-authn-authz/README.md` |
| Milestones & acceptance criteria | `docs/content/bigfred/architecture/13-delivery-order.md`, `14-acceptance-criteria/README.md` |
| DCC bus daemon | `docs/content/bigfred/architecture/16-dcc-bus/README.md` |
| Supervisord | `docs/content/bigfred/architecture/15-supervisord/README.md` |
| Published site | https://dcc-bigfred.github.io/docs/ |

Nested topics live in subfolders under `docs/content/bigfred/architecture/`; each subfolder has its own `README.md` with a local table of contents.

## Rules

- Do not skip the docs review at conversation start, even for small or seemingly local tasks.
- Prefer project terms from `00-terminology.md` in explanations and code comments.
- Cite specific doc paths when a decision is driven by documentation.
- If the user's request conflicts with the documentation, **do not proceed on assumption** — ask what they want before implementing.
