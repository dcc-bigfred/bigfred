package errors

import "errors"

const (
	CodeZ21ServerDisabled       = "z21_server_disabled"
	CodeZ21CommandStationNotOnLayout = "command_station_not_on_layout"
	CodeZ21VehicleNotOnRoster   = "z21_vehicle_not_on_roster"
	CodeZ21VehicleNotDrivable   = "z21_vehicle_not_drivable"
	CodeZ21VehicleNoDCCAddress  = "z21_vehicle_no_dcc_address"
	CodeZ21SessionNotFound      = "z21_session_not_found"
	CodeZ21PairingScopeInvalid      = "z21_pairing_scope_invalid"
	CodeZ21HandsetBrakeSecsInvalid  = "z21_handset_brake_secs_invalid"
)

var (
	ErrZ21ServerDisabled            = errors.New(CodeZ21ServerDisabled)
	ErrZ21CommandStationNotOnLayout = errors.New(CodeZ21CommandStationNotOnLayout)
	ErrZ21VehicleNotOnRoster        = errors.New(CodeZ21VehicleNotOnRoster)
	ErrZ21VehicleNotDrivable        = errors.New(CodeZ21VehicleNotDrivable)
	ErrZ21VehicleNoDCCAddress       = errors.New(CodeZ21VehicleNoDCCAddress)
	ErrZ21SessionNotFound           = errors.New(CodeZ21SessionNotFound)
	ErrZ21PairingScopeInvalid       = errors.New(CodeZ21PairingScopeInvalid)
	ErrZ21HandsetBrakeSecsInvalid   = errors.New(CodeZ21HandsetBrakeSecsInvalid)
)
