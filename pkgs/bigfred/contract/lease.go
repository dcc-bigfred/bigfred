package contract

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	VehicleLeaseKeyTmpl = "bigfred:lease:vehicle:%d"
	TrainLeaseKeyTmpl   = "bigfred:lease:train:%d"
)

// VehicleLeaseKey returns the Redis key for an active vehicle lease.
func VehicleLeaseKey(vehicleID uint) string {
	return fmt.Sprintf(VehicleLeaseKeyTmpl, vehicleID)
}

// TrainLeaseKey returns the Redis key for an active train lease.
func TrainLeaseKey(trainID uint) string {
	return fmt.Sprintf(TrainLeaseKeyTmpl, trainID)
}

// LeaseWire is the JSON payload stored at vehicle/train lease keys.
type LeaseWire struct {
	FromUserID uint      `json:"fromUserId"`
	ToUserID   uint      `json:"toUserId"`
	StartedAt  time.Time `json:"startedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Source     string    `json:"source,omitempty"`
}

// MarshalLease encodes a lease for Redis SET.
func MarshalLease(w LeaseWire) ([]byte, error) {
	return json.Marshal(w)
}

// UnmarshalLease decodes a lease from Redis GET.
func UnmarshalLease(raw []byte) (LeaseWire, error) {
	var w LeaseWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return LeaseWire{}, err
	}
	return w, nil
}
