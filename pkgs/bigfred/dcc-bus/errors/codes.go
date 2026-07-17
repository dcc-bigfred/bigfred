// Package errors holds machine-readable codes returned in dcc-bus WS
// ack payloads and loco.error frames. Drive-authority denials live in
// pkgs/dcc-bus/security (Reason* constants).
package errors

const (
	// CodeCommandStationError is returned when the underlying command
	// station rejects a SetSpeed or SendFn call.
	CodeCommandStationError = "command_station_error"

	// CodeSlotBusUnavailable is returned when the LocoNet slot-acquire circuit
	// breaker is open (repeated command-station timeouts / dead bus).
	CodeSlotBusUnavailable = "slot_bus_unavailable"

	// CodeNoFreeSlot is returned when the command station's slot table is full.
	CodeNoFreeSlot = "no_free_slot"

	// CodeSlotInUse is returned when allocatePhysicalSlots is enabled and the
	// loco is already IN_USE by another throttle (e.g. a physical FRED).
	CodeSlotInUse = "slot_in_use"

	// CodeBigFredSlotBudgetExceeded is returned when BigFred's max_loconet_slots
	// budget is exhausted before a bus round-trip is attempted.
	CodeBigFredSlotBudgetExceeded = "bigfred_slot_budget_exceeded"

	// CodeVehicleCapExceeded is returned when a user already holds the maximum
	// number of driven vehicle leases and none can be auto-evicted.
	CodeVehicleCapExceeded = "vehicle_cap_exceeded"

	// CodeSubscriptionCap is returned when a session exceeds max_vehicles_per_user
	// subscriptions; the oldest subscription is dropped.
	CodeSubscriptionCap = "subscription_cap"

	// CodeIdleTimeout is returned when a remote drive lease is released after
	// idle_timeout_secs without drive activity.
	CodeIdleTimeout = "idle_timeout"

	// CodeNotAllowed is returned when a select/drive request is denied by the
	// drive-authority policy before any slot operation.
	CodeNotAllowed = "not_allowed"

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
