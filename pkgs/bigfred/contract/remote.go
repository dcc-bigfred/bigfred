package contract

// Remote control wire types and events (protocol-agnostic names).
// Redis key templates and Z21 pairing types remain in z21pairing.go.

const (
	// RemoteClientsChangedEvent is published when an inbound handset registry changes.
	RemoteClientsChangedEvent = "remote.clients.changed"
	// RemoteProtocolZ21 identifies the Roco Z21 LAN adapter.
	RemoteProtocolZ21 = "z21"
)

// RemoteClientWire describes one inbound handset participant.
type RemoteClientWire = Z21ClientWire

// RemoteClientsSnapshotWire is stored at Z21ClientsSnapshotKey and pushed on
// remote.clients.changed events.
type RemoteClientsSnapshotWire = Z21ClientsSnapshotWire

// MarshalRemoteClientsSnapshot encodes a clients snapshot for Redis SET.
func MarshalRemoteClientsSnapshot(w RemoteClientsSnapshotWire) ([]byte, error) {
	return MarshalZ21ClientsSnapshot(w)
}

// UnmarshalRemoteClientsSnapshot decodes a clients snapshot from Redis GET.
func UnmarshalRemoteClientsSnapshot(raw []byte) (RemoteClientsSnapshotWire, error) {
	return UnmarshalZ21ClientsSnapshot(raw)
}
