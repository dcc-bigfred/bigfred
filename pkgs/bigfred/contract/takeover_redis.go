package contract

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

const (
	TakeoverNextIDKeyTmpl       = "bigfred:takeover:next_id"
	TakeoverRequestKeyTmpl      = "bigfred:takeover:req:%d"
	TakeoverPendingTargetKeyTmpl = "bigfred:takeover:pending:%s:%s"
	TakeoverPendingSetKey       = "bigfred:takeover:pending_ids"
	TakeoverSignalmanGrantedTmpl = "bigfred:takeover:signalman:%d:granted"
)

// TakeoverNextIDKey is the Redis counter for monotonic request ids.
func TakeoverNextIDKey() string { return TakeoverNextIDKeyTmpl }

// TakeoverRequestKey returns the Redis key for one active request row.
func TakeoverRequestKey(id uint) string {
	return fmt.Sprintf(TakeoverRequestKeyTmpl, id)
}

// TakeoverPendingTargetKey maps a roster target to a pending request id.
func TakeoverPendingTargetKey(target domain.TakeoverTarget, targetID string) string {
	return fmt.Sprintf(TakeoverPendingTargetKeyTmpl, string(target), targetID)
}

// TakeoverSignalmanGrantedKey indexes granted requests for one signalman.
func TakeoverSignalmanGrantedKey(signalmanID uint) string {
	return fmt.Sprintf(TakeoverSignalmanGrantedTmpl, signalmanID)
}

// TakeoverRequestWire is the JSON document at TakeoverRequestKey.
type TakeoverRequestWire struct {
	ID              uint                  `json:"id"`
	LayoutID        uint                  `json:"layoutId"`
	InterlockingID  uint                  `json:"interlockingId"`
	SignalmanUserID uint                  `json:"signalmanUserId"`
	DriverUserID    uint                  `json:"driverUserId"`
	Target          domain.TakeoverTarget `json:"target"`
	TargetID        string                `json:"targetId"`
	RequestedAt     time.Time             `json:"requestedAt"`
	DecisionAt      *time.Time            `json:"decisionAt,omitempty"`
	AutoGrantAt     time.Time             `json:"autoGrantAt"`
	GrantedLeaseID  *uint                 `json:"grantedLeaseId,omitempty"`
	ReleasedAt      *time.Time            `json:"releasedAt,omitempty"`
	State           domain.TakeoverState  `json:"state"`
}

// MarshalTakeoverRequest encodes a takeover request for Redis SET.
func MarshalTakeoverRequest(w TakeoverRequestWire) ([]byte, error) {
	return json.Marshal(w)
}

// UnmarshalTakeoverRequest decodes a takeover request from Redis GET.
func UnmarshalTakeoverRequest(raw []byte) (TakeoverRequestWire, error) {
	var w TakeoverRequestWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return TakeoverRequestWire{}, err
	}
	return w, nil
}

// TakeoverRequestFromWire maps wire → domain.
func TakeoverRequestFromWire(w TakeoverRequestWire) domain.TakeoverRequest {
	return domain.TakeoverRequest{
		ID:              w.ID,
		LayoutID:        w.LayoutID,
		InterlockingID:  w.InterlockingID,
		SignalmanUserID: w.SignalmanUserID,
		DriverUserID:    w.DriverUserID,
		Target:          w.Target,
		TargetID:        w.TargetID,
		RequestedAt:     w.RequestedAt,
		DecisionAt:      w.DecisionAt,
		AutoGrantAt:     w.AutoGrantAt,
		GrantedLeaseID:  w.GrantedLeaseID,
		ReleasedAt:      w.ReleasedAt,
		State:           w.State,
	}
}

// TakeoverRequestToWire maps domain → wire.
func TakeoverRequestToWire(row domain.TakeoverRequest) TakeoverRequestWire {
	return TakeoverRequestWire{
		ID:              row.ID,
		LayoutID:        row.LayoutID,
		InterlockingID:  row.InterlockingID,
		SignalmanUserID: row.SignalmanUserID,
		DriverUserID:    row.DriverUserID,
		Target:          row.Target,
		TargetID:        row.TargetID,
		RequestedAt:     row.RequestedAt,
		DecisionAt:      row.DecisionAt,
		AutoGrantAt:     row.AutoGrantAt,
		GrantedLeaseID:  row.GrantedLeaseID,
		ReleasedAt:      row.ReleasedAt,
		State:           row.State,
	}
}
