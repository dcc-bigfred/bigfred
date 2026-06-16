// Package errors holds machine-readable codes returned in dcc-bus WS
// ack payloads and loco.error frames. Drive-authority denials live in
// pkgs/dcc-bus/security (Reason* constants).
package errors

const (
	// CodeCommandStationError is returned when the underlying command
	// station rejects a SetSpeed or SendFn call.
	CodeCommandStationError = "command_station_error"

	// CodeTrainNotOnLayout is returned when train.setSpeed references an
	// unknown train on this layout.
	CodeTrainNotOnLayout = "train_not_on_layout"
	// CodeTrainNoPoweredMembers is returned when a train has no powered
	// DCC members to drive.
	CodeTrainNoPoweredMembers = "train_no_powered_members"
	// CodePartialFailure is returned when train.setSpeed succeeds for
	// some members but not all.
	CodePartialFailure = "partial_failure"
)
