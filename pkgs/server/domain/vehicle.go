package domain

import "time"

// VehicleKind is the closed catalogue of physical vehicle classes
// that show up on a modelling layout. Per §3a.1 the value drives UI
// icons and filtering but never DCC addressing — every kind may
// carry an optional DCC address.
type VehicleKind string

const (
	VehicleKindLoco         VehicleKind = "loco"          // Lokomotywa
	VehicleKindEMU          VehicleKind = "emu"           // EZT — elektryczny/diesel zespół trakcyjny
	VehicleKindDrivingWagon VehicleKind = "driving_wagon" // Wagon sterowniczy
	VehicleKindTrolley      VehicleKind = "trolley"       // Drezyna
	VehicleKindWagon        VehicleKind = "wagon"         // Wagon pasywny
)

// VehicleKinds returns the closed catalogue in display-order. The
// frontend dropdown can either render every value or filter to a
// subset (e.g. only `loco` for the M2 throttle bring-up).
func VehicleKinds() []VehicleKind {
	return []VehicleKind{
		VehicleKindLoco,
		VehicleKindEMU,
		VehicleKindDrivingWagon,
		VehicleKindTrolley,
		VehicleKindWagon,
	}
}

// IsValid reports whether the value is one of the catalogue entries.
// Returns false for the zero value so an uninitialised struct never
// passes validation by accident.
func (k VehicleKind) IsValid() bool {
	switch k {
	case VehicleKindLoco, VehicleKindEMU, VehicleKindDrivingWagon, VehicleKindTrolley, VehicleKindWagon:
		return true
	}
	return false
}

// Vehicle is one rail vehicle the system tracks (§3a.1).
//
// DCCAddress is OPTIONAL:
//   - non-nil — the vehicle is steerable, the address must fall inside
//     the owner's DCC pool, and (DCCAddress) is unique on the track
//     (enforced by a partial UNIQUE index WHERE dcc_address IS NOT NULL).
//   - nil    — the vehicle is a DUMMY: still listed in the catalogue,
//     still attachable to a train, still visible on the layout roster,
//     but the throttle never sends DCC against it. Typical for
//     unpowered wagons and visual fillers.
//
// Number is optional, free-text — the road number / inventory tag
// painted on the vehicle (e.g. "ET22-1175", "92510"). Kept as a
// separate column from Name so the UI can render it as a small mono
// caption next to the user-given Name.
type Vehicle struct {
	ID          uint
	DCCAddress  *uint16 `db:"dcc_address"`
	OwnerUserID uint    `db:"owner_user_id"`
	Name        string
	Kind        VehicleKind
	Number      string
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// Table tells REL which physical table backs this struct.
func (Vehicle) Table() string { return "vehicles" }

// IsDummy reports whether the vehicle has no DCC address and can
// therefore never be driven from the throttle.
func (v Vehicle) IsDummy() bool { return v.DCCAddress == nil }
