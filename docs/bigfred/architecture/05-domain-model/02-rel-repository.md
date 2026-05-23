### 3a.2 REL repository – Data Mapper in practice

REL is a **Data Mapper**: the entity structs above know nothing about
SQL; persistence goes through `rel.Repository`, which is mocked in
tests with the `reltest` package.

```go
// pkgs/server/repo/db.go
package repo

import (
    "context"
    "database/sql"

    "github.com/go-rel/rel"
    "github.com/go-rel/sqlite3"
    _ "modernc.org/sqlite"
)

func Open(ctx context.Context, dsn string) (rel.Repository, *sql.DB, error) {
    db, err := sql.Open("sqlite", dsn) // pure-Go driver, no CGO
    if err != nil {
        return nil, nil, err
    }
    adapter := sqlite3.New(db)
    repo := rel.New(adapter)
    return repo, db, nil
}
```

```go
// pkgs/server/repo/leases.go
package repo

import (
    "context"
    "time"

    "github.com/go-rel/rel"
    "github.com/go-rel/rel/where"

    "github.com/keskad/loco/pkgs/server/domain"
)

// ActiveVehicleLease returns the currently active lease for a vehicle
// (not revoked, not expired) – or rel.ErrNotFound.
func ActiveVehicleLease(ctx context.Context, repo rel.Repository, vehicleID uint) (domain.VehicleLease, error) {
    var lease domain.VehicleLease
    err := repo.Find(ctx, &lease,
        where.Eq("vehicle_id", vehicleID).
            AndNil("revoked_at").
            AndGt("expires_at", time.Now()),
    )
    return lease, err
}

// LeaseVehicle creates a lease atomically with a guard that no other
// active lease exists – everything in one transaction.
func LeaseVehicle(ctx context.Context, repo rel.Repository, l domain.VehicleLease) error {
    return repo.Transaction(ctx, func(ctx context.Context) error {
        if _, err := ActiveVehicleLease(ctx, repo, l.VehicleID); err == nil {
            return ErrAlreadyLeased
        } else if err != rel.ErrNotFound {
            return err
        }
        return repo.Insert(ctx, &l)
    })
}
```

```go
// pkgs/server/repo/migrations/001_init.go – REL migrations live in Go
package migrations

import "github.com/go-rel/rel"

func MigrateInit(schema *rel.Schema) {
    schema.CreateTable("users", func(t *rel.Table) {
        t.ID("id")
        t.String("login", rel.Unique(true))
        t.String("pin_hash")
        t.String("role")
        t.DateTime("created_at")
        t.DateTime("updated_at")
    })
    schema.CreateTable("vehicles", func(t *rel.Table) {
        t.ID("id")
        t.Int("dcc_address", rel.Unique(true))
        t.Int("owner_user_id")
        t.String("name")
        t.String("type")
        t.DateTime("created_at")
        t.DateTime("updated_at")
        t.ForeignKey("owner_user_id", "users", "id")
    })
    // ... trains, leases, interlockings, interlocking_sessions,
    // takeover_requests, radio_messages, temporary_roles, dcc_address_ranges
}
```
