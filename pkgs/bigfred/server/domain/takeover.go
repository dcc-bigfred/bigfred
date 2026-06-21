package domain

import "time"

// TakeoverTarget identifies what the signalman wants to drive.
type TakeoverTarget string

const (
	TakeoverTargetVehicle TakeoverTarget = "vehicle"
	TakeoverTargetTrain   TakeoverTarget = "train"
)

// TakeoverState is the persisted lifecycle of a takeover request.
type TakeoverState string

const (
	TakeoverStatePending   TakeoverState = "pending"
	TakeoverStateGranted   TakeoverState = "granted"
	TakeoverStateRejected  TakeoverState = "rejected"
	TakeoverStateCancelled TakeoverState = "cancelled"
	TakeoverStateReleased  TakeoverState = "released"
)

// TakeoverWindow is the driver's reject window; TakeoverLeaseDuration is
// how long the signalman holds the target once granted (§4.3).
const (
	TakeoverWindow        = 15 * time.Second
	TakeoverLeaseDuration = 5 * time.Minute
)

// TakeoverRequest is persisted in takeover_requests for auditing.
type TakeoverRequest struct {
	ID              uint           `db:"id"`
	LayoutID        uint           `db:"layout_id"`
	InterlockingID  uint           `db:"interlocking_id"`
	SignalmanUserID uint           `db:"signalman_user_id"`
	DriverUserID    uint           `db:"driver_user_id"`
	Target          TakeoverTarget `db:"target"`
	TargetID        string         `db:"target_id"`
	RequestedAt     time.Time      `db:"requested_at"`
	DecisionAt      *time.Time     `db:"decision_at"`
	AutoGrantAt     time.Time      `db:"auto_grant_at"`
	GrantedLeaseID  *uint          `db:"granted_lease_id"`
	ReleasedAt      *time.Time     `db:"released_at"`
	State           TakeoverState  `db:"state"`
}

// IsPending reports whether the driver may still reject.
func (r TakeoverRequest) IsPending(now time.Time) bool {
	return r.State == TakeoverStatePending && now.Before(r.AutoGrantAt)
}

// IsGranted reports whether the signalman currently holds the target.
func (r TakeoverRequest) IsGranted() bool {
	return r.State == TakeoverStateGranted
}
