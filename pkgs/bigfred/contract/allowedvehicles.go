package contract

import (
	"encoding/json"
	"fmt"
	"time"
)

// Layout roster snapshots (loco-server → dcc-bus). Each key is both a Redis
// STRING and a pub/sub channel; loco-server SET + PUBLISH in one pipeline.

const (
	// AllowedVehiclesKeyTmpl is the Redis STRING key — and the pub/sub
	// channel — carrying a layout's drivable vehicle roster snapshot.
	// Verb: layoutID.
	AllowedVehiclesKeyTmpl = "bigfred:layout:%d:allowed_vehicles"

	// DefinedTrainsKeyTmpl is the Redis STRING key — and the pub/sub
	// channel — carrying the trains attached to a layout roster.
	// Verb: layoutID.
	DefinedTrainsKeyTmpl = "bigfred:layout:%d:defined_trains"
)

// AllowedVehiclesKey is the Redis STRING key and pub/sub channel for
// the layout's drivable vehicle roster.
func AllowedVehiclesKey(layoutID uint) string {
	return fmt.Sprintf(AllowedVehiclesKeyTmpl, layoutID)
}

// DefinedTrainsKey is the Redis STRING key and pub/sub channel for
// trains attached to the layout roster.
func DefinedTrainsKey(layoutID uint) string {
	return fmt.Sprintf(DefinedTrainsKeyTmpl, layoutID)
}

// AllowedVehicles is the authoritative drivable roster for one layout.
// Only vehicles with a DCC address appear in Vehicles (dummies are
// omitted — they never reach the bus).
type AllowedVehicles struct {
	LayoutID  uint             `json:"layoutId"`
	UpdatedAt int64            `json:"updatedAt"` // unix ms UTC
	Vehicles  []AllowedVehicle `json:"vehicles"`
}

// AllowedVehicle ties a DCC address to catalogue metadata dcc-bus
// needs for subscribe gating and drive-authority checks without SQLite.
type AllowedVehicle struct {
	VehicleID   uint   `json:"vehicleId"`
	Addr        uint16 `json:"addr"`
	OwnerUserID uint   `json:"ownerUserId"`

	// ControllerUserIDs is the flat, already-resolved set of user ids
	// allowed to drive this address right now. loco-server computes it
	// from the catalogue (owner + active lessees + the 5-minute takeover
	// self-lease holder, §4.3) and republishes the snapshot whenever it
	// changes. dcc-bus has NO concept of a lease or a takeover: it only
	// checks membership in this slice. See §7e.5.
	ControllerUserIDs []uint `json:"controllerUserIds"`

	// Per-vehicle dead-man's switch catalogue (§7e.5). Copied from the
	// vehicle row so dcc-bus can act without SQLite.
	Rp1Function             uint8  `json:"rp1Function"`
	EmergencyLightsFunction uint8  `json:"emergencyLightsFunction"`
	DeadManSwitchOption     string `json:"deadManSwitchOption"`
}

// DefinedTrains lists every train on the layout roster with members
// resolved to DCC addresses where available.
type DefinedTrains struct {
	LayoutID  uint           `json:"layoutId"`
	UpdatedAt int64          `json:"updatedAt"`
	Trains    []DefinedTrain `json:"trains"`
}

// DefinedTrain is one consist on the layout roster.
type DefinedTrain struct {
	TrainID     uint                 `json:"trainId"`
	OwnerUserID uint                 `json:"ownerUserId"`

	// ControllerUserIDs is the flat set of user ids allowed to drive
	// this train right now (owner + active train lessees).
	ControllerUserIDs []uint `json:"controllerUserIds"`

	Members []DefinedTrainMember `json:"members"`
}

// DefinedTrainMember is one vehicle in consist order.
type DefinedTrainMember struct {
	VehicleID uint    `json:"vehicleId"`
	Position  int     `json:"position"`
	Reversed        bool    `json:"reversed"`
	SpeedMultiplier float64 `json:"speedMultiplier"`
	Addr            *uint16 `json:"addr,omitempty"`
}

// Marshal encodes a snapshot for Redis SET/PUBLISH.
func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalAllowedVehicles decodes a pub/sub payload or GET value.
func UnmarshalAllowedVehicles(raw []byte) (AllowedVehicles, error) {
	var snap AllowedVehicles
	if err := json.Unmarshal(raw, &snap); err != nil {
		return AllowedVehicles{}, err
	}
	return snap, nil
}

// UnmarshalDefinedTrains decodes a pub/sub payload or GET value.
func UnmarshalDefinedTrains(raw []byte) (DefinedTrains, error) {
	var snap DefinedTrains
	if err := json.Unmarshal(raw, &snap); err != nil {
		return DefinedTrains{}, err
	}
	return snap, nil
}

// NowMS returns a UTC unix-millisecond timestamp for UpdatedAt fields.
func NowMS() int64 {
	return time.Now().UTC().UnixMilli()
}

// BuildAllowedVehiclesPayload marshals the JSON SET and PUBLISHed under
// AllowedVehiclesKey(layoutID).
func BuildAllowedVehiclesPayload(layoutID uint, updatedAt int64, vehicles []AllowedVehicle) ([]byte, error) {
	return Marshal(AllowedVehicles{LayoutID: layoutID, UpdatedAt: updatedAt, Vehicles: vehicles})
}

// BuildDefinedTrainsPayload marshals the JSON SET and PUBLISHed under
// DefinedTrainsKey(layoutID).
func BuildDefinedTrainsPayload(layoutID uint, updatedAt int64, trains []DefinedTrain) ([]byte, error) {
	return Marshal(DefinedTrains{LayoutID: layoutID, UpdatedAt: updatedAt, Trains: trains})
}
