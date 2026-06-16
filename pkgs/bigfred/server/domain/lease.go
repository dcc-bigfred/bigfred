package domain

import "time"

// Lease is the behaviour shared by every lease row (vehicle or train).
// It lets a single generic repository persist and query both kinds
// without duplicating the active-window logic.
type Lease interface {
	// Table is the REL table name for the concrete lease type.
	Table() string
	// IsActive reports whether the lease is in force at `now`.
	IsActive(now time.Time) bool
}

// VehicleLease grants a time-bounded right to drive a single vehicle.
// Edit rights stay with the owner (§3a.1).
type VehicleLease struct {
	ID         uint
	VehicleID  uint       `db:"vehicle_id"`
	FromUserID uint       `db:"from_user_id"`
	ToUserID   uint       `db:"to_user_id"`
	StartedAt  time.Time  `db:"started_at"`
	ExpiresAt  time.Time  `db:"expires_at"`
	RevokedAt  *time.Time `db:"revoked_at"`
}

func (VehicleLease) Table() string { return "vehicle_leases" }

// TrainLease grants a time-bounded right to drive an entire train.
type TrainLease struct {
	ID         uint
	TrainID    uint       `db:"train_id"`
	FromUserID uint       `db:"from_user_id"`
	ToUserID   uint       `db:"to_user_id"`
	StartedAt  time.Time  `db:"started_at"`
	ExpiresAt  time.Time  `db:"expires_at"`
	RevokedAt  *time.Time `db:"revoked_at"`
}

func (TrainLease) Table() string { return "train_leases" }

// IsActive reports whether the lease is currently in force.
func (l VehicleLease) IsActive(now time.Time) bool {
	return l.RevokedAt == nil && now.Before(l.ExpiresAt)
}

// IsActive reports whether the lease is currently in force.
func (l TrainLease) IsActive(now time.Time) bool {
	return l.RevokedAt == nil && now.Before(l.ExpiresAt)
}

// TrainLessee is an active lease holder in a train-scoped drive
// projection (§4.3). It is the value stored in per-train lessee maps
// returned by layout roster resolution.
type TrainLessee struct {
	TrainID  uint
	ToUserID uint
}

// TrainLesseeUserIDs extracts ToUserID from a train-lessee slice.
func TrainLesseeUserIDs(lessees []TrainLessee) []uint {
	if len(lessees) == 0 {
		return nil
	}
	out := make([]uint, len(lessees))
	for i, l := range lessees {
		out[i] = l.ToUserID
	}
	return out
}
