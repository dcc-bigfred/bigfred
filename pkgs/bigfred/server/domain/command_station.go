package domain

import "time"

// CommandStationKind is the closed catalogue of supported physical
// DCC command stations (§7e.2). The value drives which driver in
// `pkgs/loco/commandstation` the `dcc-bus` daemon constructs.
type CommandStationKind string

const (
	// CommandStationKindZ21 is the Roco/Fleischmann Z21 line over UDP.
	CommandStationKindZ21 CommandStationKind = "z21"
	// CommandStationKindLocoNetSerial connects to a LocoNet bridge over
	// a serial / USB port (e.g. Digitrax PR3, MS100).
	CommandStationKindLocoNetSerial CommandStationKind = "loconet_serial"
	// CommandStationKindLocoNetTCP connects to a LocoNet-over-TCP
	// gateway (e.g. JMRI, lbserver). The connection URI scheme selects the
	// wire format: `tcp://` for raw binary LocoNet over TCP (default),
	// `lbserver://` for the ASCII LoconetOverTcp/LbServer protocol.
	CommandStationKindLocoNetTCP CommandStationKind = "loconet_tcp"
)

// CommandStationKinds returns the closed catalogue in display order
// (Z21 first because it is the bring-up target).
func CommandStationKinds() []CommandStationKind {
	return []CommandStationKind{
		CommandStationKindZ21,
		CommandStationKindLocoNetSerial,
		CommandStationKindLocoNetTCP,
	}
}

// IsValid reports whether k is one of the catalogue entries. False
// for the zero value so an uninitialised struct can never sneak past
// validation.
func (k CommandStationKind) IsValid() bool {
	switch k {
	case CommandStationKindZ21, CommandStationKindLocoNetSerial, CommandStationKindLocoNetTCP:
		return true
	}
	return false
}

// IsLocoNet reports whether the kind uses LocoNet slot leasing.
func (k CommandStationKind) IsLocoNet() bool {
	return k == CommandStationKindLocoNetSerial || k == CommandStationKindLocoNetTCP
}

// CommandStation is one physical DCC command station the system can
// drive (§7e.1). One station may be attached to several layouts
// (modelers commonly share a single Z21 between rooms) — see
// `LayoutCommandStation` for the many-to-many join.
//
// `ConnectionURI` is a kind-specific URI parsed by the daemon at
// boot time:
//   - z21:            `udp://<host>:<port>`     (port defaults to 21105)
//   - loconet_serial: `serial://<device>:<baud>` (e.g. `serial:///dev/ttyUSB0:57600`;
//                     use `serial://autodetect:<baud>` to pick the first available port)
//   - loconet_tcp:    `tcp://<host>:<port>`       (raw binary LocoNet; default)
//                     `lbserver://<host>:<port>`  (ASCII LoconetOverTcp/LbServer)
//
// Keeping the URI as a plain string (rather than a JSON blob) keeps
// admin UX trivial — copy-paste from the Z21 sticker, save, done.
type CommandStation struct {
	ID            uint
	Name          string
	Kind          CommandStationKind
	ConnectionURI string `db:"connection_uri"`
	SpeedSteps    uint   `db:"speed_steps"`
	// HeartbeatSecs is the WS ping interval the dcc-bus daemon advertises to
	// clients for this station. Default 2 when zero.
	HeartbeatSecs float64 `db:"heartbeat_secs"`
	// DeadmanSecs is the client idle window after which dcc-bus applies an
	// emergency stop. Must exceed HeartbeatSecs. Default 6 when zero.
	DeadmanSecs   float64 `db:"deadman_secs"`
	// PollIntervalMs is the state-feed polling cadence for drivers without
	// push notifications. Zero selects the dcc-bus daemon default (750 ms).
	PollIntervalMs uint `db:"poll_interval_ms"`
	// Z21ServerEnabled turns on the inbound Z21 handset UDP server in
	// dcc-bus for layouts attached to this command station.
	Z21ServerEnabled bool `db:"z21_server_enabled"`
	// Z21IPStickiness keys handset sessions by client IP only so a UDP
	// port change on reconnect does not drop the paired session.
	Z21IPStickiness bool `db:"z21_ip_stickiness"`
	// Z21InboundPort is the UDP port for inbound Z21 handset connections
	// when Z21ServerEnabled is set. Zero selects the default (21105).
	Z21InboundPort uint `db:"z21_inbound_port"`
	// WithrottleServerEnabled turns on the inbound WiThrottle TCP server in
	// dcc-bus for layouts attached to this command station.
	WithrottleServerEnabled bool `db:"withrottle_server_enabled"`
	// WithrottleInboundPort is the TCP port for inbound WiThrottle connections.
	// Zero selects the default (12090).
	WithrottleInboundPort uint `db:"withrottle_inbound_port"`
	// WithrottlePairingAddr is the DCC address of the pairing sentinel loco.
	// Zero selects the default (3).
	WithrottlePairingAddr uint `db:"withrottle_pairing_addr"`
	// WithrottleHeartbeatSecs is the dead-man heartbeat window advertised to
	// WiThrottle clients. Zero selects the default (10).
	WithrottleHeartbeatSecs float64 `db:"withrottle_heartbeat_secs"`
	// MaxLoconetSlots caps how many LocoNet slots BigFred may lease (D14).
	MaxLoconetSlots uint `db:"max_loconet_slots"`
	// IdleTimeoutSecs is the remote-handset idle window before SweepIdle
	// releases the slot. Zero disables idle release.
	IdleTimeoutSecs uint `db:"idle_timeout_secs"`
	// BootStopEnabled sends speed 0 to every roster locomotive once when
	// dcc-bus starts (after the first non-empty allowed_vehicles snapshot).
	BootStopEnabled bool `db:"boot_stop_enabled"`
	// SingleVehicleControl stops the user's other moving vehicles (speed > 1)
	// when they select or drive a different vehicle on this command station.
	SingleVehicleControl bool `db:"single_vehicle_control"`
	CreatedAt            time.Time
	UpdatedAt       time.Time
}

// Table tells REL which physical table backs this struct.
func (CommandStation) Table() string { return "command_stations" }

const (
	DefaultCommandStationSpeedSteps    = 128
	DefaultCommandStationHeartbeatSecs = 2
	DefaultCommandStationDeadmanSecs   = 6
	DefaultCommandStationPollIntervalMs = 0
	DefaultZ21InboundPort             = 21105
	DefaultWithrottleInboundPort      = 12090
	DefaultWithrottlePairingAddr      = 3
	DefaultWithrottleHeartbeatSecs    = 10
	DefaultCommandStationMaxLoconetSlots = 80
	DefaultCommandStationIdleTimeoutSecs = 60
	MaxLocoNetPhysicalSlots              = 117
)

// EffectiveSpeedSteps returns the catalogue DCC speed-step count, applying
// the default when the stored value is zero.
func (cs CommandStation) EffectiveSpeedSteps() uint {
	if cs.SpeedSteps == 0 {
		return DefaultCommandStationSpeedSteps
	}
	return cs.SpeedSteps
}

// EffectiveHeartbeatSecs returns the catalogue heartbeat interval, applying
// the default when the stored value is zero (pre-migration rows).
func (cs CommandStation) EffectiveHeartbeatSecs() float64 {
	if cs.HeartbeatSecs <= 0 {
		return DefaultCommandStationHeartbeatSecs
	}
	return cs.HeartbeatSecs
}

// EffectiveDeadmanSecs returns the catalogue dead-man idle window, applying
// the default when the stored value is zero (pre-migration rows).
func (cs CommandStation) EffectiveDeadmanSecs() float64 {
	if cs.DeadmanSecs <= 0 {
		return DefaultCommandStationDeadmanSecs
	}
	return cs.DeadmanSecs
}

// EffectivePollIntervalMs returns the catalogue state-feed poll cadence.
// Zero means the dcc-bus daemon applies its built-in default.
func (cs CommandStation) EffectivePollIntervalMs() uint {
	return cs.PollIntervalMs
}

// EffectiveZ21InboundPort returns the inbound Z21 handset UDP port.
func (cs CommandStation) EffectiveZ21InboundPort() uint16 {
	if cs.Z21InboundPort == 0 {
		return DefaultZ21InboundPort
	}
	if cs.Z21InboundPort > 65535 {
		return DefaultZ21InboundPort
	}
	return uint16(cs.Z21InboundPort)
}

// EffectiveWithrottleInboundPort returns the inbound WiThrottle TCP port.
func (cs CommandStation) EffectiveWithrottleInboundPort() uint16 {
	if cs.WithrottleInboundPort == 0 {
		return DefaultWithrottleInboundPort
	}
	if cs.WithrottleInboundPort > 65535 {
		return DefaultWithrottleInboundPort
	}
	return uint16(cs.WithrottleInboundPort)
}

// EffectiveWithrottlePairingAddr returns the pairing sentinel DCC address.
func (cs CommandStation) EffectiveWithrottlePairingAddr() uint16 {
	if cs.WithrottlePairingAddr == 0 {
		return DefaultWithrottlePairingAddr
	}
	if cs.WithrottlePairingAddr > 65535 {
		return DefaultWithrottlePairingAddr
	}
	return uint16(cs.WithrottlePairingAddr)
}

// EffectiveWithrottleHeartbeatSecs returns the WiThrottle heartbeat window.
func (cs CommandStation) EffectiveWithrottleHeartbeatSecs() float64 {
	if cs.WithrottleHeartbeatSecs <= 0 {
		return DefaultWithrottleHeartbeatSecs
	}
	return cs.WithrottleHeartbeatSecs
}

// EffectiveMaxLoconetSlots returns the BigFred slot budget for LocoNet stations.
func (cs CommandStation) EffectiveMaxLoconetSlots() uint {
	if cs.MaxLoconetSlots == 0 {
		return DefaultCommandStationMaxLoconetSlots
	}
	return cs.MaxLoconetSlots
}

// LayoutCommandStation is the join row binding a CommandStation to a
// Layout. The pair (LayoutID, CommandStationID) is unique on the
// table; that pair is the daemon identity (`dcc-bus-<L>-<C>` in
// supervisord) — see §7e.2 for the lazy spawn rule.
type LayoutCommandStation struct {
	ID               uint
	LayoutID         uint      `db:"layout_id"`
	CommandStationID uint      `db:"command_station_id"`
	AddedByUserID    uint      `db:"added_by_user_id"`
	AddedAt          time.Time `db:"added_at"`
}

// Table tells REL which physical table backs this struct.
func (LayoutCommandStation) Table() string { return "layout_command_stations" }
