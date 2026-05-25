package domain

import "time"

// SystemLayoutName is the stable, immutable Name of the bootstrap
// system layout. Per §3a.1 / §7a.1 of the architecture spec the row
// is identified by IsSystem = true, but the canonical Name is kept
// as a const so seeders and tests can refer to it without resorting
// to a magic string. The user-facing label is rendered through the
// i18n key `layout:system_default_label` ("Domyślna (warsztat)" /
// "Default (workshop)") — never compare or display Name directly.
const SystemLayoutName = "default"

// Layout (Polish: makieta) is a modeling event / room. Picked by the
// user on the login form (§7a.1) and pinned to the drive session for
// its lifetime. The two boolean flags steer its lifecycle:
//
//   - IsSystem: true ONLY for the bootstrap row. The system layout
//     cannot be deleted, cannot be locked, its Name and IsSystem
//     fields are immutable, and the set of attached command stations
//     is a virtual view of `command_stations` (admin endpoints that
//     try to mutate it return 422). The system row is seeded with
//     Name = "default"; the UI renders it via the i18n key
//     `layout:system_default_label`.
//
//   - Locked: false right after creation. Admin may toggle it on a
//     non-system layout. A locked layout is hidden from the
//     unauthenticated login dropdown (GET /api/v1/layouts/login), but
//     already-running sessions keep working until they close on their
//     own.
//
// The full §3a.1 entity carries `Signalmen`, `Interlockings` and
// `CommandStations` association slices. These will be added together
// with the matching join tables in the milestone that introduces
// command stations + interlockings (see §9 M4–M5). Keeping the struct
// minimal for now avoids dead columns in the bootstrap migration.
type Layout struct {
	ID        uint
	Name      string
	IsSystem  bool      `db:"is_system"`
	Locked    bool
	CreatedBy uint      `db:"created_by"` // admin user ID; 0 for the system seed
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Table tells REL which physical table backs this struct.
func (Layout) Table() string { return "layouts" }

// IsDefault returns true for the bootstrap system layout. Callers
// must use this helper (or the IsSystem field) — never compare Name
// against the string literal, because the displayed name comes from
// i18n while the stored Name is a stable system marker.
func (l Layout) IsDefault() bool { return l.IsSystem }
