package errors

import "errors"

const (
	CodeCommandStationNotFound               = "command_station_not_found"
	CodeCommandStationNameTaken              = "command_station_name_taken"
	CodeCommandStationNameRequired           = "command_station_name_required"
	CodeCommandStationKindInvalid            = "command_station_kind_invalid"
	CodeCommandStationSpeedInvalid           = "command_station_speed_steps_invalid"
	CodeCommandStationForbidden              = "forbidden"
	CodeLayoutNeedsAtLeastOneCommandStation  = "layout_needs_at_least_one_command_station"
	CodeSystemLayoutCommandStationsImmutable = "default_layout_command_stations_immutable"
)

var (
	ErrCommandStationNotFound               = errors.New(CodeCommandStationNotFound)
	ErrCommandStationNameTaken              = errors.New(CodeCommandStationNameTaken)
	ErrCommandStationNameRequired           = errors.New(CodeCommandStationNameRequired)
	ErrCommandStationKindInvalid            = errors.New(CodeCommandStationKindInvalid)
	ErrCommandStationSpeedInvalid           = errors.New(CodeCommandStationSpeedInvalid)
	ErrCommandStationForbidden              = errors.New(CodeCommandStationForbidden)
	ErrLayoutNeedsAtLeastOneCommandStation  = errors.New(CodeLayoutNeedsAtLeastOneCommandStation)
	ErrSystemLayoutCommandStationsImmutable = errors.New(CodeSystemLayoutCommandStationsImmutable)
)
