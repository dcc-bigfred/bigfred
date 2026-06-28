---
name: coding-standards-first
description: >-
  Before writing or planning Go, React, JavaScript, or TypeScript code in this
  repository, load coding standards and Cursor skills from the docs repository
  (local sibling, submodule, or clone). Use when implementing, refactoring,
  reviewing, or designing backend or frontend code.
---

# Coding standards first

## When to apply

**Before writing or planning code** in Go **or** React / JavaScript / TypeScript
in this repository — including new features, refactors, API design, component
structure, and code review.

This skill complements [review-docs-first](../review-docs-first/SKILL.md):
that skill covers **domain** documentation (architecture, protocols, acceptance
criteria); this one covers **coding standards and implementation skills**.

## Resolve the docs repository

The `docs` repo holds coding standards and project skills. Find it on disk
before reading anything:

1. **Monorepo sibling** (default when working in `dcc-bigfred/`):
   `../docs/` relative to this `bigfred/` repo root.
2. **Nested checkout**: `docs/` inside the workspace if the user opened a
   parent folder that contains both repos.
3. **Separate clone**: any local path the user cloned
   [github.com/dcc-bigfred/docs](https://github.com/dcc-bigfred/docs) into —
   ask if neither path above exists.

Set `DOCS_ROOT` to the resolved directory. All paths below are relative to
`DOCS_ROOT`.

Verify: `DOCS_ROOT/coding-standards/README.md` and
`DOCS_ROOT/.cursor/skills/` must exist. If not, stop and ask the user for the
docs checkout path.

## Workflow

1. **Resolve** `DOCS_ROOT` (see above).
2. **Classify** the task — Go backend, React/frontend, or both.
3. **Read** the relevant standards and skills (sections below) *before*
   proposing a design or editing code.
4. **Apply** conventions silently during implementation; cite standard paths
   when a review decision depends on them.
5. **Reconcile** — if the request conflicts with a standard, cite the file and
   ask how to proceed (same rule as review-docs-first).

## Go — what to read

| Purpose | Path under `DOCS_ROOT` |
| --- | --- |
| Index | `coding-standards/golang/README.md` |
| Project layout | `coding-standards/golang/project-layout.md` |
| Idiomatic Go | `coding-standards/golang/idiomatic-go.md` |
| SOLID | `coding-standards/golang/solid.md` |
| Clean code | `coding-standards/golang/clean-code.md` |
| DI, IoC, KISS | `coding-standards/golang/design-principles.md` |
| Design patterns | `coding-standards/golang/patterns/README.md` |

For a **specific pattern** (factory, adapter, worker pool, …), open the
matching file under `coding-standards/golang/patterns/` instead of guessing.

**Scope for planning:** at minimum read the index plus the pages that match
the task (e.g. `solid.md` + `design-principles.md` before introducing an
interface; the relevant pattern file before applying a pattern).

## React / JavaScript / TypeScript — what to read

| Purpose | Path under `DOCS_ROOT` |
| --- | --- |
| Cursor skill (entry) | `.cursor/skills/react-best-practices/SKILL.md` |
| Full compiled rules | `.cursor/skills/react-best-practices/AGENTS.md` |
| Individual rules | `.cursor/skills/react-best-practices/rules/*.md` |

Source: [vercel-labs/agent-skills — react-best-practices](https://github.com/vercel-labs/agent-skills/tree/main/skills/react-best-practices).

**Scope for planning:** read `SKILL.md` first; open rule files (or
`AGENTS.md`) for the categories that match the task — waterfalls (`async-*`),
bundle size (`bundle-*`), re-renders (`rerender-*`), etc.

Frontend code in this repo lives mainly under `web/` (`bigfred/web/`).

## Other skills in the docs repo

List `DOCS_ROOT/.cursor/skills/` when the task touches documentation or
i18n. Known skills:

| Skill | When |
| --- | --- |
| `react-best-practices` | React / Next.js / frontend performance |
| `docs-i18n` | Editing MkDocs content under `content/en/` and `content/pl/` |

## Rules

- Do not skip standards review when the change looks small — layout, naming,
  and error handling are decided here.
- Prefer **docs repo paths** over memory or generic blog advice.
- For Go, `coding-standards/golang/` is authoritative over ad-hoc style.
- For React/JS/TS, `react-best-practices` is authoritative for performance
  and component patterns unless the codebase already documents an exception.
- If `coding-standards/typescript/` is added later, treat it the same way as
  the Go tree.

## Checklist (copy for planning)

```
- [ ] DOCS_ROOT resolved
- [ ] Go task → coding-standards/golang/ relevant pages read
- [ ] Frontend task → react-best-practices SKILL.md (+ rule files) read
- [ ] Domain behaviour → review-docs-first entry points read
- [ ] Design aligns with standards before first line of code
```
