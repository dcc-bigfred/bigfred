// Package layoutroster defines the Redis snapshot format loco-server
// publishes and dcc-bus consumes for layout vehicle/train rosters.
package layoutroster

import (
	"encoding/json"
	"fmt"
	"time"
)

// AllowedVehiclesKey is the Redis STRING key and pub/sub channel for
// the layout's drivable vehicle roster.
func AllowedVehiclesKey(layoutID uint) string {
	return fmt.Sprintf("bigfred:layout:%d:allowed_vehicles", layoutID)
}

// DefinedTrainsKey is the Redis STRING key and pub/sub channel for
// trains attached to the layout roster.
func DefinedTrainsKey(layoutID uint) string {
	return fmt.Sprintf("bigfred:layout:%d:defined_trains", layoutID)
}

// AllowedVehicles is the authoritative drivable roster for one layout.
// Only vehicles with a DCC address appear in Vehicles (dummies are
// omitted — they never reach the bus).
type AllowedVehicles struct {
	LayoutID  uint              `json:"layoutId"`
	UpdatedAt int64             `json:"updatedAt"` // unix ms UTC
	Vehicles  []AllowedVehicle  `json:"vehicles"`
}

// AllowedVehicle ties a DCC address to catalogue metadata dcc-bus
// needs for subscribe gating and drive-authority checks without SQLite.
type AllowedVehicle struct {
	VehicleID         uint   `json:"vehicleId"`
	Addr              uint16 `json:"addr"`
	OwnerUserID       uint   `json:"ownerUserId"`
	ControllerUserIDs []uint `json:"controllerUserIds"`
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
	TrainID       uint                 `json:"trainId"`
	OwnerUserID   uint                 `json:"ownerUserId"`
	Members       []DefinedTrainMember `json:"members"`
}

// DefinedTrainMember is one vehicle in consist order.
type DefinedTrainMember struct {
	VehicleID uint    `json:"vehicleId"`
	Position  int     `json:"position"`
	Reversed  bool    `json:"reversed"`
	Addr      *uint16 `json:"addr,omitempty"`
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
