package domain

import "time"

// VehicleLease grants a time-bounded right to drive a single vehicle.
// Edit rights stay with the owner (§3a.1).
type VehicleLease struct {
	VehicleID  VehicleID
	FromUserID uint
	ToUserID   uint
	SpeedLimit uint8 // 0 = no cap, 1–100 = percent of max speed for lessee
	StartedAt  time.Time
	ExpiresAt  time.Time
	Source     string // "manual" | "takeover"
}

// TrainLease grants a time-bounded right to drive an entire train.
type TrainLease struct {
	TrainID    TrainID
	FromUserID uint
	ToUserID   uint
	SpeedLimit uint8
	StartedAt  time.Time
	ExpiresAt  time.Time
	Source     string
}

// IsActive reports whether the lease is currently in force.
func (l VehicleLease) IsActive(now time.Time) bool {
	return now.Before(l.ExpiresAt)
}

// IsActive reports whether the lease is currently in force.
func (l TrainLease) IsActive(now time.Time) bool {
	return now.Before(l.ExpiresAt)
}

// VehicleLessee is an active controller on a vehicle from a lease row.
type VehicleLessee struct {
	UserID     uint
	SpeedLimit uint8
}

// VehicleLesseeUserIDs extracts user ids from a vehicle-lessee slice.
func VehicleLesseeUserIDs(lessees []VehicleLessee) []uint {
	if len(lessees) == 0 {
		return nil
	}
	out := make([]uint, len(lessees))
	for i, l := range lessees {
		out[i] = l.UserID
	}
	return out
}

// TrainLessee is an active lease holder in a train-scoped drive
// projection (§4.3). It is the value stored in per-train lessee maps
// returned by layout roster resolution.
type TrainLessee struct {
	TrainID    TrainID
	ToUserID   uint
	SpeedLimit uint8
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
