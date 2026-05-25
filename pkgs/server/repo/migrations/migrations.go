// Package migrations centralises every schema change applied to the
// BigFred database. The bootstrap intentionally relies on REL's own
// migrator (github.com/go-rel/rel/migrator) so that future milestones
// can simply Register() additional versions next to the ones declared
// here without changing the call site in `main`.
package migrations

import (
	"context"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/migrator"
)

// MigrateUp applies every Register-ed migration in ascending version
// order. The migrator skips versions already recorded in the
// `rel_schema_versions` table, so calling MigrateUp on every server
// startup is safe and cheap.
func MigrateUp(ctx context.Context, repo rel.Repository) {
	m := migrator.New(repo)
	register(&m)
	m.Migrate(ctx)
}

// register is the single place where new migrations are wired in.
// Versions MUST be monotonically increasing integers (epoch-like
// stamps are recommended so concurrent feature branches don't collide).
func register(m *migrator.Migrator) {
	m.Register(20260523_000001, createUsersUp, createUsersDown)
	m.Register(20260525_000001, createLayoutsUp, createLayoutsDown)
	m.Register(20260525_000002, createInterlockingsUp, createInterlockingsDown)
	m.Register(20260525_000003, createLayoutSignalmenUp, createLayoutSignalmenDown)
	m.Register(20260525_000004, createInterlockingSessionsUp, createInterlockingSessionsDown)
	m.Register(20260525_000005, createDCCAddressRangesUp, createDCCAddressRangesDown)
	m.Register(20260525_000006, createVehiclesUp, createVehiclesDown)
	m.Register(20260525_000007, createTrainsUp, createTrainsDown)
	m.Register(20260525_000008, createLayoutVehiclesUp, createLayoutVehiclesDown)
	m.Register(20260525_000009, addUsersActiveColumnUp, addUsersActiveColumnDown)
	m.Register(20260525_000010, addLayoutsAdminPINUp, addLayoutsAdminPINDown)
	m.Register(20260525_000011, createSudoElevationsUp, createSudoElevationsDown)
}

func createUsersUp(s *rel.Schema) {
	s.CreateTable("users", func(t *rel.Table) {
		t.ID("id")
		t.String("login")
		t.String("pin_hash")
		t.String("role")
		t.DateTime("created_at")
		t.DateTime("updated_at")

		t.Unique([]string{"login"})
	})
}

func createUsersDown(s *rel.Schema) {
	s.DropTable("users")
}

// createLayoutsUp wires the `layouts` table that backs domain.Layout
// (§3a.1). On top of the obvious columns it installs two SQLite
// constraints that the spec calls out explicitly (§9 step 11):
//
//  1. CHECK (NOT (is_system = 1 AND locked = 1)) — the system layout
//     can never be locked, so this catches both a buggy service call
//     and a hand-rolled UPDATE.
//
//  2. UNIQUE partial index on `is_system` WHERE is_system = 1 —
//     guarantees at most one system row exists. SQLite does not
//     support partial indexes through REL's typed Schema builder,
//     hence the raw s.Exec.
//
// `name` is a plain unique constraint; the user-facing label for the
// system row is rendered through the i18n key `layout:system_default_label`,
// so the stored Name ("default") is an opaque system marker.
func createLayoutsUp(s *rel.Schema) {
	s.CreateTable("layouts", func(t *rel.Table) {
		t.ID("id")
		t.String("name")
		t.Bool("is_system", rel.Default(false))
		t.Bool("locked", rel.Default(false))
		t.Int("created_by", rel.Unsigned(true), rel.Default(0))
		t.DateTime("created_at")
		t.DateTime("updated_at")

		t.Unique([]string{"name"})
		t.Fragment("CHECK (NOT (is_system = 1 AND locked = 1))")
	})
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX layouts_unique_system ON layouts(is_system) WHERE is_system = 1`))
}

func createLayoutsDown(s *rel.Schema) {
	s.DropTable("layouts")
}

func createInterlockingsUp(s *rel.Schema) {
	s.CreateTable("interlockings", func(t *rel.Table) {
		t.ID("id")
		t.String("name")
		t.Text("location")
		t.DateTime("created_at")

		t.Unique([]string{"name"})
	})

	s.CreateTable("layout_interlockings", func(t *rel.Table) {
		t.ID("id")
		t.Int("layout_id", rel.Unsigned(true))
		t.Int("interlocking_id", rel.Unsigned(true))
		t.Int("added_by_user_id", rel.Unsigned(true), rel.Default(0))
		t.DateTime("added_at")

		t.Unique([]string{"layout_id", "interlocking_id"})
	})
}

func createInterlockingsDown(s *rel.Schema) {
	s.DropTable("layout_interlockings")
	s.DropTable("interlockings")
}

func createLayoutSignalmenUp(s *rel.Schema) {
	s.CreateTable("layout_signalmen", func(t *rel.Table) {
		t.ID("id")
		t.Int("layout_id", rel.Unsigned(true))
		t.Int("user_id", rel.Unsigned(true))
		t.Int("granted_by", rel.Unsigned(true), rel.Default(0))
		t.DateTime("granted_at")
		t.DateTime("expires_at", rel.Required(false))

		t.Unique([]string{"layout_id", "user_id"})
	})
}

func createLayoutSignalmenDown(s *rel.Schema) {
	s.DropTable("layout_signalmen")
}

func createInterlockingSessionsUp(s *rel.Schema) {
	s.CreateTable("interlocking_sessions", func(t *rel.Table) {
		t.ID("id")
		t.Int("interlocking_id", rel.Unsigned(true))
		t.Int("signalman_user_id", rel.Unsigned(true))
		t.DateTime("started_at")
		t.DateTime("ended_at")
	})
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX interlocking_sessions_one_active
		ON interlocking_sessions(interlocking_id) WHERE ended_at IS NULL`))
}

func createInterlockingSessionsDown(s *rel.Schema) {
	s.DropTable("interlocking_sessions")
}

// createDCCAddressRangesUp installs the per-user DCC pool table
// (goal 3, §3a.1). Several rows per user are allowed so the admin can
// hand out non-contiguous windows ("100..199 + 3001..3010"). Bounds
// are inclusive; service-side code rejects from>to.
func createDCCAddressRangesUp(s *rel.Schema) {
	s.CreateTable("dcc_address_ranges", func(t *rel.Table) {
		t.ID("id")
		t.Int("user_id", rel.Unsigned(true))
		t.Int("from_addr", rel.Unsigned(true))
		t.Int("to_addr", rel.Unsigned(true))

		t.Fragment("CHECK (from_addr <= to_addr)")
	})
	s.Exec(rel.Raw(`CREATE INDEX dcc_address_ranges_user_id ON dcc_address_ranges(user_id)`))
}

func createDCCAddressRangesDown(s *rel.Schema) {
	s.DropTable("dcc_address_ranges")
}

// createVehiclesUp wires the vehicles table that backs domain.Vehicle
// (§3a.1). Key points:
//
//   - `dcc_address` is NULLABLE so dummy vehicles (vehicles without a
//     DCC decoder, used as visual fillers / unpowered wagons attached
//     to a train) coexist with the rest of the catalogue.
//
//   - The uniqueness constraint on `dcc_address` is a partial index
//     (`WHERE dcc_address IS NOT NULL`) so multiple dummies can sit
//     side-by-side in the catalogue without colliding on NULL.
//
//   - `kind` is a closed catalogue (loco | emu | driving_wagon |
//     trolley | wagon); the CHECK enforces the enum at the DB so an
//     out-of-band SQL UPDATE cannot wedge the application.
func createVehiclesUp(s *rel.Schema) {
	s.CreateTable("vehicles", func(t *rel.Table) {
		t.ID("id")
		t.Int("dcc_address", rel.Unsigned(true), rel.Required(false))
		t.Int("owner_user_id", rel.Unsigned(true))
		t.String("name")
		t.String("kind")
		t.String("number", rel.Default(""))
		t.DateTime("created_at")
		t.DateTime("updated_at")

		t.Fragment("CHECK (kind IN ('loco','emu','driving_wagon','trolley','wagon'))")
	})
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX vehicles_unique_dcc_address ON vehicles(dcc_address) WHERE dcc_address IS NOT NULL`))
	s.Exec(rel.Raw(`CREATE INDEX vehicles_owner_user_id ON vehicles(owner_user_id)`))
}

func createVehiclesDown(s *rel.Schema) {
	s.DropTable("vehicles")
}

// createTrainsUp installs the trains catalogue + the ordered
// `train_members` join. Position is the throttle-render ordering;
// Reversed flips the per-member DCC direction so a vehicle coupled
// the other way around rolls the right way under a unified train
// slider (§4.2 train.setSpeed).
func createTrainsUp(s *rel.Schema) {
	s.CreateTable("trains", func(t *rel.Table) {
		t.ID("id")
		t.Int("owner_user_id", rel.Unsigned(true))
		t.String("name")
		t.DateTime("created_at")
		t.DateTime("updated_at")

		t.Unique([]string{"owner_user_id", "name"})
	})
	s.Exec(rel.Raw(`CREATE INDEX trains_owner_user_id ON trains(owner_user_id)`))

	s.CreateTable("train_members", func(t *rel.Table) {
		t.ID("id")
		t.Int("train_id", rel.Unsigned(true))
		t.Int("vehicle_id", rel.Unsigned(true))
		t.Int("position")
		t.Bool("reversed", rel.Default(false))

		t.Unique([]string{"train_id", "vehicle_id"})
	})
	s.Exec(rel.Raw(`CREATE INDEX train_members_train_id ON train_members(train_id)`))
}

func createTrainsDown(s *rel.Schema) {
	s.DropTable("train_members")
	s.DropTable("trains")
}

// createLayoutVehiclesUp installs the layout vehicle / train roster
// (§3a.1, §6.3c). Unique indexes on (layout_id, vehicle_id) and
// (layout_id, train_id) guarantee a vehicle/train appears at most
// once on a given layout — matching the §3a.3 invariant.
func createLayoutVehiclesUp(s *rel.Schema) {
	s.CreateTable("layout_vehicles", func(t *rel.Table) {
		t.ID("id")
		t.Int("layout_id", rel.Unsigned(true))
		t.Int("vehicle_id", rel.Unsigned(true))
		t.Int("added_by_user_id", rel.Unsigned(true))
		t.DateTime("added_at")

		t.Unique([]string{"layout_id", "vehicle_id"})
	})
	s.Exec(rel.Raw(`CREATE INDEX layout_vehicles_layout_id ON layout_vehicles(layout_id)`))

	s.CreateTable("layout_trains", func(t *rel.Table) {
		t.ID("id")
		t.Int("layout_id", rel.Unsigned(true))
		t.Int("train_id", rel.Unsigned(true))
		t.Int("added_by_user_id", rel.Unsigned(true))
		t.DateTime("added_at")

		t.Unique([]string{"layout_id", "train_id"})
	})
	s.Exec(rel.Raw(`CREATE INDEX layout_trains_layout_id ON layout_trains(layout_id)`))
}

func createLayoutVehiclesDown(s *rel.Schema) {
	s.DropTable("layout_trains")
	s.DropTable("layout_vehicles")
}

// addUsersActiveColumnUp installs the `active` flag on the `users`
// table (§7a, user management). Existing rows default to active so
// the migration is non-destructive on installations that pre-date
// the user-management feature.
//
// SQLite does not support adding a NOT NULL column without a default
// to an existing table, so we explicitly pin `DEFAULT 1` — REL's
// typed AlterTable builder forwards this as the column default.
func addUsersActiveColumnUp(s *rel.Schema) {
	s.AlterTable("users", func(t *rel.AlterTable) {
		t.Bool("active", rel.Default(true))
	})
}

func addUsersActiveColumnDown(s *rel.Schema) {
	s.AlterTable("users", func(t *rel.AlterTable) {
		t.DropColumn("active")
	})
}

// addLayoutsAdminPINUp installs the `admin_pin_hash` column on the
// `layouts` table (§7a.7). The column is NOT NULL — every layout
// (including the bootstrap system row) MUST carry a digest so the
// sudo flow has a comparable hash on day one. SQLite forbids
// adding a NOT NULL column without a default, hence the empty-string
// fallback. The seeder rotates the bootstrap PIN to the well-known
// "0000" value (logged on first boot, mirrors the admin login
// PIN-warning UX) on freshly-created installations; existing rows
// keep their empty digest until an admin rotates it via the layout
// settings dialog (the empty digest can never match any PIN, so the
// migration is non-destructive but does deactivate sudo until the
// rotation happens — same trade-off as the admin PIN rotation).
func addLayoutsAdminPINUp(s *rel.Schema) {
	s.AlterTable("layouts", func(t *rel.AlterTable) {
		t.String("admin_pin_hash", rel.Default(""))
	})
}

func addLayoutsAdminPINDown(s *rel.Schema) {
	s.AlterTable("layouts", func(t *rel.AlterTable) {
		t.DropColumn("admin_pin_hash")
	})
}

// createSudoElevationsUp installs the `sudo_elevations` table that
// backs domain.SudoElevation (§7a.7). Sudo is admin-only; the unique
// index guarantees at most one active row per (user_id, layout_id)
// so the service-level "renew the timer" path is a single upsert.
// Expired rows are reaped by the janitor goroutine; nothing relies
// on stale rows hanging around once `expires_at` passes.
func createSudoElevationsUp(s *rel.Schema) {
	s.CreateTable("sudo_elevations", func(t *rel.Table) {
		t.ID("id")
		t.Int("user_id", rel.Unsigned(true))
		t.Int("layout_id", rel.Unsigned(true))
		t.DateTime("granted_at")
		t.DateTime("expires_at")

		t.Unique([]string{"user_id", "layout_id"})
	})
	s.Exec(rel.Raw(`CREATE INDEX sudo_elevations_expires_at ON sudo_elevations(expires_at)`))
}

func createSudoElevationsDown(s *rel.Schema) {
	s.DropTable("sudo_elevations")
}
