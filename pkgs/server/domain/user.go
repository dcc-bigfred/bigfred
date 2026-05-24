// Package domain holds the pure entities of the BigFred web application.
//
// Per §3a.1 of the architecture spec, entities here have no persistence
// concerns and no transport concerns — they are mapped onto the database
// by the REL repositories under `pkgs/server/repo`, and serialised to
// JSON by the REST handlers under `pkgs/server/http`.
package domain

import "time"

// Role is the primary, permanent role of a user. The architecture spec
// (§7a.2) lists three values:
//
//   - "driver"    – regular operator
//   - "admin"     – platform administrator
//   - "signalman" – party-scoped grant only (never stored on User.Role)
//
// The string type is preserved instead of an int enum because REL/SQLite
// stores it as TEXT for human readability of the audit log.
type Role string

const (
	RoleDriver    Role = "driver"
	RoleAdmin     Role = "admin"
	RoleSignalman Role = "signalman"
)

// User is the canonical account record. PINHash holds the argon2id
// digest of the user's PIN; the plaintext PIN never leaves
// AuthService.Login (§7a.1).
type User struct {
	ID        uint
	Login     string
	PINHash   string `db:"pin_hash"`
	Role      Role
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Table tells REL which physical table backs this struct.
func (User) Table() string { return "users" }
