### 3a.3 Invariants enforced by services + DB constraints

| Invariant                                                          | Enforced by                                                       |
|--------------------------------------------------------------------|-------------------------------------------------------------------|
| `vehicle.dcc_address` is globally unique                            | DB `UNIQUE` constraint                                            |
| A user can only register vehicles in their DCC pool                | `UserService.RegisterVehicle` checks `DCCAddressRange`            |
| At most one active lease per vehicle/train                         | `LeaseVehicle` transaction + partial unique index on active rows  |
| Lessee cannot edit, only drive                                     | Authorization middleware (§11)                                    |
| Exactly one signalman per interlocking                             | Partial unique index `UNIQUE(interlocking_id) WHERE ended_at IS NULL` |
| Temporary role expires automatically                               | Filtered out in `AuthService.Roles()`; janitor goroutine reaps    |
| Takeover auto-grant after 15 s with no rejection                   | `TakeoverService` timer + `AutoGrantAt` column                    |
| API key lifetime ≤ 365 days                                         | `APIKeyService.Create` validates `expires_at - now() ≤ 365d`     |
| API key plaintext never stored                                      | Only `KeyHash` and `KeyPrefix` are persisted                     |
| API key inherits owner's effective roles & DCC pool                 | `APIKeyService.Verify` returns the same `auth.Identity` as login |
| Every non-default party has exactly one layout                      | DB CHECK: `(name = 'default') OR (layout_id IS NOT NULL)` + FK to `layouts(id)`; service validates on create/update |
| The `default` party always exists                                   | Bootstrap migration seeds one row with `name='default'` and `layout_id=NULL`; admins cannot delete it |
| Only the `default` party may have `LayoutID = NULL`                 | Same DB CHECK as above; `PartyService.Create` rejects non-default parties without `layoutId` |
| Joining a NON-default party whose layout is unreachable fails fast  | `PartyService.Join` initialises the `Station` for that layout and rejects on connection error before opening a drive session |
| In the `default` party the driver picks the layout per session      | `PartyService.Join` opens the drive session with `session.LayoutID = nil`; throttle commands return `layout_not_selected` until `session.setLayout` is sent |
| Signalman role is party-scoped                                      | `AuthService.Effective(user, partyID)` unions `PartySignalman` rows for that party only |
| Interlocking visibility filtered by party                           | `InterlockingService.List(partyID)` returns only `PartyInterlocking` whitelisted rows |
| Layout edits are admin-only                                         | `LayoutSecurityContext.CanEditLayout` (§7a.3) returns `Deny` for non-admin |
| Vehicle function number ∈ [0, 32]                                   | DB `CHECK (num BETWEEN 0 AND 32)` + service-side validation       |
| `(VehicleID, Num)` / `(TemplateID, Num)` are unique                 | DB `UNIQUE` indexes; service catches collisions before insert     |
| Only the vehicle's owner may edit its function definitions          | `FunctionSecurityContext.CanEditFunctions` (lessees and signalmen are denied even with active driving authority) |
| Editing a linked vehicle detaches it via copy-on-write              | `FunctionService.EnsureDetached` runs as the first step of every mutation, in the same transaction |
| Deleting a referenced template requires explicit cascade            | `TemplateService.Delete` returns `409` unless `cascade=true`; with cascade, every linked vehicle is detached first |
| A script attachment binds to a Vehicle XOR a Train                  | DB `CHECK ( (vehicle_id IS NULL) <> (train_id IS NULL) )` + service validation |
| Only the script's owner may edit it                                 | `ScriptSecurityContext.CanEditScript` (lessees seeing it via leased vehicle can only run) |
| A script can never escalate its user's driving authority            | Every Goja binding routes through `LocoService`/`TrainService` in `server`, which re-checks `LocoSecurityContext.CanDriveLoco` against the **current** session state on every call – the same code path a manual throttle takes |
| Script source size cap                                              | `ScriptService.Save` rejects sources larger than 64 KiB with `422` |
| The same script appears at most once per throttle                   | `UNIQUE(script_id, vehicle_id) WHERE vehicle_id IS NOT NULL` + analogous index on `train_id` |
| One running goroutine per VM                                        | The executor creates exactly one `*goja.Runtime` per active run and owns it from one goroutine; never shared (Goja VMs are not goroutine-safe per its FAQ) |
| Every run has a hard wall-clock deadline                            | The executor arms `time.AfterFunc(Script.DeadlineSec, vm.Interrupt)` at run start; default 60 s, hard cap 600 s validated by `ScriptService.Save` |
| At most one active run per `ScriptAttachment` per user              | `ScriptService.Run` rejects a second press on the same attachment from the same user with `ack { ok:false, error:"already_running" }`; different attachments and different users run independently |
| Executor process crash never affects the DCC bus                    | The command station lives in `server` only; on executor crash `server` fan-outs `script.runStopped { reason:"executor_crashed" }` to every affected session and respawns the executor (§7.x) |
| Script source is never sent to the executor over the wire... that user does not own | The RPC connection is local-only (Unix socket, `0600`) – source travels server→executor only for the runs server itself authored on the user's behalf |
| A train slider moves every member in lock-step                      | `TrainService.SetSpeed` fans `Station.SetSpeed(addr, speed, forward XOR member.Reversed)` over **every** `TrainMember` row, never a subset |
| `train.setSpeed` is best-effort, not atomic                         | DCC bus has no multi-address atomic write; the WS `ack` returns per-member `{ok, error?}` so the UI can surface a partial failure rather than silently desync the consist |
