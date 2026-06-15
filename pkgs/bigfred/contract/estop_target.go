package contract

import "github.com/keskad/loco/pkgs/bigfred/server/domain"

// TypeSystemEStopTarget is the control-plane per-target emergency stop
// action (§4.2, §6.3d — „Zatrzymaj skład").
const TypeSystemEStopTarget = "system.estopTarget"

// EStopTargetPayload is the client → server estopTarget body.
type EStopTargetPayload struct {
	Target   domain.TakeoverTarget `json:"target"`
	TargetID uint                  `json:"targetId"`
}
