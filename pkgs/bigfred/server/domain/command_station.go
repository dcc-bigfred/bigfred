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

// CommandStation is one physical DCC command station the system can
// drive (§7e.1). One station may be attached to several layouts
// (modelers commonly share a single Z21 between rooms) — see
// `LayoutCommandStation` for the many-to-many join.
//
// `ConnectionURI` is a kind-specific URI parsed by the daemon at
// boot time:
//   - z21:            `udp://<host>:<port>`     (port defaults to 21105)
//   - loconet_serial: `serial://<device>:<baud>` (e.g. `serial:///dev/ttyUSB0:57600`)
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
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Table tells REL which physical table backs this struct.
func (CommandStation) Table() string { return "command_stations" }

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
