package contract

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

const (
	// Z21PairingReqTTL is how long a generated CV3/CV4 pair stays valid.
	Z21PairingReqTTL = 5 * time.Minute

	Z21PairingReqKeyTmpl        = "bigfred:z21pair:req:%d:%d:%d:%d"
	Z21PairingActiveKeyTmpl     = "bigfred:z21pair:active:%d:%d:%s"
	Z21PairingByUserKeyTmpl     = "bigfred:z21pair:byuser:%d:%d:%d"
	Z21PairingReqPairsKeyTmpl   = "bigfred:z21pair:reqpairs:%d:%d"
	Z21PairingReqByUserKeyTmpl  = "bigfred:z21pair:reqbyuser:%d:%d:%d"
	Z21PairingActiveKeyScanTmpl = "bigfred:z21pair:active:%d:%d:*"
	Z21ClientsSnapshotKeyTmpl   = "bigfred:z21:clients:%d:%d"

	// Z21StickySessionIdle is how long an IP-sticky handset session stays
	// paired without UDP activity before unpair.
	Z21StickySessionIdle = 30 * time.Minute

	Z21HandsetBrakeSecsDefault = 6
	Z21HandsetBrakeSecsMin     = 6
	Z21HandsetBrakeSecsMax     = 60
)

// Z21PairingReqKey is the Redis STRING key for one pending handset pair.
func Z21PairingReqKey(layoutID, commandStationID uint, pairingCV3, pairingCV4 int) string {
	return fmt.Sprintf(Z21PairingReqKeyTmpl, layoutID, commandStationID, pairingCV3, pairingCV4)
}

// Z21PairingActiveKey is the Redis STRING key for one paired UDP endpoint.
func Z21PairingActiveKey(layoutID, commandStationID uint, clientKey string) string {
	return fmt.Sprintf(Z21PairingActiveKeyTmpl, layoutID, commandStationID, clientKey)
}

// Z21PairingByUserKey indexes active clientKey values for one user.
func Z21PairingByUserKey(layoutID, commandStationID, userID uint) string {
	return fmt.Sprintf(Z21PairingByUserKeyTmpl, layoutID, commandStationID, userID)
}

// Z21PairingReqPairsKey tracks in-flight CV pairs on one command station.
func Z21PairingReqPairsKey(layoutID, commandStationID uint) string {
	return fmt.Sprintf(Z21PairingReqPairsKeyTmpl, layoutID, commandStationID)
}

// Z21PairingReqByUserKey points at the pending req key for one user.
func Z21PairingReqByUserKey(layoutID, commandStationID, userID uint) string {
	return fmt.Sprintf(Z21PairingReqByUserKeyTmpl, layoutID, commandStationID, userID)
}

// Z21PairingActiveKeyScanPattern matches every active session on one CS.
func Z21PairingActiveKeyScanPattern(layoutID, commandStationID uint) string {
	return fmt.Sprintf(Z21PairingActiveKeyScanTmpl, layoutID, commandStationID)
}

// Z21ClientsSnapshotKey holds the latest handset presence snapshot for one CS.
func Z21ClientsSnapshotKey(layoutID, commandStationID uint) string {
	return fmt.Sprintf(Z21ClientsSnapshotKeyTmpl, layoutID, commandStationID)
}

// Z21PairLabel formats a CV3/CV4 pair for the reqpairs SET.
func Z21PairLabel(pairingCV3, pairingCV4 int) string {
	return fmt.Sprintf("%d:%d", pairingCV3, pairingCV4)
}

// Z21PairingDisplayLabel is shorthand shown in the UI (e.g. "122-145").
func Z21PairingDisplayLabel(pairingCV3, pairingCV4 int) string {
	return fmt.Sprintf("%d-%d", pairingCV3, pairingCV4)
}

// ValidPairingCV reports whether v satisfies the [1–2][1–5][1–5] pattern (111–255).
func ValidPairingCV(v int) bool {
	if v < 111 || v > 255 {
		return false
	}
	d1, d2, d3 := v/100, (v/10)%10, v%10
	return d1 >= 1 && d1 <= 2 && d2 >= 1 && d2 <= 5 && d3 >= 1 && d3 <= 5
}

// AllValidPairingCVs returns every allowed single-CV pairing value.
func AllValidPairingCVs() []int {
	out := make([]int, 0, 50)
	for v := 111; v <= 255; v++ {
		if ValidPairingCV(v) {
			out = append(out, v)
		}
	}
	return out
}

// RandomPairingCV picks one valid CV value using rng.
func RandomPairingCV(rng *rand.Rand) int {
	vals := AllValidPairingCVs()
	return vals[rng.Intn(len(vals))]
}

// Z21PairingReqWire is stored at Z21PairingReqKey with a 5-minute TTL.
type Z21PairingReqWire struct {
	LayoutID          uint     `json:"layoutId"`
	CommandStationID  uint     `json:"commandStationId"`
	UserID            uint     `json:"userId"`
	PairingCV3        int      `json:"pairingCV3"`
	PairingCV4        int      `json:"pairingCV4"`
	DisplayLabel      string   `json:"displayLabel"`
	VehicleIDs        []string `json:"vehicleIds"`
	AllowedAddrs      []uint16 `json:"allowedAddrs"`
	AllowAllVehicles  bool     `json:"allowAllVehicles"`
	HandsetBrakeSecs  uint     `json:"handsetBrakeSecs"`
	CreatedAt         int64    `json:"createdAt"` // unix ms UTC
}

// Z21PairingActiveWire is stored at Z21PairingActiveKey until §1.1 idle or LAN_LOGOFF.
type Z21PairingActiveWire struct {
	UserID           uint     `json:"userId"`
	VehicleIDs       []string `json:"vehicleIds"`
	AllowedAddrs     []uint16 `json:"allowedAddrs"`
	AllowAllVehicles bool     `json:"allowAllVehicles"`
	PairedAt         int64    `json:"pairedAt"`    // unix ms UTC
	PairingCV3       int      `json:"pairingCV3"`
	PairingCV4       int      `json:"pairingCV4"`
	LastSeenAt       int64    `json:"lastSeenAt"`  // unix ms UTC
	ClientKey        string   `json:"clientKey"`   // "<ip>:<port>" or "<ip>" when sticky
	HandsetBrakeSecs uint     `json:"handsetBrakeSecs"`
}

// Z21ClientWire describes one inbound Z21 UDP participant.
type Z21ClientWire struct {
	ClientKey        string `json:"clientKey"`
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

// Z21ClientsSnapshotWire is stored at Z21ClientsSnapshotKey and pushed on
// z21.clients.changed events.
type Z21ClientsSnapshotWire struct {
	LayoutID         uint            `json:"layoutId"`
	CommandStationID uint            `json:"commandStationId"`
	IPStickiness     bool            `json:"ipStickiness"`
	UpdatedAt        int64           `json:"updatedAt"`
	Clients          []Z21ClientWire `json:"clients"`
}

// MarshalZ21ClientsSnapshot encodes a clients snapshot for Redis SET.
func MarshalZ21ClientsSnapshot(w Z21ClientsSnapshotWire) ([]byte, error) {
	return json.Marshal(w)
}

// UnmarshalZ21ClientsSnapshot decodes a clients snapshot from Redis GET.
func UnmarshalZ21ClientsSnapshot(raw []byte) (Z21ClientsSnapshotWire, error) {
	var w Z21ClientsSnapshotWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return Z21ClientsSnapshotWire{}, err
	}
	return w, nil
}

// MarshalZ21PairingReq encodes a pending pairing request for Redis SET.
func MarshalZ21PairingReq(w Z21PairingReqWire) ([]byte, error) {
	return json.Marshal(w)
}

// UnmarshalZ21PairingReq decodes a pending pairing request from Redis GET.
func UnmarshalZ21PairingReq(raw []byte) (Z21PairingReqWire, error) {
	var w Z21PairingReqWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return Z21PairingReqWire{}, err
	}
	return w, nil
}

// MarshalZ21PairingActive encodes an active handset session for Redis SET.
func MarshalZ21PairingActive(w Z21PairingActiveWire) ([]byte, error) {
	return json.Marshal(w)
}

// UnmarshalZ21PairingActive decodes an active handset session from Redis GET.
func UnmarshalZ21PairingActive(raw []byte) (Z21PairingActiveWire, error) {
	var w Z21PairingActiveWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return Z21PairingActiveWire{}, err
	}
	return w, nil
}

// ValidHandsetBrakeSecs reports whether secs is in the allowed pairing range.
func ValidHandsetBrakeSecs(secs uint) bool {
	return secs >= Z21HandsetBrakeSecsMin && secs <= Z21HandsetBrakeSecsMax
}

// NormaliseHandsetBrakeSecs returns secs clamped to the allowed range, or the
// default when secs is zero (legacy rows).
func NormaliseHandsetBrakeSecs(secs uint) uint {
	if secs == 0 {
		return Z21HandsetBrakeSecsDefault
	}
	if secs < Z21HandsetBrakeSecsMin {
		return Z21HandsetBrakeSecsMin
	}
	if secs > Z21HandsetBrakeSecsMax {
		return Z21HandsetBrakeSecsMax
	}
	return secs
}
