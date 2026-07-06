package errors

import "errors"

const (
	CodeCommandStationNotFound               = "command_station_not_found"
	CodeCommandStationNameTaken              = "command_station_name_taken"
	CodeCommandStationNameRequired           = "command_station_name_required"
	CodeCommandStationKindInvalid            = "command_station_kind_invalid"
	CodeCommandStationSpeedInvalid           = "command_station_speed_steps_invalid"
	CodeCommandStationHeartbeatInvalid       = "command_station_heartbeat_invalid"
	CodeCommandStationDeadmanInvalid         = "command_station_deadman_invalid"
	CodeCommandStationDeadmanTooShort        = "command_station_deadman_too_short"
	CodeCommandStationPollIntervalInvalid    = "command_station_poll_interval_invalid"
	CodeCommandStationInboundPortConflict    = "command_station_inbound_port_conflict"
	CodeCommandStationMaxLoconetSlotsInvalid  = "command_station_max_loconet_slots_invalid"
	CodeCommandStationIdleTimeoutInvalid     = "command_station_idle_timeout_invalid"
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
	ErrCommandStationHeartbeatInvalid       = errors.New(CodeCommandStationHeartbeatInvalid)
	ErrCommandStationDeadmanInvalid         = errors.New(CodeCommandStationDeadmanInvalid)
	ErrCommandStationDeadmanTooShort        = errors.New(CodeCommandStationDeadmanTooShort)
	ErrCommandStationPollIntervalInvalid    = errors.New(CodeCommandStationPollIntervalInvalid)
	ErrCommandStationInboundPortConflict    = errors.New(CodeCommandStationInboundPortConflict)
	ErrCommandStationMaxLoconetSlotsInvalid = errors.New(CodeCommandStationMaxLoconetSlotsInvalid)
	ErrCommandStationIdleTimeoutInvalid     = errors.New(CodeCommandStationIdleTimeoutInvalid)
	ErrCommandStationForbidden              = errors.New(CodeCommandStationForbidden)
	ErrLayoutNeedsAtLeastOneCommandStation  = errors.New(CodeLayoutNeedsAtLeastOneCommandStation)
	ErrSystemLayoutCommandStationsImmutable = errors.New(CodeSystemLayoutCommandStationsImmutable)
)
