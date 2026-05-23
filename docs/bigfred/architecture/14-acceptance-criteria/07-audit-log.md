### 10.5 Audit log (M3, extended in M4–M5)

- Every CRUD on a vehicle, train, layout or party produces **exactly
  one** audit entry, persisted **in the same transaction** as the
  underlying mutation. If the mutation rolls back, the audit row does
  not appear; if the audit insert fails, the mutation rolls back.
- Granting a vehicle lease writes a `vehicle.leased` entry with
  `ObjectId = vehicle.id`, `ObjectName = vehicle.name` at the time of
  the lease, and `Metadata` containing the lessee's id and login plus
  the lease `expires_at`.
- Revoking the same lease early writes `vehicle.lease_revoked`;
  letting it expire naturally writes `vehicle.lease_expired` (emitted
  by the janitor, not the user, so the actor is the **system user**
  with id `0` and login `"system"`).
- When the dead-man's switch fires for a user (§4.5), a
  `session.emergency_executed` entry is written with the affected
  vehicle DCC addresses in `Metadata`. An admin opening the activity
  screen sees this row labelled `"maszynista zasnął"` in the UI.
- Subsequently renaming or deleting the affected vehicle, the user or
  the layout/party **does not change** any historical audit row. The
  denormalized `ObjectName` / `ActorLogin` fields still reflect the
  state at the time of the event.
- A non-admin calling `GET /api/v1/audit-log` receives `403`. There
  are no `PUT`, `PATCH` or `DELETE` routes on `/api/v1/audit-log/...`;
  attempts return `405 Method Not Allowed`.
- Filtering the audit log by `?action=`, `?actor=`, `?objectType=` and
  a `since`/`until` window returns only matching rows; the default
  page size is 200 and the maximum is 1000.
