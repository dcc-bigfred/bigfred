package contract

import "github.com/keskad/loco/pkgs/bigfred/server/domain"

// TypeSystemEStopTarget is the control-plane per-target emergency stop
// action (§4.2, §6.3d — „Zatrzymaj skład").
const TypeSystemEStopTarget = "system.estopTarget"

// EStopTargetPayload is the client → server estopTarget body.
type EStopTargetPayload struct {
	Target   domain.TakeoverTarget `json:"target"`
	TargetID string                  `json:"targetId"`
}

// EStopTargetCommandWire is the loco-server → daemon control command
// carrying the resolved DCC addresses of a single vehicle or train to
// brake. It travels on DccBusCommandChannel under the
// protocol.TypeSystemEStopTarget envelope type.
type EStopTargetCommandWire struct {
	Addresses []uint16 `json:"addresses"`
}
