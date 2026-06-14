package contract

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// dcc-bus coordination: command/event pub/sub channels, port-pool hash, and
// the envelope wire shape shared by those channels and the dcc-bus WebSocket.

const (
	// DccBusEventChannelTmpl is the pub/sub channel a dcc-bus daemon
	// publishes onto and loco-server consumes from. Verbs: layoutID,
	// commandStationID.
	DccBusEventChannelTmpl = "dcc-bus:evt:%d:%d"

	// DccBusCommandChannelTmpl is the inverse of the event channel:
	// loco-server publishes, the daemon subscribes (train.setSpeed
	// fan-out, cross-process estop). Verbs: layoutID, commandStationID.
	DccBusCommandChannelTmpl = "dcc-bus:cmd:%d:%d"

	// DccBusEventChannelPattern is the PSUBSCRIBE glob loco-server uses
	// to fan in events from every daemon at once.
	DccBusEventChannelPattern = "dcc-bus:evt:*"

	// DccBusEventChannelPrefix is the fixed prefix of an event channel,
	// used when parsing <layoutId>:<csId> back out of a channel name.
	DccBusEventChannelPrefix = "dcc-bus:evt:"

	// DccBusPortsKey is the Redis HASH holding allocated dcc-bus port
	// assignments. Each field is keyed by DccBusPortsFieldTmpl.
	DccBusPortsKey = "dcc-bus:ports"

	// DccBusPortsFieldTmpl is the field name within DccBusPortsKey for
	// one (layout, command-station) pair. Verbs: layoutID,
	// commandStationID.
	DccBusPortsFieldTmpl = "%d:%d"
)

// DccBusEventChannel is the daemon → server pub/sub channel.
func DccBusEventChannel(layoutID, commandStationID uint) string {
	return fmt.Sprintf(DccBusEventChannelTmpl, layoutID, commandStationID)
}

// DccBusCommandChannel is the server → daemon pub/sub channel.
func DccBusCommandChannel(layoutID, commandStationID uint) string {
	return fmt.Sprintf(DccBusCommandChannelTmpl, layoutID, commandStationID)
}

// DccBusPortsField is the field name within DccBusPortsKey for one
// (layout, command-station) pair.
func DccBusPortsField(layoutID, commandStationID uint) string {
	return fmt.Sprintf(DccBusPortsFieldTmpl, layoutID, commandStationID)
}

// EnvelopeWire is the common pub/sub frame on the event and command
// channels (and the dcc-bus WebSocket): a type tag, an optional correlation
// id the client may set so the daemon can echo it on the matching ack, and
// an opaque already-encoded inner payload.
type EnvelopeWire struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func marshalEnvelope(eventType, id string, payload []byte) ([]byte, error) {
	env := EnvelopeWire{Type: eventType, ID: id}
	if len(payload) > 0 {
		env.Payload = json.RawMessage(payload)
	}
	return json.Marshal(env)
}

// BuildEventPayload wraps an already-encoded inner payload into the
// envelope PUBLISHed on DccBusEventChannel (daemon → server). id is empty
// for server-unsolicited events. The inner payload is typically the output
// of BuildLocoStatePayload or a marshaled audit object.
func BuildEventPayload(eventType, id string, payload []byte) ([]byte, error) {
	return marshalEnvelope(eventType, id, payload)
}

// BuildCommandPayload wraps an already-encoded inner payload into the
// envelope PUBLISHed on DccBusCommandChannel (server → daemon), e.g. a
// train.setSpeed fan-out or a cross-process estop.
func BuildCommandPayload(eventType, id string, payload []byte) ([]byte, error) {
	return marshalEnvelope(eventType, id, payload)
}

// BuildPortValue renders the value stored in the DccBusPortsKey hash under
// DccBusPortsField(...). Ports are persisted as their base-10 string.
func BuildPortValue(port uint16) string {
	return strconv.Itoa(int(port))
}
