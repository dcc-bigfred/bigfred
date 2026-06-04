### 3a.6 Vehicle functions and template inheritance (copy-on-write)

Vehicle functions (goal 16) and templates (goal 17) share one persistence
model: a single `dcc_functions` table and one domain type `DccFunction`.
Template-owned rows carry `template_id`; vehicle-owned rows carry
`vehicle_id`. Exactly one of those foreign keys is set per row.

#### 3a.6.0 Unified table `dcc_functions`

```sql
CREATE TABLE dcc_functions (
    id           INTEGER PRIMARY KEY,
    vehicle_id   INTEGER REFERENCES vehicles(id) ON DELETE CASCADE,
    template_id  INTEGER REFERENCES vehicle_templates(id) ON DELETE CASCADE,
    num          INTEGER NOT NULL CHECK (num BETWEEN 0 AND 31),
    name         TEXT NOT NULL,
    icon         TEXT NOT NULL,
    position     INTEGER NOT NULL,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    -- exactly one owner: template XOR vehicle
    CHECK (
        (vehicle_id IS NOT NULL AND template_id IS NULL)
        OR (vehicle_id IS NULL AND template_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX dcc_functions_vehicle_num
    ON dcc_functions (vehicle_id, num)
    WHERE vehicle_id IS NOT NULL;

CREATE UNIQUE INDEX dcc_functions_template_num
    ON dcc_functions (template_id, num)
    WHERE template_id IS NOT NULL;
```

REL migration mirrors the above in `repo/migrations/…`. Repository helpers:

- `ListFunctionsByTemplateID(ctx, templateID)` → `WHERE template_id = ?`
- `ListFunctionsByVehicleID(ctx, vehicleID)` → `WHERE vehicle_id = ?`
- `InsertFunction`, `UpdateFunction`, `DeleteFunction` — always set exactly
  one of `vehicle_id` / `template_id` on insert.

There are **no** separate `vehicle_functions` / `template_functions`
tables.

#### 3a.6.1 Resolution at read time

```go
// pkgs/server/service/function.go
type ResolvedFunction struct {
    Num      uint8
    Name     string
    Icon     domain.FunctionIcon
    Position int
    Source   string // "template" | "vehicle"
}

// List returns the effective function list a driver sees on the
// throttle. Pure read; no mutation, no detach.
func (s *FunctionService) List(ctx context.Context, vehicleID uint) ([]ResolvedFunction, error) {
    v, err := s.repo.LoadVehicle(ctx, vehicleID)
    if err != nil { return nil, err }

    // Linked: read template rows from dcc_functions (template_id = T).
    if v.TemplateID != nil && v.FunctionsDetachedAt == nil {
        fns, err := s.repo.ListFunctionsByTemplateID(ctx, *v.TemplateID)
        if err != nil { return nil, err }
        return toResolved(fns, "template"), nil
    }
    // Stand-alone or detached: read vehicle rows (vehicle_id = v.ID).
    fns, err := s.repo.ListFunctionsByVehicleID(ctx, v.ID)
    if err != nil { return nil, err }
    return toResolved(fns, "vehicle"), nil
}
```

#### 3a.6.2 Detach as the first step of every mutation

`EnsureDetached` is the bottleneck through which **every** mutating
function call on a **vehicle** passes (add/update/remove/reorder). It is
**idempotent** – a no-op for stand-alone or already-detached vehicles.
Materialisation happens in **one REL transaction** with the requested
mutation.

```go
// EnsureDetached copies template rows into new vehicle rows in
// dcc_functions when the vehicle is still linked. Template rows are
// left untouched. Subsequent calls are no-ops.
func (s *FunctionService) EnsureDetached(ctx context.Context, v *domain.Vehicle) error {
    if v.TemplateID == nil || v.FunctionsDetachedAt != nil {
        return nil
    }
    return s.repo.Transaction(ctx, func(ctx context.Context) error {
        tplFns, err := s.repo.ListFunctionsByTemplateID(ctx, *v.TemplateID)
        if err != nil { return err }
        now := time.Now().UTC()
        for _, tf := range tplFns {
            vid := v.ID
            err := s.repo.Insert(ctx, &domain.DccFunction{
                VehicleID:  &vid,
                TemplateID: nil,
                Num:        tf.Num,
                Name:       tf.Name,
                Icon:       tf.Icon,
                Kind:       tf.Kind,
                Position:   tf.Position,
                CreatedAt:  now,
                UpdatedAt:  now,
            })
            if err != nil { return err }
        }
        v.FunctionsDetachedAt = &now
        return s.repo.Update(ctx, v)
    })
}

// Template mutations operate on rows WHERE template_id = T directly.
// Vehicle mutations call EnsureDetached first, then upsert rows WHERE
// vehicle_id = v.ID.
func (s *FunctionService) UpsertVehicle(ctx context.Context, actor domain.User, v *domain.Vehicle, f domain.DccFunction) error {
    if d := s.sec.CanEditFunctions(actor, *v); !d.Allowed {
        return ErrForbidden(d.Reason)
    }
    return s.repo.Transaction(ctx, func(ctx context.Context) error {
        if err := s.EnsureDetached(ctx, v); err != nil { return err }
        vid := v.ID
        f.VehicleID, f.TemplateID = &vid, nil
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
                   │  dcc_functions.vehicle_id│
                   └──────────┬───────────────┘
                              │  (attach with template T)
                              ▼
                  ┌────────────────────────────────────┐
                  │           LINKED                    │
                  │  TemplateID = T                     │
                  │  FunctionsDetachedAt = nil          │
                  │                                     │
                  │  no rows with vehicle_id = v        │
                  │  list read from template_id = T     │
                  └─────────┬──────────────────────────┘
                            │  first edit on vehicle functions
                            │  (or explicit POST /functions/detach)
                            │
                            ▼   in ONE transaction:
              ┌──────────────────────────────────────────┐
              │  INSERT dcc_functions copies             │
              │    (vehicle_id=v, template_id=NULL)      │
              │    from rows (template_id=T)             │
              │  set v.FunctionsDetachedAt = now()       │
              └──────────────────────────────────────────┘
                            │
                            ▼
                  ┌────────────────────────────────────┐
                  │           DETACHED                  │
                  │  TemplateID = T (lineage kept)      │
                  │  FunctionsDetachedAt = ts           │
                  │                                     │
                  │  rows with vehicle_id = v           │
                  └─────────┬──────────────────────────┘
                            │  POST /functions/attach
                            ▼
                  DELETE rows WHERE vehicle_id = v;
                  FunctionsDetachedAt = nil → LINKED
```

#### 3a.6.4 Template deletion

Deleting a template returns `409 Conflict` if any vehicle is currently
linked **or** detached-with-this-lineage and the request did not pass
`?cascade=true`. With cascade:

1. For every linked vehicle the template's function list is materialized
   (`EnsureDetached`) – preserving every driver's current configuration.
2. For every vehicle (linked or detached) the `TemplateID` is set to
   `nil` so the lineage row does not dangle.
3. The template row is deleted; `ON DELETE CASCADE` removes all
   `dcc_functions` rows with `template_id` equal to that template.

The entire cascade runs inside a single transaction; partial deletion
is impossible.

#### 3a.6.5 Display order (`position`)

Each `DccFunction` row carries a dense integer `position` (0..n-1).
Clients MUST sort by `position` ascending when rendering both the
function editor list (§6.3e) and the throttle `<FunctionButtons>` row.
`POST …/functions/reorder` is the only way to change order; there is no
separate throttle layout.

#### 3a.6.6 Live propagation on the wire

When a function definition changes (vehicle-level OR template-level
that affects linked vehicles), the server emits a WebSocket event so
every open throttle re-renders without polling:

- `vehicle.functionsChanged` `{ addr }` – sent to every subscriber of
  that vehicle (driving, lessee, signalman). The UI re-fetches
  `GET /api/v1/vehicles/{addr}/functions`.

Template edits fan out: `TemplateService` mutation on rows with
`template_id = T` collects linked vehicles and emits
`vehicle.functionsChanged` for every *linked* vehicle. Detached vehicles
are unaffected by definition.
