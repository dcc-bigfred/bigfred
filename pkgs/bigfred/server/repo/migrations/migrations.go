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
	remapLegacyMigrationVersions(ctx, repo)
	m := migrator.New(repo)
	register(&m)
	m.Migrate(ctx)
}

// migrationVersion encodes YYYYMMDD + a per-day sequence into an int
// that fits on 32-bit platforms (CI cross-builds linux/arm GOARM=7).
// go-rel's migrator.Register takes int, not int64.
func migrationVersion(date, seq int) int {
	return date*1000 + seq
}

// remapLegacyMigrationVersions rewrites pre-2026-06-18 epoch stamps
// (YYYYMMDD*1_000_000+seq) into the compact form above. Existing DBs
// store versions as BIGINT so the old values survived on amd64; without
// this remap the migrator would either re-apply or panic on mismatch.
func remapLegacyMigrationVersions(ctx context.Context, repo rel.Repository) {
	_, _, _ = repo.Exec(ctx, `
		UPDATE rel_schema_versions
		SET version = (version / 1000000) * 1000 + (version % 1000000)
		WHERE version > 1000000000000
	`)
}

// register is the single place where new migrations are wired in.
// Versions MUST be monotonically increasing integers. Use
// migrationVersion(YYYYMMDD, seq) so IDs stay int32-safe on armv7.
func register(m *migrator.Migrator) {
	m.Register(migrationVersion(20260523, 1), createUsersUp, createUsersDown)
	m.Register(migrationVersion(20260525, 1), createLayoutsUp, createLayoutsDown)
	m.Register(migrationVersion(20260525, 2), createInterlockingsUp, createInterlockingsDown)
	m.Register(migrationVersion(20260525, 3), createLayoutSignalmenUp, createLayoutSignalmenDown)
	m.Register(migrationVersion(20260525, 4), createInterlockingSessionsUp, createInterlockingSessionsDown)
	m.Register(migrationVersion(20260525, 5), createDCCAddressRangesUp, createDCCAddressRangesDown)
	m.Register(migrationVersion(20260525, 6), createVehiclesUp, createVehiclesDown)
	m.Register(migrationVersion(20260525, 7), createTrainsUp, createTrainsDown)
	m.Register(migrationVersion(20260525, 8), createLayoutVehiclesUp, createLayoutVehiclesDown)
	m.Register(migrationVersion(20260525, 9), addUsersActiveColumnUp, addUsersActiveColumnDown)
	m.Register(migrationVersion(20260525, 10), addLayoutsAdminPINUp, addLayoutsAdminPINDown)
	m.Register(migrationVersion(20260525, 11), createSudoElevationsUp, createSudoElevationsDown)
	m.Register(migrationVersion(20260525, 12), dropSystemLayoutLockCheckUp, dropSystemLayoutLockCheckDown)
	m.Register(migrationVersion(20260526, 1), createCommandStationsUp, createCommandStationsDown)
	m.Register(migrationVersion(20260526, 2), createLayoutCommandStationsUp, createLayoutCommandStationsDown)
	m.Register(migrationVersion(20260604, 1), createVehicleTemplatesAndDccFunctionsUp, createVehicleTemplatesAndDccFunctionsDown)
	m.Register(migrationVersion(20260604, 2), addVehicleDeadManSwitchColumnsUp, addVehicleDeadManSwitchColumnsDown)

	m.Register(migrationVersion(20260608, 1), seedPikoSm31TemplateUp, seedPikoSm31TemplateDown)
	m.Register(migrationVersion(20260608, 2), seedPikoZimoSm31TemplateUp, seedPikoZimoSm31TemplateDown)
	m.Register(migrationVersion(20260608, 3), seedSchlesienModelleEsuLoksoundTemplateUp, seedSchlesienModelleEsuLoksoundTemplateDown)
	m.Register(migrationVersion(20260608, 4), seedPikoXpSu46TemplateUp, seedPikoXpSu46TemplateDown)
	m.Register(migrationVersion(20260608, 5), seedPikoXpSp45Su45TemplateUp, seedPikoXpSp45Su45TemplateDown)
	m.Register(migrationVersion(20260608, 6), seedPikoDcc24EsuSp45Su45TemplateUp, seedPikoDcc24EsuSp45Su45TemplateDown)
	m.Register(migrationVersion(20260608, 7), seedRocoZimo810TemplateUp, seedRocoZimo810TemplateDown)
	m.Register(migrationVersion(20260608, 8), seedPikoXpSt44TemplateUp, seedPikoXpSt44TemplateDown)
	m.Register(migrationVersion(20260608, 9), seedPikoDcc24EsuEp07Eu07TemplateUp, seedPikoDcc24EsuEp07Eu07TemplateDown)
	m.Register(migrationVersion(20260608, 10), seedSchlesienModelleEsuEp07Eu07TemplateUp, seedSchlesienModelleEsuEp07Eu07TemplateDown)
	m.Register(migrationVersion(20260608, 11), seedPikoXpEn57TemplateUp, seedPikoXpEn57TemplateDown)
	m.Register(migrationVersion(20260608, 12), seedRoboEsuEn57TemplateUp, seedRoboEsuEn57TemplateDown)
	m.Register(migrationVersion(20260608, 13), seedRoboDigisoundEn57TemplateUp, seedRoboDigisoundEn57TemplateDown)
	m.Register(migrationVersion(20260608, 14), seedRocoDcc24ZimoTy2TemplateUp, seedRocoDcc24ZimoTy2TemplateDown)
	m.Register(migrationVersion(20260608, 15), seedRocoDcc24EsuTy2TemplateUp, seedRocoDcc24EsuTy2TemplateDown)
	m.Register(migrationVersion(20260608, 16), seedPikoXpEt21TemplateUp, seedPikoXpEt21TemplateDown)
	m.Register(migrationVersion(20260615, 1), createVehicleLeasesUp, createVehicleLeasesDown)
	m.Register(migrationVersion(20260615, 2), createTrainLeasesUp, createTrainLeasesDown)
	m.Register(migrationVersion(20260615, 3), createTakeoverRequestsUp, createTakeoverRequestsDown)
	m.Register(migrationVersion(20260615, 4), seedSchlesienModelleDcc24EsuEp07V3TemplateUp, seedSchlesienModelleDcc24EsuEp07V3TemplateDown)
	m.Register(migrationVersion(20260616, 1), addTrainMemberSpeedMultiplierUp, addTrainMemberSpeedMultiplierDown)
	m.Register(migrationVersion(20260617, 1), addTrainMemberExcludeFromSpeedUp, addTrainMemberExcludeFromSpeedDown)
	m.Register(migrationVersion(20260617, 2), addTrainMemberStartDelayMsUp, addTrainMemberStartDelayMsDown)
	m.Register(migrationVersion(20260617, 3), addTrainMemberAccelRampUp, addTrainMemberAccelRampDown)
	m.Register(migrationVersion(20260617, 4), addTrainMemberBrakeRampUp, addTrainMemberBrakeRampDown)
	m.Register(migrationVersion(20260617, 5), seedRailboxRb23xxEp07Eu07TemplateUp, seedRailboxRb23xxEp07Eu07TemplateDown)
	m.Register(migrationVersion(20260617, 6), seedRailboxRb23xxSt44TemplateUp, seedRailboxRb23xxSt44TemplateDown)
	m.Register(migrationVersion(20260617, 7), seedRailboxRb23xxSm42TemplateUp, seedRailboxRb23xxSm42TemplateDown)
	m.Register(migrationVersion(20260617, 8), seedRailboxRb23xxEs64TemplateUp, seedRailboxRb23xxEs64TemplateDown)
	m.Register(migrationVersion(20260617, 9), seedRailboxRb23xxVectronTemplateUp, seedRailboxRb23xxVectronTemplateDown)
	m.Register(migrationVersion(20260617, 10), seedRailboxRb23xxEp09TemplateUp, seedRailboxRb23xxEp09TemplateDown)
	m.Register(migrationVersion(20260617, 11), seedRailboxRb23xxSm31TemplateUp, seedRailboxRb23xxSm31TemplateDown)
	m.Register(migrationVersion(20260617, 12), seedRailboxRb23xxEp08TemplateUp, seedRailboxRb23xxEp08TemplateDown)
	m.Register(migrationVersion(20260617, 13), seedRailboxRb23xxEt22TemplateUp, seedRailboxRb23xxEt22TemplateDown)
	m.Register(migrationVersion(20260617, 14), seedRailboxRb23xxEn57TemplateUp, seedRailboxRb23xxEn57TemplateDown)
	m.Register(migrationVersion(20260617, 15), seedRailboxRb23xxSu45TemplateUp, seedRailboxRb23xxSu45TemplateDown)
	m.Register(migrationVersion(20260617, 16), seedRailboxRb23xxBr232LudmilaTemplateUp, seedRailboxRb23xxBr232LudmilaTemplateDown)
	m.Register(migrationVersion(20260617, 17), seedRailboxRb23xxTp1TemplateUp, seedRailboxRb23xxTp1TemplateDown)
	m.Register(migrationVersion(20260621, 1), migrateVehicleTrainStringIDsUp, migrateVehicleTrainStringIDsDown)
	m.Register(migrationVersion(20260622, 1), addUsersOrganizationColumnUp, addUsersOrganizationColumnDown)
	m.Register(migrationVersion(20260622, 2), addLeaseSpeedLimitColumnUp, addLeaseSpeedLimitColumnDown)
	m.Register(migrationVersion(20260623, 1), addCommandStationTimingColumnsUp, addCommandStationTimingColumnsDown)
	m.Register(migrationVersion(20260623, 2), addCommandStationPollIntervalColumnUp, addCommandStationPollIntervalColumnDown)
}

// createCommandStationsUp installs the `command_stations` catalogue
// row backing domain.CommandStation (§7e). One row per physical DCC
// command station. `kind` is a closed enum that drives which driver
// `pkgs/loco/commandstation` should construct; `connection_uri` is
// a kind-specific URI parsed by the daemon (e.g.
// `udp://192.168.1.10:21105` for z21, `serial:///dev/ttyUSB0:57600`
// for loconet-serial). `speed_steps` is the catalogue default the
// daemon advertises to clients; admins may override per session in
// later milestones.
func createCommandStationsUp(s *rel.Schema) {
	s.CreateTable("command_stations", func(t *rel.Table) {
		t.ID("id")
		t.String("name")
		t.String("kind")
		t.Text("connection_uri", rel.Default(""))
		t.Int("speed_steps", rel.Unsigned(true), rel.Default(128))
		t.DateTime("created_at")
		t.DateTime("updated_at")

		t.Unique([]string{"name"})
		t.Fragment("CHECK (kind IN ('z21','loconet_serial','loconet_tcp'))")
		t.Fragment("CHECK (speed_steps IN (14,28,128))")
	})
}

func createCommandStationsDown(s *rel.Schema) {
	s.DropTable("command_stations")
}

// createLayoutCommandStationsUp installs the join between layouts
// and command_stations. A command station may be attached to many
// layouts (e.g. a roving Z21 lent between rooms); the UNIQUE index
// makes (layout_id, command_station_id) a set rather than a multi-
// set. `dcc-bus` daemons are keyed by this pair (§7e.2).
func createLayoutCommandStationsUp(s *rel.Schema) {
	s.CreateTable("layout_command_stations", func(t *rel.Table) {
		t.ID("id")
		t.Int("layout_id", rel.Unsigned(true))
		t.Int("command_station_id", rel.Unsigned(true))
		t.Int("added_by_user_id", rel.Unsigned(true), rel.Default(0))
		t.DateTime("added_at")

		t.Unique([]string{"layout_id", "command_station_id"})
	})
	s.Exec(rel.Raw(`CREATE INDEX layout_command_stations_layout_id ON layout_command_stations(layout_id)`))
}

func createLayoutCommandStationsDown(s *rel.Schema) {
	s.DropTable("layout_command_stations")
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
// (§3a.1). On top of the obvious columns it installs a partial unique
// index on `is_system` WHERE is_system = 1 — guarantees at most one
// system row exists. SQLite does not support partial indexes through
// REL's typed Schema builder, hence the raw s.Exec.
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

// addUsersOrganizationColumnUp stores the optional club / organisation
// label shown on the user profile.
func addUsersOrganizationColumnUp(s *rel.Schema) {
	s.AlterTable("users", func(t *rel.AlterTable) {
		t.String("organization", rel.Default(""))
	})
}

func addUsersOrganizationColumnDown(s *rel.Schema) {
	s.AlterTable("users", func(t *rel.AlterTable) {
		t.DropColumn("organization")
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

// dropSystemLayoutLockCheckUp removes the SQLite CHECK that forbade
// locking the system layout. SQLite cannot drop a CHECK in place, so
// the table is recreated without it.
func dropSystemLayoutLockCheckUp(s *rel.Schema) {
	s.Exec(rel.Raw(`
		CREATE TABLE "layouts__migration" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"name" VARCHAR(255),
			"is_system" BOOL DEFAULT 0,
			"locked" BOOL DEFAULT 0,
			"created_by" UNSIGNED INTEGER DEFAULT 0,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"admin_pin_hash" VARCHAR(255) DEFAULT '',
			UNIQUE ("name")
		)
	`))
	s.Exec(rel.Raw(`
		INSERT INTO "layouts__migration"
			("id", "name", "is_system", "locked", "created_by", "created_at", "updated_at", "admin_pin_hash")
		SELECT
			"id", "name", "is_system", "locked", "created_by", "created_at", "updated_at", "admin_pin_hash"
		FROM "layouts"
	`))
	s.Exec(rel.Raw(`DROP TABLE "layouts"`))
	s.Exec(rel.Raw(`ALTER TABLE "layouts__migration" RENAME TO "layouts"`))
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX layouts_unique_system ON layouts(is_system) WHERE is_system = 1`))
}

func dropSystemLayoutLockCheckDown(s *rel.Schema) {
	s.Exec(rel.Raw(`
		CREATE TABLE "layouts__migration" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"name" VARCHAR(255),
			"is_system" BOOL DEFAULT 0,
			"locked" BOOL DEFAULT 0,
			"created_by" UNSIGNED INTEGER DEFAULT 0,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"admin_pin_hash" VARCHAR(255) DEFAULT '',
			UNIQUE ("name"),
			CHECK (NOT (is_system = 1 AND locked = 1))
		)
	`))
	s.Exec(rel.Raw(`
		INSERT INTO "layouts__migration"
			("id", "name", "is_system", "locked", "created_by", "created_at", "updated_at", "admin_pin_hash")
		SELECT
			"id", "name", "is_system", "locked", "created_by", "created_at", "updated_at", "admin_pin_hash"
		FROM "layouts"
	`))
	s.Exec(rel.Raw(`DROP TABLE "layouts"`))
	s.Exec(rel.Raw(`ALTER TABLE "layouts__migration" RENAME TO "layouts"`))
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX layouts_unique_system ON layouts(is_system) WHERE is_system = 1`))
}

// createVehicleTemplatesAndDccFunctionsUp installs vehicle templates and
// the unified dcc_functions table (§3a.6.0).
func createVehicleTemplatesAndDccFunctionsUp(s *rel.Schema) {
	s.CreateTable("vehicle_templates", func(t *rel.Table) {
		t.ID("id")
		t.String("name")
		t.Text("description", rel.Default(""))
		t.Int("owner_user_id", rel.Unsigned(true))
		t.Int("version", rel.Default(1))
		t.DateTime("created_at")
		t.DateTime("updated_at")

		t.Unique([]string{"name"})
	})
	s.Exec(rel.Raw(`CREATE INDEX vehicle_templates_owner_user_id ON vehicle_templates(owner_user_id)`))

	s.AlterTable("vehicles", func(t *rel.AlterTable) {
		t.Int("template_id", rel.Unsigned(true), rel.Required(false))
		t.DateTime("functions_detached_at", rel.Required(false))
	})

	s.CreateTable("dcc_functions", func(t *rel.Table) {
		t.ID("id")
		t.Int("vehicle_id", rel.Unsigned(true), rel.Required(false))
		t.Int("template_id", rel.Unsigned(true), rel.Required(false))
		t.Int("num", rel.Unsigned(true))
		t.String("name")
		t.String("icon")
		t.Int("position")
		t.DateTime("created_at")
		t.DateTime("updated_at")

		t.Fragment("CHECK (num BETWEEN 0 AND 31)")
		t.Fragment(`CHECK (
			(vehicle_id IS NOT NULL AND template_id IS NULL)
			OR (vehicle_id IS NULL AND template_id IS NOT NULL)
		)`)
	})
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX dcc_functions_vehicle_num
		ON dcc_functions(vehicle_id, num) WHERE vehicle_id IS NOT NULL`))
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX dcc_functions_template_num
		ON dcc_functions(template_id, num) WHERE template_id IS NOT NULL`))
}

func createVehicleTemplatesAndDccFunctionsDown(s *rel.Schema) {
	s.DropTable("dcc_functions")
	s.AlterTable("vehicles", func(t *rel.AlterTable) {
		t.DropColumn("template_id")
		t.DropColumn("functions_detached_at")
	})
	s.DropTable("vehicle_templates")
}

// addVehicleDeadManSwitchColumnsUp stores per-vehicle dead-man's switch
// function mappings and behaviour (§7e.5).
func addVehicleDeadManSwitchColumnsUp(s *rel.Schema) {
	s.AlterTable("vehicles", func(t *rel.AlterTable) {
		t.Int("rp1_function", rel.Unsigned(true), rel.Default("2"))
		t.Int("emergency_lights_function", rel.Unsigned(true), rel.Default("0"))
		t.String("deadman_switch_option", rel.Default("stop"))
	})
}

func addVehicleDeadManSwitchColumnsDown(s *rel.Schema) {
	s.AlterTable("vehicles", func(t *rel.AlterTable) {
		t.DropColumn("rp1_function")
		t.DropColumn("emergency_lights_function")
		t.DropColumn("deadman_switch_option")
	})
}

func createVehicleLeasesUp(s *rel.Schema) {
	s.CreateTable("vehicle_leases", func(t *rel.Table) {
		t.ID("id")
		t.Int("vehicle_id", rel.Unsigned(true))
		t.Int("from_user_id", rel.Unsigned(true))
		t.Int("to_user_id", rel.Unsigned(true))
		t.DateTime("started_at")
		t.DateTime("expires_at")
		t.DateTime("revoked_at", rel.Required(false))
	})
	s.Exec(rel.Raw(`CREATE INDEX vehicle_leases_vehicle_id_expires_at ON vehicle_leases(vehicle_id, expires_at)`))
}

func createVehicleLeasesDown(s *rel.Schema) {
	s.DropTable("vehicle_leases")
}

func createTrainLeasesUp(s *rel.Schema) {
	s.CreateTable("train_leases", func(t *rel.Table) {
		t.ID("id")
		t.Int("train_id", rel.Unsigned(true))
		t.Int("from_user_id", rel.Unsigned(true))
		t.Int("to_user_id", rel.Unsigned(true))
		t.DateTime("started_at")
		t.DateTime("expires_at")
		t.DateTime("revoked_at", rel.Required(false))
	})
	s.Exec(rel.Raw(`CREATE INDEX train_leases_train_id_expires_at ON train_leases(train_id, expires_at)`))
}

func createTrainLeasesDown(s *rel.Schema) {
	s.DropTable("train_leases")
}

func createTakeoverRequestsUp(s *rel.Schema) {
	s.CreateTable("takeover_requests", func(t *rel.Table) {
		t.ID("id")
		t.Int("layout_id", rel.Unsigned(true))
		t.Int("interlocking_id", rel.Unsigned(true))
		t.Int("signalman_user_id", rel.Unsigned(true))
		t.Int("driver_user_id", rel.Unsigned(true))
		t.String("target")
		t.Int("target_id", rel.Unsigned(true))
		t.DateTime("requested_at")
		t.DateTime("decision_at", rel.Required(false))
		t.DateTime("auto_grant_at")
		t.Int("granted_lease_id", rel.Unsigned(true), rel.Required(false))
		t.DateTime("released_at", rel.Required(false))
		t.String("state")
	})
	s.Exec(rel.Raw(`CREATE INDEX takeover_requests_state_auto_grant_at ON takeover_requests(state, auto_grant_at)`))
	s.Exec(rel.Raw(`CREATE INDEX takeover_requests_signalman_state ON takeover_requests(signalman_user_id, state)`))
}

func createTakeoverRequestsDown(s *rel.Schema) {
	s.DropTable("takeover_requests")
}

// addTrainMemberSpeedMultiplierUp adds per-member speed calibration for
// train throttle fan-out (leading vehicle × multiplier).
func addTrainMemberSpeedMultiplierUp(s *rel.Schema) {
	s.Exec(rel.Raw(`ALTER TABLE train_members ADD COLUMN speed_multiplier REAL NOT NULL DEFAULT 1.0`))
}

func addTrainMemberSpeedMultiplierDown(s *rel.Schema) {
	// SQLite cannot DROP COLUMN in older schemas; leave column in place.
}

// addTrainMemberExcludeFromSpeedUp lets a consist member opt out of
// train.setSpeed fan-out while remaining available for function control.
func addTrainMemberExcludeFromSpeedUp(s *rel.Schema) {
	s.Exec(rel.Raw(`ALTER TABLE train_members ADD COLUMN exclude_from_speed INTEGER NOT NULL DEFAULT 0`))
}

func addTrainMemberExcludeFromSpeedDown(s *rel.Schema) {
	// SQLite cannot DROP COLUMN in older schemas; leave column in place.
}

// addTrainMemberStartDelayMsUp stores per-member consist start delay.
func addTrainMemberStartDelayMsUp(s *rel.Schema) {
	s.Exec(rel.Raw(`ALTER TABLE train_members ADD COLUMN start_delay_ms INTEGER NOT NULL DEFAULT 0`))
}

func addTrainMemberStartDelayMsDown(s *rel.Schema) {
	// SQLite cannot DROP COLUMN in older schemas; leave column in place.
}

// addTrainMemberAccelRampUp stores per-member consist acceleration ramp.
func addTrainMemberAccelRampUp(s *rel.Schema) {
	s.Exec(rel.Raw(`ALTER TABLE train_members ADD COLUMN accel_ramp_ms INTEGER NOT NULL DEFAULT 0`))
	s.Exec(rel.Raw(`ALTER TABLE train_members ADD COLUMN accel_ramp_max_steps INTEGER NOT NULL DEFAULT 1`))
}

func addTrainMemberAccelRampDown(s *rel.Schema) {
	// SQLite cannot DROP COLUMN in older schemas; leave columns in place.
}

// addTrainMemberBrakeRampUp stores per-member consist braking ramp.
func addTrainMemberBrakeRampUp(s *rel.Schema) {
	s.Exec(rel.Raw(`ALTER TABLE train_members ADD COLUMN brake_ramp_ms INTEGER NOT NULL DEFAULT 0`))
	s.Exec(rel.Raw(`ALTER TABLE train_members ADD COLUMN brake_ramp_max_steps INTEGER NOT NULL DEFAULT 1`))
}

func addTrainMemberBrakeRampDown(s *rel.Schema) {
	// SQLite cannot DROP COLUMN in older schemas; leave columns in place.
}

func addLeaseSpeedLimitColumnUp(s *rel.Schema) {
	s.Exec(rel.Raw(`ALTER TABLE vehicle_leases ADD COLUMN speed_limit INTEGER NOT NULL DEFAULT 0`))
	s.Exec(rel.Raw(`ALTER TABLE train_leases ADD COLUMN speed_limit INTEGER NOT NULL DEFAULT 0`))
}

func addLeaseSpeedLimitColumnDown(s *rel.Schema) {
	// SQLite cannot DROP COLUMN in older schemas; leave columns in place.
}

// addCommandStationTimingColumnsUp stores per-station WS ping and dead-man
// windows forwarded to dcc-bus as --heartbeat-secs / --deadman-secs.
func addCommandStationTimingColumnsUp(s *rel.Schema) {
	s.Exec(rel.Raw(`ALTER TABLE command_stations ADD COLUMN heartbeat_secs REAL NOT NULL DEFAULT 2`))
	s.Exec(rel.Raw(`ALTER TABLE command_stations ADD COLUMN deadman_secs REAL NOT NULL DEFAULT 6`))
}

func addCommandStationTimingColumnsDown(s *rel.Schema) {
	// SQLite cannot DROP COLUMN in older schemas; leave columns in place.
}

// addCommandStationPollIntervalColumnUp stores per-station state-feed polling
// cadence forwarded to dcc-bus as --poll-interval-ms (0 == daemon default).
func addCommandStationPollIntervalColumnUp(s *rel.Schema) {
	s.Exec(rel.Raw(`ALTER TABLE command_stations ADD COLUMN poll_interval_ms INTEGER NOT NULL DEFAULT 0`))
}

func addCommandStationPollIntervalColumnDown(s *rel.Schema) {
	// SQLite cannot DROP COLUMN in older schemas; leave columns in place.
}
