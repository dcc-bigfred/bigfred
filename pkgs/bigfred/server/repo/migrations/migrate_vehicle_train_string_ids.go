package migrations

import "github.com/go-rel/rel"

// migrateVehicleTrainStringIDsUp converts vehicle and train catalogue primary
// keys (and every referencing FK) from INTEGER AUTOINCREMENT to prefixed TEXT
// ids (V-1 legacy, V-{nanoid} for new rows). Adds external_id + source for
// future idempotent import.
func migrateVehicleTrainStringIDsUp(s *rel.Schema) {
	s.Exec(rel.Raw(`PRAGMA foreign_keys=OFF`))

	// --- vehicles ---
	s.Exec(rel.Raw(`
		CREATE TABLE "vehicles__migration" (
			"id" TEXT PRIMARY KEY NOT NULL,
			"external_id" TEXT,
			"source" TEXT NOT NULL DEFAULT 'local',
			"dcc_address" INTEGER,
			"owner_user_id" INTEGER NOT NULL,
			"name" TEXT NOT NULL,
			"kind" TEXT NOT NULL,
			"number" TEXT NOT NULL DEFAULT '',
			"template_id" INTEGER,
			"functions_detached_at" DATETIME,
			"rp1_function" INTEGER NOT NULL DEFAULT 2,
			"emergency_lights_function" INTEGER NOT NULL DEFAULT 0,
			"deadman_switch_option" TEXT NOT NULL DEFAULT 'stop',
			"created_at" DATETIME NOT NULL,
			"updated_at" DATETIME NOT NULL
		)
	`))
	s.Exec(rel.Raw(`
		INSERT INTO "vehicles__migration"
			("id", "external_id", "source", "dcc_address", "owner_user_id", "name", "kind", "number",
			 "template_id", "functions_detached_at", "rp1_function", "emergency_lights_function",
			 "deadman_switch_option", "created_at", "updated_at")
		SELECT
			'V-' || "id", NULL, 'local', "dcc_address", "owner_user_id", "name", "kind", "number",
			"template_id", "functions_detached_at", "rp1_function", "emergency_lights_function",
			"deadman_switch_option", "created_at", "updated_at"
		FROM "vehicles"
	`))
	s.Exec(rel.Raw(`DROP TABLE "vehicles"`))
	s.Exec(rel.Raw(`ALTER TABLE "vehicles__migration" RENAME TO "vehicles"`))
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX vehicles_unique_dcc_address ON vehicles(dcc_address) WHERE dcc_address IS NOT NULL`))
	s.Exec(rel.Raw(`CREATE INDEX vehicles_owner_user_id ON vehicles(owner_user_id)`))
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX vehicles_unique_external_source ON vehicles(source, external_id) WHERE external_id IS NOT NULL`))

	// --- trains ---
	s.Exec(rel.Raw(`
		CREATE TABLE "trains__migration" (
			"id" TEXT PRIMARY KEY NOT NULL,
			"external_id" TEXT,
			"source" TEXT NOT NULL DEFAULT 'local',
			"owner_user_id" INTEGER NOT NULL,
			"name" TEXT NOT NULL,
			"created_at" DATETIME NOT NULL,
			"updated_at" DATETIME NOT NULL,
			UNIQUE ("owner_user_id", "name")
		)
	`))
	s.Exec(rel.Raw(`
		INSERT INTO "trains__migration"
			("id", "external_id", "source", "owner_user_id", "name", "created_at", "updated_at")
		SELECT 'T-' || "id", NULL, 'local', "owner_user_id", "name", "created_at", "updated_at"
		FROM "trains"
	`))
	s.Exec(rel.Raw(`DROP TABLE "trains"`))
	s.Exec(rel.Raw(`ALTER TABLE "trains__migration" RENAME TO "trains"`))
	s.Exec(rel.Raw(`CREATE INDEX trains_owner_user_id ON trains(owner_user_id)`))
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX trains_unique_external_source ON trains(source, external_id) WHERE external_id IS NOT NULL`))

	// --- train_members ---
	s.Exec(rel.Raw(`
		CREATE TABLE "train_members__migration" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"train_id" TEXT NOT NULL,
			"vehicle_id" TEXT NOT NULL,
			"position" INTEGER NOT NULL,
			"reversed" INTEGER NOT NULL DEFAULT 0,
			"speed_multiplier" REAL NOT NULL DEFAULT 1.0,
			"exclude_from_speed" INTEGER NOT NULL DEFAULT 0,
			"start_delay_ms" INTEGER NOT NULL DEFAULT 0,
			"accel_ramp_ms" INTEGER NOT NULL DEFAULT 0,
			"accel_ramp_max_steps" INTEGER NOT NULL DEFAULT 1,
			"brake_ramp_ms" INTEGER NOT NULL DEFAULT 0,
			"brake_ramp_max_steps" INTEGER NOT NULL DEFAULT 1,
			UNIQUE ("train_id", "vehicle_id")
		)
	`))
	s.Exec(rel.Raw(`
		INSERT INTO "train_members__migration"
			("id", "train_id", "vehicle_id", "position", "reversed", "speed_multiplier",
			 "exclude_from_speed", "start_delay_ms", "accel_ramp_ms", "accel_ramp_max_steps",
			 "brake_ramp_ms", "brake_ramp_max_steps")
		SELECT
			"id", 'T-' || "train_id", 'V-' || "vehicle_id", "position", "reversed", "speed_multiplier",
			"exclude_from_speed", "start_delay_ms", "accel_ramp_ms", "accel_ramp_max_steps",
			"brake_ramp_ms", "brake_ramp_max_steps"
		FROM "train_members"
	`))
	s.Exec(rel.Raw(`DROP TABLE "train_members"`))
	s.Exec(rel.Raw(`ALTER TABLE "train_members__migration" RENAME TO "train_members"`))
	s.Exec(rel.Raw(`CREATE INDEX train_members_train_id ON train_members(train_id)`))

	// --- layout_vehicles ---
	s.Exec(rel.Raw(`
		CREATE TABLE "layout_vehicles__migration" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"layout_id" INTEGER NOT NULL,
			"vehicle_id" TEXT NOT NULL,
			"added_by_user_id" INTEGER NOT NULL,
			"added_at" DATETIME NOT NULL,
			UNIQUE ("layout_id", "vehicle_id")
		)
	`))
	s.Exec(rel.Raw(`
		INSERT INTO "layout_vehicles__migration"
			("id", "layout_id", "vehicle_id", "added_by_user_id", "added_at")
		SELECT "id", "layout_id", 'V-' || "vehicle_id", "added_by_user_id", "added_at"
		FROM "layout_vehicles"
	`))
	s.Exec(rel.Raw(`DROP TABLE "layout_vehicles"`))
	s.Exec(rel.Raw(`ALTER TABLE "layout_vehicles__migration" RENAME TO "layout_vehicles"`))
	s.Exec(rel.Raw(`CREATE INDEX layout_vehicles_layout_id ON layout_vehicles(layout_id)`))

	// --- layout_trains ---
	s.Exec(rel.Raw(`
		CREATE TABLE "layout_trains__migration" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"layout_id" INTEGER NOT NULL,
			"train_id" TEXT NOT NULL,
			"added_by_user_id" INTEGER NOT NULL,
			"added_at" DATETIME NOT NULL,
			UNIQUE ("layout_id", "train_id")
		)
	`))
	s.Exec(rel.Raw(`
		INSERT INTO "layout_trains__migration"
			("id", "layout_id", "train_id", "added_by_user_id", "added_at")
		SELECT "id", "layout_id", 'T-' || "train_id", "added_by_user_id", "added_at"
		FROM "layout_trains"
	`))
	s.Exec(rel.Raw(`DROP TABLE "layout_trains"`))
	s.Exec(rel.Raw(`ALTER TABLE "layout_trains__migration" RENAME TO "layout_trains"`))
	s.Exec(rel.Raw(`CREATE INDEX layout_trains_layout_id ON layout_trains(layout_id)`))

	// --- vehicle_leases ---
	s.Exec(rel.Raw(`
		CREATE TABLE "vehicle_leases__migration" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"vehicle_id" TEXT NOT NULL,
			"from_user_id" INTEGER NOT NULL,
			"to_user_id" INTEGER NOT NULL,
			"started_at" DATETIME NOT NULL,
			"expires_at" DATETIME NOT NULL,
			"revoked_at" DATETIME
		)
	`))
	s.Exec(rel.Raw(`
		INSERT INTO "vehicle_leases__migration"
			("id", "vehicle_id", "from_user_id", "to_user_id", "started_at", "expires_at", "revoked_at")
		SELECT "id", 'V-' || "vehicle_id", "from_user_id", "to_user_id", "started_at", "expires_at", "revoked_at"
		FROM "vehicle_leases"
	`))
	s.Exec(rel.Raw(`DROP TABLE "vehicle_leases"`))
	s.Exec(rel.Raw(`ALTER TABLE "vehicle_leases__migration" RENAME TO "vehicle_leases"`))
	s.Exec(rel.Raw(`CREATE INDEX vehicle_leases_vehicle_id_expires_at ON vehicle_leases(vehicle_id, expires_at)`))

	// --- train_leases ---
	s.Exec(rel.Raw(`
		CREATE TABLE "train_leases__migration" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"train_id" TEXT NOT NULL,
			"from_user_id" INTEGER NOT NULL,
			"to_user_id" INTEGER NOT NULL,
			"started_at" DATETIME NOT NULL,
			"expires_at" DATETIME NOT NULL,
			"revoked_at" DATETIME
		)
	`))
	s.Exec(rel.Raw(`
		INSERT INTO "train_leases__migration"
			("id", "train_id", "from_user_id", "to_user_id", "started_at", "expires_at", "revoked_at")
		SELECT "id", 'T-' || "train_id", "from_user_id", "to_user_id", "started_at", "expires_at", "revoked_at"
		FROM "train_leases"
	`))
	s.Exec(rel.Raw(`DROP TABLE "train_leases"`))
	s.Exec(rel.Raw(`ALTER TABLE "train_leases__migration" RENAME TO "train_leases"`))
	s.Exec(rel.Raw(`CREATE INDEX train_leases_train_id_expires_at ON train_leases(train_id, expires_at)`))

	// --- dcc_functions (vehicle_id only) ---
	s.Exec(rel.Raw(`
		CREATE TABLE "dcc_functions__migration" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"vehicle_id" TEXT,
			"template_id" INTEGER,
			"num" INTEGER NOT NULL,
			"name" TEXT NOT NULL,
			"icon" TEXT NOT NULL,
			"position" INTEGER NOT NULL,
			"created_at" DATETIME NOT NULL,
			"updated_at" DATETIME NOT NULL,
			CHECK (num BETWEEN 0 AND 31),
			CHECK (
				(vehicle_id IS NOT NULL AND template_id IS NULL)
				OR (vehicle_id IS NULL AND template_id IS NOT NULL)
			)
		)
	`))
	s.Exec(rel.Raw(`
		INSERT INTO "dcc_functions__migration"
			("id", "vehicle_id", "template_id", "num", "name", "icon", "position", "created_at", "updated_at")
		SELECT
			"id",
			CASE WHEN "vehicle_id" IS NOT NULL THEN 'V-' || "vehicle_id" ELSE NULL END,
			"template_id", "num", "name", "icon", "position", "created_at", "updated_at"
		FROM "dcc_functions"
	`))
	s.Exec(rel.Raw(`DROP TABLE "dcc_functions"`))
	s.Exec(rel.Raw(`ALTER TABLE "dcc_functions__migration" RENAME TO "dcc_functions"`))
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX dcc_functions_vehicle_num ON dcc_functions(vehicle_id, num) WHERE vehicle_id IS NOT NULL`))
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX dcc_functions_template_num ON dcc_functions(template_id, num) WHERE template_id IS NOT NULL`))

	// --- takeover_requests ---
	s.Exec(rel.Raw(`
		CREATE TABLE "takeover_requests__migration" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"layout_id" INTEGER NOT NULL,
			"interlocking_id" INTEGER NOT NULL,
			"signalman_user_id" INTEGER NOT NULL,
			"driver_user_id" INTEGER NOT NULL,
			"target" TEXT NOT NULL,
			"target_id" TEXT NOT NULL,
			"requested_at" DATETIME NOT NULL,
			"decision_at" DATETIME,
			"auto_grant_at" DATETIME NOT NULL,
			"granted_lease_id" INTEGER,
			"released_at" DATETIME,
			"state" TEXT NOT NULL
		)
	`))
	s.Exec(rel.Raw(`
		INSERT INTO "takeover_requests__migration"
			("id", "layout_id", "interlocking_id", "signalman_user_id", "driver_user_id",
			 "target", "target_id", "requested_at", "decision_at", "auto_grant_at",
			 "granted_lease_id", "released_at", "state")
		SELECT
			"id", "layout_id", "interlocking_id", "signalman_user_id", "driver_user_id",
			"target",
			CASE
				WHEN "target" = 'vehicle' THEN 'V-' || "target_id"
				WHEN "target" = 'train' THEN 'T-' || "target_id"
				ELSE CAST("target_id" AS TEXT)
			END,
			"requested_at", "decision_at", "auto_grant_at",
			"granted_lease_id", "released_at", "state"
		FROM "takeover_requests"
	`))
	s.Exec(rel.Raw(`DROP TABLE "takeover_requests"`))
	s.Exec(rel.Raw(`ALTER TABLE "takeover_requests__migration" RENAME TO "takeover_requests"`))
	s.Exec(rel.Raw(`CREATE INDEX takeover_requests_state_auto_grant_at ON takeover_requests(state, auto_grant_at)`))
	s.Exec(rel.Raw(`CREATE INDEX takeover_requests_signalman_state ON takeover_requests(signalman_user_id, state)`))

	s.Exec(rel.Raw(`PRAGMA foreign_keys=ON`))
}

func migrateVehicleTrainStringIDsDown(s *rel.Schema) {
	// Irreversible: numeric ids cannot be recovered from prefixed strings.
}
