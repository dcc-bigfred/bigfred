### 3a.6 Vehicle functions and template inheritance (copy-on-write)

Vehicle functions (goal 15) and templates (goal 16) form a small,
self-contained sub-system. The interesting bit is the **copy-on-write
detachment**: while a vehicle is linked, its function list is purely
virtual; the first edit materializes the rows and severs the link.

#### 3a.6.1 Resolution at read time

```go
// pkgs/server/service/function.go
type ResolvedFunction struct {
    Num      uint8
    Name     string
    Icon     domain.FunctionIcon
    Kind     domain.FunctionKind
    Position int
    Source   string // "template" | "vehicle"
}

// List returns the effective function list a driver sees on the
// throttle. Pure read; no mutation, no detach.
func (s *FunctionService) List(ctx context.Context, vehicleID uint) ([]ResolvedFunction, error) {
    v, err := s.repo.LoadVehicle(ctx, vehicleID)
    if err != nil { return nil, err }

    // Linked: read template rows live.
    if v.TemplateID != nil && v.FunctionsDetachedAt == nil {
        tfs, err := s.repo.ListTemplateFunctions(ctx, *v.TemplateID)
        if err != nil { return nil, err }
        return toResolved(tfs, "template"), nil
    }
    // Stand-alone or detached: read the vehicle's own rows.
    vfs, err := s.repo.ListVehicleFunctions(ctx, v.ID)
    if err != nil { return nil, err }
    return toResolved(vfs, "vehicle"), nil
}
```

#### 3a.6.2 Detach as the first step of every mutation

`EnsureDetached` is the bottleneck through which **every** mutating
function call passes (add/update/remove/reorder). It is **idempotent**
– a no-op for stand-alone or already-detached vehicles. The whole
materialisation happens in **one REL transaction** with the requested
mutation, so a vehicle is never observed half-detached.

```go
// EnsureDetached idempotently materializes the template's current
// function list into vehicle_functions rows when the vehicle is still
// linked. Subsequent calls are no-ops.
func (s *FunctionService) EnsureDetached(ctx context.Context, v *domain.Vehicle) error {
    if v.TemplateID == nil || v.FunctionsDetachedAt != nil {
        return nil
    }
    return s.repo.Transaction(ctx, func(ctx context.Context) error {
        tfs, err := s.repo.ListTemplateFunctions(ctx, *v.TemplateID)
        if err != nil { return err }
        now := time.Now().UTC()
        for _, tf := range tfs {
            err := s.repo.Insert(ctx, &domain.VehicleFunction{
                VehicleID: v.ID,
                Num:       tf.Num,
                Name:      tf.Name,
                Icon:      tf.Icon,
                Kind:      tf.Kind,
                Position:  tf.Position,
                CreatedAt: now,
                UpdatedAt: now,
            })
            if err != nil { return err }
        }
        v.FunctionsDetachedAt = &now
        return s.repo.Update(ctx, v)
    })
}

// Every mutating method starts with EnsureDetached, then operates on
// the vehicle_functions rows.
func (s *FunctionService) Upsert(ctx context.Context, actor domain.User, v *domain.Vehicle, f domain.VehicleFunction) error {
    if d := s.sec.CanEditFunctions(actor, *v); !d.Allowed {
        return ErrForbidden(d.Reason)
    }
    return s.repo.Transaction(ctx, func(ctx context.Context) error {
        if err := s.EnsureDetached(ctx, v); err != nil { return err }
        // ... upsert by (vehicle_id, num) ...
        return s.audit.Log(ctx, makeAuditEntry(actor, v, f, "vehicle.functions_updated"))
    })
}
```

#### 3a.6.3 State diagram

```
                   ┌──────────────────────────┐
                   │     stand-alone vehicle  │
                   │  (TemplateID == nil)     │
                   └──────────┬───────────────┘
                              │  (attach with template T)
                              ▼
                  ┌────────────────────────────────────┐
                  │           LINKED                    │
                  │  TemplateID = T                     │
                  │  FunctionsDetachedAt = nil          │
                  │                                     │
                  │  function list is VIRTUAL,          │
                  │  read live from TemplateFunction    │
                  └─────────┬──────────────────────────┘
                            │  first edit on this vehicle's functions
                            │  (or explicit POST /functions/detach)
                            │
                            ▼   in ONE transaction:
              ┌──────────────────────────────────────────┐
              │  copy TemplateFunction(T) → VehicleFunction(v)│
              │  set v.FunctionsDetachedAt = now()           │
              └──────────────────────────────────────────┘
                            │
                            ▼
                  ┌────────────────────────────────────┐
                  │           DETACHED                  │
                  │  TemplateID = T (lineage kept)      │
                  │  FunctionsDetachedAt = ts           │
                  │                                     │
                  │  function list is OWNED,            │
                  │  read from VehicleFunction          │
                  └─────────┬──────────────────────────┘
                            │  POST /functions/attach (manual re-sync)
                            ▼
                  delete VehicleFunction rows; set
                  FunctionsDetachedAt = nil → back to LINKED
```

#### 3a.6.4 Template deletion

Deleting a template returns `409 Conflict` if any vehicle is currently
linked **or** detached-with-this-lineage and the request did not pass
`?cascade=true`. With cascade:

1. For every linked vehicle the template's function list is materialized
   (`EnsureDetached`) – preserving every driver's current configuration.
2. For every vehicle (linked or detached) the `TemplateID` is set to
   `nil` so the lineage row does not dangle.
3. The template and its `TemplateFunction` rows are deleted.

The entire cascade runs inside a single transaction; partial deletion
is impossible.

#### 3a.6.5 Live propagation on the wire

When a function definition changes (vehicle-level OR template-level
that affects linked vehicles), the server emits a WebSocket event so
every open throttle re-renders without polling:

- `vehicle.functionsChanged` `{ addr }` – sent to every subscriber of
  that vehicle (driving, lessee, signalman). The UI re-fetches
  `GET /api/v1/vehicles/{addr}/functions`.

Template edits fan out: `TemplateService.Update` collects the list of
linked vehicles **and** the list of detached-with-this-lineage vehicles
(for which only `Description` / `Name` of the template itself matters,
not the function list); it emits `vehicle.functionsChanged` for every
*linked* vehicle. Detached vehicles are unaffected by definition.
