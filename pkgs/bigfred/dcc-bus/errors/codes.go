// Package errors holds machine-readable codes returned in dcc-bus WS
// ack payloads and loco.error frames. Drive-authority denials live in
// pkgs/dcc-bus/security (Reason* constants).
package errors

const (
	// CodeCommandStationError is returned when the underlying command
	// station rejects a SetSpeed or SendFn call.
	CodeCommandStationError = "command_station_error"
)
