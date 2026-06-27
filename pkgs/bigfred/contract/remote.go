package contract

import "encoding/json"

// Remote control wire types and events (protocol-agnostic names).

const (
	// RemoteClientsChangedEvent is published when an inbound handset registry changes.
	RemoteClientsChangedEvent = "remote.clients.changed"
	// RemoteProtocolZ21 identifies the Roco Z21 LAN adapter.
	RemoteProtocolZ21 = "z21"
)

// RemoteClientWire describes one inbound handset participant.
type RemoteClientWire struct {
	ClientKey        string `json:"clientKey"`
	Protocol         string `json:"protocol,omitempty"`
	IP               string `json:"ip"`
	Port             int    `json:"port"`
	Paired           bool   `json:"paired"`
	UserID           uint   `json:"userId,omitempty"`
	UserLogin        string `json:"userLogin,omitempty"`
	LastSeenAt       int64  `json:"lastSeenAt"`
	ConnectedAt      int64  `json:"connectedAt"`
	SessionExpiresAt int64  `json:"sessionExpiresAt,omitempty"`
	IdleBraked       bool   `json:"idleBraked"`
}

// RemoteClientsSnapshotWire is stored at RemoteClientsSnapshotKey and pushed on
// remote.clients.changed events.
type RemoteClientsSnapshotWire struct {
	LayoutID         uint               `json:"layoutId"`
	CommandStationID uint               `json:"commandStationId"`
	IPStickiness     bool               `json:"ipStickiness,omitempty"`
	UpdatedAt        int64              `json:"updatedAt"`
	Clients          []RemoteClientWire `json:"clients"`
}

// Z21 wire aliases kept for gradual migration within protocol adapters.
type (
	Z21PairingReqWire    = RemotePendingWire
	Z21PairingActiveWire = RemoteSessionWire
	Z21ClientWire        = RemoteClientWire
	Z21ClientsSnapshotWire = RemoteClientsSnapshotWire
)

// MarshalRemoteClientsSnapshot encodes a clients snapshot for Redis SET.
func MarshalRemoteClientsSnapshot(w RemoteClientsSnapshotWire) ([]byte, error) {
	return json.Marshal(w)
}

// UnmarshalRemoteClientsSnapshot decodes a clients snapshot from Redis GET.
func UnmarshalRemoteClientsSnapshot(raw []byte) (RemoteClientsSnapshotWire, error) {
	var w RemoteClientsSnapshotWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return RemoteClientsSnapshotWire{}, err
	}
	return w, nil
}

// MarshalZ21PairingReq encodes a pending pairing request for Redis SET.
func MarshalZ21PairingReq(w Z21PairingReqWire) ([]byte, error) {
	return MarshalRemotePending(w)
}

// UnmarshalZ21PairingReq decodes a pending pairing request from Redis GET.
func UnmarshalZ21PairingReq(raw []byte) (Z21PairingReqWire, error) {
	return UnmarshalRemotePending(raw)
}

// MarshalZ21PairingActive encodes an active handset session for Redis SET.
func MarshalZ21PairingActive(w Z21PairingActiveWire) ([]byte, error) {
	return MarshalRemoteSession(w)
}

// UnmarshalZ21PairingActive decodes an active handset session from Redis GET.
func UnmarshalZ21PairingActive(raw []byte) (Z21PairingActiveWire, error) {
	return UnmarshalRemoteSession(raw)
}
