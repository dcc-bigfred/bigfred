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
// satisfy the per-daemon dead-man's switch (§7e.5). The act of
// receiving the frame is the signal; LastPingLatencyMs optionally
// piggybacks the previous ping/pong RTT measured in the browser so
// the daemon can export client-visible latency without browser OTLP.
type PingPayload struct {
	LastPingLatencyMs float64 `json:"lastPingLatencyMs,omitempty"`
}

// LocoSubscribePayload tells the daemon to start pushing
// `loco.state` updates for the listed locomotive addresses. The
// daemon collapses multiple subscribe frames onto its internal
// subscription set; unsubscribe is implicit on WS close.
type LocoSubscribePayload struct {
	Addresses []uint16 `json:"addresses"`
}

// LocoSelectPayload marks one address as the session's active drive
// target and leases its command-station slot (LocoNet only).
type LocoSelectPayload struct {
	Address uint16 `json:"address"`
}

// LocoStealSlotPayload requests an explicit takeover of a LocoNet slot
// already IN_USE by another throttle (e.g. physical FRED).
type LocoStealSlotPayload struct {
	Address uint16 `json:"address"`
}

// LocoDeselectPayload clears the active drive target and releases the
// slot when this session was the last driver.
type LocoDeselectPayload struct {
	Address uint16 `json:"address"`
}

// TrainSelectPayload leases slots for every powered member of a train.
type TrainSelectPayload struct {
	TrainID string `json:"trainId"`
}

// SystemEStopPayload is the data-plane emergency stop. Scope is
// always "the command station this daemon owns"; the loco-server
// emits the broader cross-cs broadcast as documented in §7e.4.
// `Reason` is a free-form audit hint ("button", "deadman",
// "takeover").
type SystemEStopPayload struct {
	Reason string `json:"reason,omitempty"`
}

// CVEntry is one configuration-variable slot: the CV number and its
// value. Used both as a write instruction and as a read result.
type CVEntry struct {
	CV    uint16 `json:"cv"`
	Value uint8  `json:"value"`
}

// LocoCVWritePayload programs one or more CVs on a decoder. `Mode`
// selects the programming track ("prog") or programming-on-main
// ("pom"); empty falls back to the daemon's --default-programming-track.
type LocoCVWritePayload struct {
	Address uint16    `json:"address"`
	CVs     []CVEntry `json:"cvs"`
	Mode    string    `json:"mode,omitempty"` // "pom"|"prog"
}

// LocoCVReadPayload reads the listed CVs back from a decoder. POM
// reads need a RailCom-capable command station; programming-track
// reads do not.
type LocoCVReadPayload struct {
	Address uint16   `json:"address"`
	CVs     []uint16 `json:"cvs"`
	Mode    string   `json:"mode,omitempty"`
}

// LocoAddrSetPayload rewrites a decoder's DCC address. The daemon
// derives the CV1 / CV17 / CV18 / CV29 writes from the requested
// address, preserving every CV29 bit other than the long-address bit.
type LocoAddrSetPayload struct {
	Address uint16 `json:"address"`
	Mode    string `json:"mode,omitempty"`
	Verify  bool   `json:"verify,omitempty"`
}

// LocoAddrGetPayload reads a decoder's currently programmed address.
// `Address` is only meaningful in "pom" mode, where it addresses the
// decoder being interrogated.
type LocoAddrGetPayload struct {
	Address uint16 `json:"address,omitempty"`
	Mode    string `json:"mode,omitempty"`
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
	Address     uint16   `json:"address,omitempty"`
	Code        string   `json:"code"`
	Detail      string   `json:"detail,omitempty"`
	DrivenAddrs []uint16 `json:"drivenAddrs,omitempty"`
}

// ControlProgrammingRejectedPayload is published on the dcc-bus event
// channel when a programming command (loco.cvWrite / cvRead / addrSet /
// addrGet) arriving on the Redis control channel is rejected by the
// router. The control channel is fire-and-forget, so this event is the
// server's only signal that the command did not run — loco-server can
// log it, surface it to an admin HUD, or retry against a different
// station.
type ControlProgrammingRejectedPayload struct {
	// FrameType is the original control frame type (loco.cvWrite, …).
	FrameType string `json:"frameType"`
	// Code is the machine-readable rejection code
	// (errors.CodeProgrammingDisabled / CodeProgrammingFailed / …).
	Code string `json:"code"`
	// Address is the decoder address the command targeted, when known.
	Address uint16 `json:"address,omitempty"`
}

// TrainSetSpeedMemberAck is one member result inside a train.setSpeed ack.
type TrainSetSpeedMemberAck struct {
	Addr  uint16 `json:"addr"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// AckPayload is the standard reply to any client request that
// expected confirmation. `Ok` is true on success; `Error` is the
// machine-readable failure code when `Ok` is false.
type AckPayload struct {
	OK          bool                     `json:"ok"`
	Error       string                   `json:"error,omitempty"`
	Members     []TrainSetSpeedMemberAck `json:"members,omitempty"`
	EvictedAddr uint16                   `json:"evictedAddr,omitempty"`
	DrivenAddrs []uint16                 `json:"drivenAddrs,omitempty"`
	// CVs carries the CVs read back (loco.cvRead) or the CVs the
	// daemon actually wrote (loco.cvWrite, loco.addrSet).
	CVs []CVEntry `json:"cvs,omitempty"`
	// LocoAddress and LongAddress report the decoder address decoded
	// from CV1/CV17/CV18/CV29 on loco.addrGet and loco.addrSet.
	LocoAddress uint16 `json:"locoAddress,omitempty"`
	LongAddress bool   `json:"longAddress,omitempty"`
}

// -------- Frame type catalogue --------

// Frame type catalogue. Kept as a const block so callers cannot
// drift from the wire spec by typo.
const (
	TypePing              = "ping"
	TypePong              = "pong"
	TypeLocoSubscribe     = "loco.subscribe"
	TypeLocoSelect        = "loco.select"
	TypeLocoDeselect      = "loco.deselect"
	TypeLocoStealSlot     = "loco.stealSlot"
	TypeTrainSelect       = "train.select"
	TypeLocoSetSpeed      = "loco.setSpeed"
	TypeTrainSetSpeed     = "train.setSpeed"
	TypeLocoSetFunction   = "loco.setFunction"
	TypeSystemEStop       = "system.estop"
	TypeSystemRadioStop   = "system.radioStop"
	TypeSystemEStopTarget = "system.estopTarget"
	TypeLocoCVWrite       = "loco.cvWrite"
	TypeLocoCVRead        = "loco.cvRead"
	TypeLocoAddrSet       = "loco.addrSet"
	TypeLocoAddrGet       = "loco.addrGet"

	// TypeControlProgrammingRejected is published on the dcc-bus event
	// channel when a programming command arriving on the Redis control
	// channel (dcc-bus:cmd) is rejected by the router (e.g. programming
	// disabled, no command station, decoder error). The control channel
	// is fire-and-forget, so this event is the only feedback the server
	// gets that the command did not run.
	TypeControlProgrammingRejected = "control.programming.rejected"

	TypeDccBusOpened = "dcc-bus.opened"
	TypeLocoState    = "loco.state"
	TypeLocoError    = "loco.error"
	TypeAck          = "ack"
)

// Programming track selectors accepted in the `mode` field of the
// CV / address frames. They mirror commandstation.Mode on the wire:
// POM programs a decoder on the main track while it is running,
// Prog uses the command station's isolated programming output.
const (
	ProgrammingModePOM  = "pom"
	ProgrammingModeProg = "prog"
)
