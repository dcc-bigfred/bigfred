package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

const (
	// RemotePairingReqTTL is how long a pending handset pairing code stays valid.
	RemotePairingReqTTL = 5 * time.Minute

	RemotePairingReqKeyTmpl       = "bigfred:remote:req:%d:%d:%s"
	RemotePairingActiveKeyTmpl    = "bigfred:remote:active:%d:%d:%s"
	RemotePairingByUserKeyTmpl    = "bigfred:remote:byuser:%d:%d:%d"
	RemotePairingReqByUserKeyTmpl = "bigfred:remote:reqbyuser:%d:%d:%d"
	RemotePairingReqDedupKeyTmpl  = "bigfred:remote:reqdedup:%d:%d:%s"
	RemoteClientsSnapshotKeyTmpl  = "bigfred:remote:clients:%d:%d"

	// RemoteStickySessionIdle is how long a paired handset session stays
	// in Redis without activity (last-seen idle window). TouchSeen refreshes
	// this TTL so a driving throttle keeps its pairing for days.
	RemoteStickySessionIdle = 72 * time.Hour
)

// RemotePairingReqKey is the Redis STRING key for one pending handset pair.
// reqID is protocol-specific (e.g. "z21:122:145").
func RemotePairingReqKey(layoutID, commandStationID uint, reqID string) string {
	return fmt.Sprintf(RemotePairingReqKeyTmpl, layoutID, commandStationID, reqID)
}

// RemotePairingActiveKey is the Redis STRING key for one paired client.
func RemotePairingActiveKey(layoutID, commandStationID uint, clientKey string) string {
	return fmt.Sprintf(RemotePairingActiveKeyTmpl, layoutID, commandStationID, clientKey)
}

// RemotePairingActiveKeyPrefix is the Redis key prefix for all active sessions on
// one command station (used by Lua eviction scripts).
func RemotePairingActiveKeyPrefix(layoutID, commandStationID uint) string {
	return fmt.Sprintf("bigfred:remote:active:%d:%d:", layoutID, commandStationID)
}

// RemotePairingByUserKey points at the active clientKey for one user (STRING).
func RemotePairingByUserKey(layoutID, commandStationID, userID uint) string {
	return fmt.Sprintf(RemotePairingByUserKeyTmpl, layoutID, commandStationID, userID)
}

// RemotePairingReqByUserKey points at the pending req key for one user.
func RemotePairingReqByUserKey(layoutID, commandStationID, userID uint) string {
	return fmt.Sprintf(RemotePairingReqByUserKeyTmpl, layoutID, commandStationID, userID)
}

// RemotePairingReqDedupKey tracks in-flight pairing codes per protocol on one CS.
func RemotePairingReqDedupKey(layoutID, commandStationID uint, protocol string) string {
	return fmt.Sprintf(RemotePairingReqDedupKeyTmpl, layoutID, commandStationID, protocol)
}

// RemoteClientsSnapshotKey holds the latest handset presence snapshot for one CS.
func RemoteClientsSnapshotKey(layoutID, commandStationID uint) string {
	return fmt.Sprintf(RemoteClientsSnapshotKeyTmpl, layoutID, commandStationID)
}

// RemoteSessionSyncChannel is the pub/sub channel loco-server publishes on
// when a REST mutation changes an active handset session (unpair / scope
// update). The dcc-bus daemon subscribes per command station and re-syncs
// the affected client's in-process session without per-packet Redis reads.
const RemoteSessionSyncChannelTmpl = "bigfred:remote:sync:%d:%d"

// RemoteSessionSyncChannel returns the sync channel for one command station.
func RemoteSessionSyncChannel(layoutID, commandStationID uint) string {
	return fmt.Sprintf(RemoteSessionSyncChannelTmpl, layoutID, commandStationID)
}

// RemoteSessionSyncAction enumerates the REST mutations loco-server signals.
const (
	RemoteSessionSyncUnpair = "unpair"
	RemoteSessionSyncScope  = "scope"
)

// RemoteSessionSyncEventWire is the payload published on RemoteSessionSyncChannel.
type RemoteSessionSyncEventWire struct {
	LayoutID         uint   `json:"layoutId"`
	CommandStationID uint   `json:"commandStationId"`
	ClientKey        string `json:"clientKey"`
	Action           string `json:"action"`
}

// MarshalRemoteSessionSync encodes a sync event for Redis PUBLISH.
func MarshalRemoteSessionSync(w RemoteSessionSyncEventWire) ([]byte, error) {
	return json.Marshal(w)
}

// UnmarshalRemoteSessionSync decodes a sync event from a pub/sub message.
func UnmarshalRemoteSessionSync(raw []byte) (RemoteSessionSyncEventWire, error) {
	var w RemoteSessionSyncEventWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return RemoteSessionSyncEventWire{}, err
	}
	return w, nil
}

// RemotePendingWire is stored at RemotePairingReqKey with RemotePairingReqTTL.
type RemotePendingWire struct {
	LayoutID         uint     `json:"layoutId"`
	CommandStationID uint     `json:"commandStationId"`
	Protocol         string   `json:"protocol"`
	UserID           uint     `json:"userId"`
	UserLogin        string   `json:"userLogin,omitempty"`
	ReqID            string   `json:"reqId"`
	DisplayLabel     string   `json:"displayLabel"`
	VehicleIDs       []string `json:"vehicleIds"`
	AllowedAddrs     []uint16 `json:"allowedAddrs"`
	// AllowAllVehicles, when true, means every vehicle this user may
	// drive (owner or active lessee via ControllerUserIDs), not a
	// preselected VehicleIDs list and not the whole layout roster.
	AllowAllVehicles bool  `json:"allowAllVehicles"`
	HandsetBrakeSecs uint  `json:"handsetBrakeSecs"`
	CreatedAt        int64 `json:"createdAt"` // unix ms UTC
	// Z21-only pairing CV values (omitempty for other protocols).
	PairingCV3 int `json:"pairingCV3,omitempty"`
	PairingCV4 int `json:"pairingCV4,omitempty"`
	// WiThrottle-only 6-digit pairing code (omitempty for other protocols).
	PairingCode string `json:"pairingCode,omitempty"`
	// ExpectedClientKey restricts completion to a pre-declared handset (optional).
	ExpectedClientKey string `json:"expectedClientKey,omitempty"`
}

// RemoteSessionWire is stored at RemotePairingActiveKey until idle evict or logoff.
type RemoteSessionWire struct {
	Protocol     string   `json:"protocol"`
	UserID       uint     `json:"userId"`
	UserLogin    string   `json:"userLogin,omitempty"`
	VehicleIDs   []string `json:"vehicleIds"`
	AllowedAddrs []uint16 `json:"allowedAddrs"`
	// AllowAllVehicles, when true, means every vehicle this user may
	// drive (owner or active lessee via ControllerUserIDs), not a
	// preselected VehicleIDs list and not the whole layout roster.
	AllowAllVehicles bool   `json:"allowAllVehicles"`
	PairedAt         int64  `json:"pairedAt"`   // unix ms UTC
	LastSeenAt       int64  `json:"lastSeenAt"` // unix ms UTC
	ClientKey        string `json:"clientKey"`
	HandsetBrakeSecs uint   `json:"handsetBrakeSecs"`
	PairingCV3       int    `json:"pairingCV3,omitempty"`
	PairingCV4       int    `json:"pairingCV4,omitempty"`
	PairingCode      string `json:"pairingCode,omitempty"`
}

// MarshalRemotePending encodes a pending pairing request for Redis SET.
func MarshalRemotePending(w RemotePendingWire) ([]byte, error) {
	return json.Marshal(w)
}

// UnmarshalRemotePending decodes a pending pairing request from Redis GET.
func UnmarshalRemotePending(raw []byte) (RemotePendingWire, error) {
	var w RemotePendingWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return RemotePendingWire{}, err
	}
	return w, nil
}

// MarshalRemoteSession encodes an active handset session for Redis SET.
func MarshalRemoteSession(w RemoteSessionWire) ([]byte, error) {
	return json.Marshal(w)
}

// UnmarshalRemoteSession decodes an active handset session from Redis GET.
func UnmarshalRemoteSession(raw []byte) (RemoteSessionWire, error) {
	var w RemoteSessionWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return RemoteSessionWire{}, err
	}
	return w, nil
}

// UnmarshalJSON accepts Redis Lua cjson's empty-object stand-in for empty
// arrays (`"vehicleIds":{}`) so TouchSeen round-trips do not 500.
func (w *RemotePendingWire) UnmarshalJSON(raw []byte) error {
	coerced, err := coerceLuaEmptyArrays(raw)
	if err != nil {
		return err
	}
	type wire RemotePendingWire
	var tmp wire
	if err := json.Unmarshal(coerced, &tmp); err != nil {
		return err
	}
	*w = RemotePendingWire(tmp)
	return nil
}

// UnmarshalJSON accepts Redis Lua cjson's empty-object stand-in for empty
// arrays (`"vehicleIds":{}`) so TouchSeen round-trips do not 500.
func (w *RemoteSessionWire) UnmarshalJSON(raw []byte) error {
	coerced, err := coerceLuaEmptyArrays(raw)
	if err != nil {
		return err
	}
	type wire RemoteSessionWire
	var tmp wire
	if err := json.Unmarshal(coerced, &tmp); err != nil {
		return err
	}
	*w = RemoteSessionWire(tmp)
	return nil
}

// coerceLuaEmptyArrays rewrites "vehicleIds":{} / "allowedAddrs":{} to [].
// Redis lua-cjson encodes empty Lua tables as objects, not arrays.
func coerceLuaEmptyArrays(raw []byte) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	changed := false
	for _, key := range []string{"vehicleIds", "allowedAddrs"} {
		v, ok := obj[key]
		if !ok {
			continue
		}
		if !bytes.Equal(bytes.TrimSpace(v), []byte("{}")) {
			continue
		}
		obj[key] = json.RawMessage("[]")
		changed = true
	}
	if !changed {
		return raw, nil
	}
	return json.Marshal(obj)
}
