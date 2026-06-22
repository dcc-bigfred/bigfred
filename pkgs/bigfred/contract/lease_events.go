package contract

import "github.com/keskad/loco/pkgs/bigfred/server/domain"

// WS lease event type strings.
const (
	TypeLeaseCreated = "lease.created"
	TypeLeaseUpdated = "lease.updated"
	TypeLeaseRevoked = "lease.revoked"
	TypeLeaseExpired = "lease.expired"
)

// LeaseEventWire is broadcast to owner and lessee on lease lifecycle changes.
type LeaseEventWire struct {
	Kind       domain.TakeoverTarget `json:"kind"`
	TargetID   string                `json:"targetId"`
	TargetName string                `json:"targetName"`
	FromUserID uint                  `json:"fromUserId"`
	FromLogin  string                `json:"fromLogin"`
	ToUserID   uint                  `json:"toUserId"`
	ToLogin    string                `json:"toLogin"`
	ExpiresAt  int64                 `json:"expiresAt"` // unix ms UTC
	SpeedLimit uint8                 `json:"speedLimit"`
}
