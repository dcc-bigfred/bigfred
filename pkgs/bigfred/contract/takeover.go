package contract

import "github.com/keskad/loco/pkgs/bigfred/server/domain"

// WS action type strings (§4.2).
const (
	TypeTakeoverRequest = "takeover.request"
	TypeTakeoverReject  = "takeover.reject"
	TypeTakeoverCancel  = "takeover.cancel"
	TypeTakeoverRelease = "takeover.release"
)

// WS event type strings (§4.2).
const (
	TypeTakeoverRequested = "takeover.requested"
	TypeTakeoverGranted   = "takeover.granted"
	TypeTakeoverReleased  = "takeover.released"
	TypeTakeoverRejected  = "takeover.rejected"
	TypeTakeoverCancelled = "takeover.cancelled"
)

// TakeoverUserWire identifies a user on takeover envelopes.
type TakeoverUserWire struct {
	UserID uint   `json:"userId"`
	Login  string `json:"login"`
}

// TakeoverRequestPayload is the client → server takeover.request body.
type TakeoverRequestPayload struct {
	Target   domain.TakeoverTarget `json:"target"`
	TargetID uint                  `json:"targetId"`
}

// TakeoverRequestIDPayload carries a request id on reject/cancel/release.
type TakeoverRequestIDPayload struct {
	RequestID uint `json:"requestId"`
}

// TakeoverRequestedWire is sent to the affected driver (§4.2).
type TakeoverRequestedWire struct {
	RequestID  uint                  `json:"requestId"`
	Signalman  TakeoverUserWire      `json:"signalman"`
	Target     domain.TakeoverTarget `json:"target"`
	TargetID   uint                  `json:"targetId"`
	AutoGrantAt int64                `json:"autoGrantAt"`
}

// TakeoverGrantedWire is sent when the 15 s window elapses (§4.2).
type TakeoverGrantedWire struct {
	RequestID       uint                  `json:"requestId"`
	Target          domain.TakeoverTarget `json:"target"`
	TargetID        uint                  `json:"targetId"`
	Signalman       TakeoverUserWire      `json:"signalman"`
	LeaseExpiresAt  int64                 `json:"leaseExpiresAt"`
}

// TakeoverReleasedWire is sent when the lease ends (§4.2).
type TakeoverReleasedWire struct {
	RequestID uint                  `json:"requestId"`
	Target    domain.TakeoverTarget `json:"target"`
	TargetID  uint                  `json:"targetId"`
	Reason    string                `json:"reason,omitempty"`
}

// TakeoverRejectedWire / TakeoverCancelledWire are terminal notifications.
type TakeoverRejectedWire struct {
	RequestID uint `json:"requestId"`
}

type TakeoverCancelledWire struct {
	RequestID uint `json:"requestId"`
}
