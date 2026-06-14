package contract

import (
	"encoding/json"
	"fmt"
)

// Layout-scoped pub/sub channels (loco-server → every dcc-bus on the layout).

const (
	// LayoutRadioStopChannelTmpl is published when a user triggers Radio
	// Stop (§4.6). Every dcc-bus daemon pinned to the layout subscribes
	// and runs its local roster halt + per-user dead-man plan.
	LayoutRadioStopChannelTmpl = "bigfred:layout:%d:radio_stop"
)

// LayoutRadioStopChannel is the pub/sub channel for layout-wide Radio Stop.
func LayoutRadioStopChannel(layoutID uint) string {
	return fmt.Sprintf(LayoutRadioStopChannelTmpl, layoutID)
}

// RadioStopCommandWire is the payload PUBLISHed on LayoutRadioStopChannel
// when loco-server orchestrates a halt.
type RadioStopCommandWire struct {
	TriggeredByUserID uint   `json:"triggeredByUserId"`
	TriggeredByLogin  string `json:"triggeredByLogin,omitempty"`
	At                int64  `json:"at"`
}

// RadioStopAckWire is emitted on dcc-bus:evt after a daemon finishes the
// roster halt on its command station.
type RadioStopAckWire struct {
	CommandStationID uint     `json:"commandStationId"`
	Addrs            []uint16 `json:"addrs"`
	At               int64    `json:"at"`
}

// RadioStopPushWire is the control-plane fan-out after the halt is issued.
type RadioStopPushWire struct {
	TriggeredBy struct {
		UserID uint   `json:"userId"`
		Login  string `json:"login"`
	} `json:"triggeredBy"`
	At int64 `json:"at"`
}

// BuildRadioStopCommandPayload marshals the layout radio_stop command.
func BuildRadioStopCommandPayload(cmd RadioStopCommandWire) ([]byte, error) {
	return json.Marshal(cmd)
}
