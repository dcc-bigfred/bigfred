package domain

import "time"

// VehicleTemplate pre-defines a function list for a class of vehicles (§3a.6).
type VehicleTemplate struct {
	ID          uint
	Name        string
	Description string
	OwnerUserID uint `db:"owner_user_id"`
	Version     int
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// Table tells REL which physical table backs this struct.
func (VehicleTemplate) Table() string { return "vehicle_templates" }
