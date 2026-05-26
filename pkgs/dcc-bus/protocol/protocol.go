// Package protocol carries the wire shapes exchanged on the dcc-bus
// WebSocket endpoint (§7e.4) and on the Redis pub/sub channels
// (§7e.3). The Go declarations are the source of truth for the
// dcc-bus side of the contract; `tygo` generates the matching
// TypeScript declarations consumed by the frontend.
//
// The contract is intentionally tiny — every frame is either an
// authoritative state snapshot from the daemon or a control intent
// from the user. Anything richer (presence, takeover, scripts) stays
// on loco-server's control-plane WS as documented in §7e.4.
package protocol

import "encoding/json"

// Envelope is the common wire shape for every WebSocket frame on the
// dcc-bus endpoint. `ID` is an opaque correlation token that the
// client may set on requests so the daemon can echo it back on the
// matching `ack`; it stays empty on server-initiated events.
type Envelope struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Frame is a small helper that re-wraps a strongly-typed payload
// into an Envelope ready for json.Marshal. Used by handlers that
// build server-initiated events.
func Frame(eventType string, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Type: eventType, Payload: raw}, nil
}

// FrameWithID is the counterpart of Frame for request/response
// pairs. The daemon's ack frames carry the same ID as the inbound
// request so the client can correlate.
func FrameWithID(eventType, id string, payload any) (Envelope, error) {
	env, err := Frame(eventType, payload)
	if err != nil {
		return Envelope{}, err
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

// LocoSetSpeedPayload carries one throttle move. `Speed` is 0-127
// (128-step mode) or 0-28 (28-step) — the daemon translates to the
// command-station's wire format. `Forward` is the direction bit.
// `Emergency` triggers an EMG stop regardless of Speed when true.
type LocoSetSpeedPayload struct {
	Address   uint16 `json:"address"`
	Speed     uint8  `json:"speed"`
	Forward   bool   `json:"forward"`
	Emergency bool   `json:"emergency,omitempty"`
}

// LocoSetFunctionPayload toggles a single locomotive function (F0..
// F28). The daemon coalesces consecutive toggles of the same Fn
// within `coalesceWindow` to a single DCC packet so a UI bounce
// doesn't flood the bus.
type LocoSetFunctionPayload struct {
	Address  uint16 `json:"address"`
	Function uint8  `json:"function"`
	On       bool   `json:"on"`
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

// LocoStatePayload is the authoritative state snapshot for one
// locomotive. The daemon publishes one frame per loco every time
// it observes a change — either from its own poller or from an
// external throttle / Goja script.
type LocoStatePayload struct {
	Address   uint16 `json:"address"`
	Speed     uint8  `json:"speed"`
	Forward   bool   `json:"forward"`
	Functions []bool `json:"functions"`
	// ControlledByUserID is 0 when nobody is driving this loco; on
	// a takeover it points at the user whose throttle now owns it.
	ControlledByUserID uint `json:"controlledByUserId,omitempty"`
	// Source identifies who triggered the change: "throttle" |
	// "script" | "external" | "poller". Used by the frontend to
	// suppress its own optimistic update echo.
	Source string `json:"source,omitempty"`
	// At is the unix millis stamp set by the daemon at observation
	// time. Used by the frontend's "stale" indicator (§6.3b).
	At int64 `json:"at"`
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

	TypeDccBusOpened = "dcc-bus.opened"
	TypeLocoState    = "loco.state"
	TypeLocoError    = "loco.error"
	TypeAck          = "ack"
)
