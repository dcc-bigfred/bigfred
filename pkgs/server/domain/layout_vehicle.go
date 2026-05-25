package domain

import "time"

// LayoutVehicle pins a registered Vehicle to a layout's operating
// roster (§3a.1). A vehicle must be registered globally before it
// can be attached to a layout; only the vehicle owner may add or
// remove their own row.
//
// Roster membership is distinct from leasing: it is visibility /
// participation in the current operating session, not a transfer of
// driving authority. Every participant of the layout sees the
// roster on the dashboard (§6.3c).
type LayoutVehicle struct {
	ID            uint
	LayoutID      uint      `db:"layout_id"`
	VehicleID     uint      `db:"vehicle_id"`
	AddedByUserID uint      `db:"added_by_user_id"` // must equal Vehicle.OwnerUserID at insert time
	AddedAt       time.Time `db:"added_at"`
}

// Table tells REL which physical table backs this struct.
func (LayoutVehicle) Table() string { return "layout_vehicles" }

// LayoutTrain pins a Train to a layout's operating roster — the
// train-shaped sibling of LayoutVehicle. The spec (§3a.1) leaves
// trains layout-agnostic by default (ownership travels with the
// user), but having an explicit roster row lets every participant
// see "this consist is on the floor right now" without inferring it
// from per-vehicle membership.
type LayoutTrain struct {
	ID            uint
	LayoutID      uint      `db:"layout_id"`
	TrainID       uint      `db:"train_id"`
	AddedByUserID uint      `db:"added_by_user_id"` // must equal Train.OwnerUserID at insert time
	AddedAt       time.Time `db:"added_at"`
}

// Table tells REL which physical table backs this struct.
func (LayoutTrain) Table() string { return "layout_trains" }
