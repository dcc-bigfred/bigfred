// Package protocol carries the wire shapes that are exclusive to the
// dcc-bus WebSocket endpoint (§7e.4): subscribe requests, the welcome
// frame, acks and errors. The Go declarations are the source of truth
// for the dcc-bus side of the contract; `tygo` generates the matching
// TypeScript declarations consumed by the frontend.
//
// Control intents that ALSO travel server → daemon on the Redis command
// channel (§7e.3) — loco.setSpeed and loco.setFunction — live in
// pkgs/bigfred/contract (LocoSetSpeedWire, LocoSetFunctionWire) so both
// processes share one definition. Likewise the per-loco state snapshot
// (contract.LocoStateWire) and the envelope (contract.EnvelopeWire).
//
// The contract is intentionally tiny — every frame is either an
// authoritative state snapshot from the daemon or a control intent
// from the user. Anything richer (presence, takeover, scripts) stays
// on loco-server's control-plane WS as documented in §7e.4.
package protocol

import (
	"encoding/json"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// Frame is a small helper that re-wraps a strongly-typed payload
// into a contract.EnvelopeWire ready for json.Marshal. Used by handlers
// that build server-initiated events.
func Frame(eventType string, payload any) (contract.EnvelopeWire, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return contract.EnvelopeWire{}, err
	}
	return contract.EnvelopeWire{Type: eventType, Payload: raw}, nil
}

// FrameWithID is the counterpart of Frame for request/response
// pairs. The daemon's ack frames carry the same ID as the inbound
// request so the client can correlate.
func FrameWithID(eventType, id string, payload any) (contract.EnvelopeWire, error) {
	env, err := Frame(eventType, payload)
	if err != nil {
		return contract.EnvelopeWire{}, err
	}
	env.ID = id
	return env, nil
}

// -------- Client → Server frames --------

// PingPayload is sent by the client every Heartbeat interval to
// satisfy the per-daemon dead-man's switch (§7e.5). The body is
// empty on purpose — the act of receiving the frame is the signal.
type PingPayload struct{}

// LocoSubscribePayload tells the daemon to start pushing
// `loco.state` updates for the listed locomotive addresses. The
// daemon collapses multiple subscribe frames onto its internal
// subscription set; unsubscribe is implicit on WS close.
type LocoSubscribePayload struct {
	Addresses []uint16 `json:"addresses"`
}

// SystemEStopPayload is the data-plane emergency stop. Scope is
// always "the command station this daemon owns"; the loco-server
// emits the broader cross-cs broadcast as documented in §7e.4.
// `Reason` is a free-form audit hint ("button", "deadman",
// "takeover").
type SystemEStopPayload struct {
	Reason string `json:"reason,omitempty"`
}

// -------- Server → Client frames --------

// DccBusOpenedPayload is the welcome frame the daemon sends right
// after the upgrade handshake. It tells the client which (layout,
// command-station) it landed on, the daemon's view of subscription
// limits, and the DMS heartbeat interval the client MUST honour.
type DccBusOpenedPayload struct {
	LayoutID         uint    `json:"layoutId"`
	CommandStationID uint    `json:"commandStationId"`
	SpeedSteps       uint    `json:"speedSteps"`
	HeartbeatSecs    float64 `json:"heartbeatSecs"`
	// DeadmanSecs is the inactivity window after which the daemon
	// applies its emergency plan to every loco the client owns.
	// Reset by every inbound frame (ping, setSpeed, ...).
	DeadmanSecs float64 `json:"deadmanSecs"`
	// SessionID is the daemon-side handle for this client. It is
	// echoed back on every event so the frontend can correlate
	// multi-tab fan-out and audit log entries.
	SessionID string `json:"sessionId"`
}

// LocoErrorPayload reports a daemon-side rejection of a previously
// accepted client frame (e.g. a takeover invalidated mid-flight or
// the command station dropped). `Code` is machine-readable so the
// frontend can localise without parsing free text.
type LocoErrorPayload struct {
	Address uint16 `json:"address,omitempty"`
	Code    string `json:"code"`
	Detail  string `json:"detail,omitempty"`
}

// AckPayload is the standard reply to any client request that
// expected confirmation. `Ok` is true on success; `Error` is the
// machine-readable failure code when `Ok` is false.
type AckPayload struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// -------- Frame type catalogue --------

// Frame type catalogue. Kept as a const block so callers cannot
// drift from the wire spec by typo.
const (
	TypePing         = "ping"
	TypePong         = "pong"
	TypeLocoSubscribe   = "loco.subscribe"
	TypeLocoSetSpeed    = "loco.setSpeed"
	TypeLocoSetFunction = "loco.setFunction"
	TypeSystemEStop     = "system.estop"
	TypeSystemRadioStop = "system.radioStop"

	TypeDccBusOpened = "dcc-bus.opened"
	TypeLocoState    = "loco.state"
	TypeLocoError    = "loco.error"
	TypeAck          = "ack"
)
