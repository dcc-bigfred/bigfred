package domain

import "time"

// LayoutSignalman grants the signalman role to UserID while they are
// active in LayoutID (§7a.2). Administered per layout; ExpiresAt nil
// means the grant is permanent inside that layout.
type LayoutSignalman struct {
	ID        uint
	LayoutID  uint `db:"layout_id"`
	UserID    uint `db:"user_id"`
	GrantedBy uint `db:"granted_by"`
	GrantedAt time.Time
	ExpiresAt *time.Time `db:"expires_at"`
}

// Table tells REL which physical table backs this struct.
func (LayoutSignalman) Table() string { return "layout_signalmen" }
