package domain

import "strings"

// VehicleID is the catalogue primary key for a pojazd (vehicle).
// Local rows use the V-{nanoid} form; legacy migrated rows use V-{integer}.
type VehicleID string

// TrainID is the catalogue primary key for a skład (train/consist).
type TrainID string

// EntitySource names the system that assigned ExternalID. "local" means
// the row was created inside BigFred and ExternalID is NULL.
type EntitySource string

const (
	EntitySourceLocal EntitySource = "local"
	vehicleIDPrefix                  = "V-"
	trainIDPrefix                    = "T-"
)

// String returns the wire/database representation.
func (id VehicleID) String() string { return string(id) }

// String returns the wire/database representation.
func (id TrainID) String() string { return string(id) }

// IsZero reports whether the ID is unset.
func (id VehicleID) IsZero() bool { return id == "" }

// IsZero reports whether the ID is unset.
func (id TrainID) IsZero() bool { return id == "" }

// Valid reports whether id carries the expected V- prefix.
func (id VehicleID) Valid() bool {
	return strings.HasPrefix(string(id), vehicleIDPrefix) && len(id) > len(vehicleIDPrefix)
}

// Valid reports whether id carries the expected T- prefix.
func (id TrainID) Valid() bool {
	return strings.HasPrefix(string(id), trainIDPrefix) && len(id) > len(trainIDPrefix)
}

// ParseVehicleID validates and returns a VehicleID.
func ParseVehicleID(raw string) (VehicleID, bool) {
	id := VehicleID(raw)
	return id, id.Valid()
}

// ParseTrainID validates and returns a TrainID.
func ParseTrainID(raw string) (TrainID, bool) {
	id := TrainID(raw)
	return id, id.Valid()
}
