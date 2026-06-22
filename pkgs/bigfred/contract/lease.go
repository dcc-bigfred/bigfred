package contract

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	VehicleLeaseKeyTmpl         = "bigfred:lease:vehicle:%s"
	TrainLeaseKeyTmpl           = "bigfred:lease:train:%s"
	VehicleLeaseKeyScanPattern  = "bigfred:lease:vehicle:*"
	TrainLeaseKeyScanPattern    = "bigfred:lease:train:*"
	VehicleLeaseByOwnerKeyTmpl  = "bigfred:lease:byOwner:vehicle:%d"
	VehicleLeaseByLesseeKeyTmpl = "bigfred:lease:byLessee:vehicle:%d"
	TrainLeaseByOwnerKeyTmpl    = "bigfred:lease:byOwner:train:%d"
	TrainLeaseByLesseeKeyTmpl   = "bigfred:lease:byLessee:train:%d"
)

// VehicleLeaseKey returns the Redis key for an active vehicle lease.
func VehicleLeaseKey(vehicleID string) string {
	return fmt.Sprintf(VehicleLeaseKeyTmpl, vehicleID)
}

// TrainLeaseKey returns the Redis key for an active train lease.
func TrainLeaseKey(trainID string) string {
	return fmt.Sprintf(TrainLeaseKeyTmpl, trainID)
}

// VehicleLeaseByOwnerKey indexes vehicle lease keys granted by a user.
func VehicleLeaseByOwnerKey(ownerUserID uint) string {
	return fmt.Sprintf(VehicleLeaseByOwnerKeyTmpl, ownerUserID)
}

// VehicleLeaseByLesseeKey indexes vehicle lease keys received by a user.
func VehicleLeaseByLesseeKey(lesseeUserID uint) string {
	return fmt.Sprintf(VehicleLeaseByLesseeKeyTmpl, lesseeUserID)
}

// TrainLeaseByOwnerKey indexes train lease keys granted by a user.
func TrainLeaseByOwnerKey(ownerUserID uint) string {
	return fmt.Sprintf(TrainLeaseByOwnerKeyTmpl, ownerUserID)
}

// TrainLeaseByLesseeKey indexes train lease keys received by a user.
func TrainLeaseByLesseeKey(lesseeUserID uint) string {
	return fmt.Sprintf(TrainLeaseByLesseeKeyTmpl, lesseeUserID)
}

// LeaseWire is the JSON payload stored at vehicle/train lease keys.
type LeaseWire struct {
	FromUserID uint      `json:"fromUserId"`
	ToUserID   uint      `json:"toUserId"`
	SpeedLimit uint8     `json:"speedLimit,omitempty"`
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
