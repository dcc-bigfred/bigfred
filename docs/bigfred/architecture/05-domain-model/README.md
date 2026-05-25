## 3a. Domain Model (REL – Data Mapper)

The multi-user functional goals translate into the following entities.
REL maps these structs onto SQLite tables; the domain types live in
`pkgs/server/domain/` and contain **no persistence tags beyond the few
hints REL needs**, so the controller layer (`LocoApp` and the services)
stays free of any ORM imports.

## Subsections

1. [Entities](./01-entities.md) — `User`, `Vehicle`, `Train`, `Lease`, `Interlocking`, `CommandStation`, `Layout`, `Function`, `Template`, `Script`, `AuditLog`, …
2. [REL repository – Data Mapper in practice](./02-rel-repository.md)
3. [Invariants enforced by services + DB constraints](./03-invariants.md)
4. [Layout-and-command station addressing rules](./04-layout-command-station-addressing.md)
5. [Audit log](./05-audit-log.md)
6. [Vehicle functions and template inheritance (copy-on-write)](./06-functions-and-templates.md)
7. [Server-side scripts (Goja sandbox in a sibling executor process)](./07-scripts.md)
