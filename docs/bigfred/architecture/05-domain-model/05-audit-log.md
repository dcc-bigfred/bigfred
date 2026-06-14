### 3a.5 Audit log

The audit log is implemented as a single append-only table
`audit_log_entries` mapped onto `domain.AuditLogEntry`. The
`AuditService` is the only writer; every other service injects it and
calls `Log` **after** the underlying mutation has succeeded.

```go
// pkgs/bigfred/server/service/audit.go
type AuditService struct {
    repo rel.Repository
    bus  *bus.Bus // optional: stream to admin WS sessions
}

// Log appends a single entry. Denormalized fields (actor login,
// object name) MUST be passed in by the caller – the service does NOT
// look them up itself, so write paths stay one-DB-roundtrip cheap and
// idempotent.
func (s *AuditService) Log(ctx context.Context, e domain.AuditLogEntry) error {
    if e.OccurredAt.IsZero() {
        e.OccurredAt = time.Now().UTC()
    }
    if err := s.repo.Insert(ctx, &e); err != nil {
        return fmt.Errorf("audit write failed: %w", err)
    }
    if s.bus != nil {
        s.bus.Publish(bus.AuditAppended{Entry: e}) // for live admin feed
    }
    return nil
}

// Read paths are dedicated, filterable and admin-only.
type AuditQuery struct {
    Action      *domain.AuditAction
    ActorUserID *uint
    ObjectType  *string
    ObjectID    *uint
    LayoutID     *uint
    Since, Until *time.Time
    Limit, Offset int
}

func (s *AuditService) List(ctx context.Context, q AuditQuery) ([]domain.AuditLogEntry, error)
```

**Where each event is emitted (one call site per row):**

| Action                          | Emitted by                              |
|---------------------------------|------------------------------------------|
| `vehicle.created/updated/deleted` | `LocoService.Create/Update/Delete`     |
| `vehicle.leased`                 | `LeaseService.LeaseVehicle`            |
| `vehicle.lease_revoked`          | `LeaseService.RevokeVehicleLease`      |
| `vehicle.lease_expired`          | Janitor goroutine (§7 cross-cutting)   |
| `train.created/updated/deleted`  | `TrainService.Create/Update/Delete`    |
| `train.leased / lease_revoked / lease_expired` | `LeaseService` + janitor |
| `command_station.created/updated/deleted` | `CommandStationService.Create/Update/Delete`   |
| `layout.created/updated/deleted`  | `LayoutService.Create/Update/Delete`    |
| `layout.locked / layout.unlocked` | `LayoutService.Lock / Unlock`. `ObjectID = layout.id`, `ObjectName = layout.name`. The system layout cannot be locked, so neither row ever appears for it. |
| `layout.command_station_attached / layout.command_station_detached` | `LayoutService.AttachCommandStation / DetachCommandStation` (admin-only, non-system layouts only). `ObjectID = layout.id`, `ObjectName = layout.name`, `Metadata = { commandStationId, commandStationName }`. |
| `layout.admin_pin_changed`           | `LayoutService.UpdateAdminPIN` (any effective admin, sudo or permanent). `ObjectID = layout.id`, `ObjectName = layout.name`, `Metadata = { previousHashPrefix }` (first 8 chars of the previous argon2id hash for forensic correlation; never plaintext, never the full hash). The system layout uses the same audit row. |
| `auth.sudo_granted`                  | `SudoService.Sudo` after a successful PIN verification. `ActorUserID/ActorLogin = caller`. `ObjectType = "layout"`, `ObjectID = layoutID`, `ObjectName = layout.Name` at grant time. `LayoutID = layoutID`. `Metadata = { expiresAt }`. |
| `auth.sudo_revoked`                  | `SudoService.Revoke` (explicit user revoke), `AuthService.Logout` cleanup, or `LayoutService.Delete` cascade. `Metadata = { reason: "user_action"\|"logout"\|"layout_deleted" }`. |
| `auth.sudo_expired`                  | Janitor goroutine when `SudoElevation.ExpiresAt <= now()`. The actor is the **system user** (id `0`, login `"system"`). `Metadata = { grantedAt }`. |
| `auth.sudo_locked`                   | `SudoService.Sudo` / `GrantSignalman` after the rate-limit threshold has been crossed and the (userId, layoutId) tuple is soft-locked. `Metadata = { attempts, lockedUntil }`. |
| `layout_signalman.granted / revoked` | `SudoService.GrantSignalman / RevokeSignalman` (the engineer's-cap icon, §7a.7) AND `LayoutService.AddSignalman / RemoveSignalman` (the admin-side endpoint). `Metadata = { source: "self"\|"admin" }` distinguishes the two paths. |
| `vehicle.functions_updated`      | `FunctionService.Upsert / Remove / Reorder` (after `EnsureDetached`). `ObjectID = vehicle.id`, `Metadata = {num, name, icon, position, prev?}` |
| `vehicle.functions_detached`     | `FunctionService.EnsureDetached` when the copy fires. `Metadata = {templateId, copied_rows}` |
| `vehicle.functions_attached`     | `FunctionService.Attach` on explicit re-link. `Metadata = {templateId}` |
| `template.created/updated/deleted` | `TemplateService.Create/Update/Delete` |
| `script.created/updated/deleted` | `ScriptService.Create/Update/Delete`. `ObjectID = script.id`, `ObjectName = script.name`, `Metadata = {runtime, icon, sourceLen}` (source body is **never** stored in audit). |
| `script.attached / detached`     | `ScriptService.Attach / Detach`. `ObjectID = script.id`, `Metadata = {vehicleId? , trainId?}`. |
| `session.emergency_executed`     | `ws.Hub` after the dead-man's switch runs the user's `EmergencyPlan` (§4.5.5). `ObjectID = sessionID`, `ObjectName = sessionID prefix`, `Metadata = {action, affected_vehicles, terminated_scripts}` (number of Goja VMs `vm.Interrupt("deadman")`-ed in the sibling executor as part of the emergency). |
| `system.radio_stop`              | `ws.Hub` after a successful `system.radioStop` (§4.6). `ObjectType = "layout"`, `ObjectID = layoutID`, `ObjectName = layout.Name`, `Metadata = {triggered_by_user_id, affected_vehicles, terminated_scripts, command_stations: [{id, addrs[]}]}` – one entry per `dcc-bus` that acknowledged the halt. |

**Write-path discipline:**

- Writes are **best-effort but logged**: an audit-write error is logged
  with `logrus.WithError(...)` and otherwise swallowed, so a broken
  audit storage cannot block a driver's legitimate command. Sites that
  consider the audit critical (admin actions) may opt into "strict
  mode" via `AuditService.LogStrict` which returns the error to the
  caller.
- Where possible, the audit insert is wrapped in the same REL
  transaction as the domain change so both commit atomically (relevant
  for create/update/delete on vehicle/train/command station/layout).

**Read-path discipline:**

- Listing the audit log is gated by `AuditSecurityContext.CanReadAuditLog`,
  which permits `admin` only.
- All filter parameters in `AuditQuery` are server-validated; the API
  enforces a maximum window (default 90 days) and a maximum page size
  (default 200) to keep the listing cheap.
- There are **no `UPDATE` or `DELETE` endpoints** for audit entries.
  The table has only an `INSERT` path. SQLite indices on
  `(occurred_at)`, `(actor_user_id, occurred_at)` and
  `(object_type, object_id, occurred_at)` cover the common queries.
