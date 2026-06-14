package domain

import "time"

// InterlockingSession records a signalman's occupation of an
// interlocking. At most one row per interlocking may have EndedAt ==
// nil (enforced by a partial unique index).
type InterlockingSession struct {
	ID              uint
	InterlockingID  uint `db:"interlocking_id"`
	SignalmanUserID uint `db:"signalman_user_id"`
	StartedAt       time.Time
	EndedAt         *time.Time `db:"ended_at"`
}

// Table tells REL which physical table backs this struct.
func (InterlockingSession) Table() string { return "interlocking_sessions" }
