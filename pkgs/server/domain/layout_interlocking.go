package domain

import "time"

// LayoutInterlocking whitelists which interlockings are visible to
// drivers (and which may be occupied) within a specific layout (§3a.1).
type LayoutInterlocking struct {
	ID             uint
	LayoutID       uint `db:"layout_id"`
	InterlockingID uint `db:"interlocking_id"`
	AddedByUserID  uint `db:"added_by_user_id"`
	AddedAt        time.Time
}

// Table tells REL which physical table backs this struct.
func (LayoutInterlocking) Table() string { return "layout_interlockings" }
