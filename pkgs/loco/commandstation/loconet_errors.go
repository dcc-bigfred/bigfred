package commandstation

import "errors"

// LocoNet driver sentinel errors. Callers use errors.Is to distinguish them
// from wrapped transport or timeout failures.
var (
	// ErrSlotBusUnavailable is returned when the slot-acquire circuit breaker
	// is open after repeated command-station timeouts.
	ErrSlotBusUnavailable = errors.New("loconet: slot bus temporarily unavailable (circuit breaker open)")

	// ErrNoFreeSlot is returned when the command station rejects LOCO_ADR with
	// OPC_LONG_ACK code 0x00 (no free slot in the 120-slot table).
	ErrNoFreeSlot = errors.New("loconet: command station has no free slot")

	// ErrSpeedSuperseded is returned when a newer SetSpeed for the same address
	// overtook this call before its frame reached the bus. The newer call owns
	// downstream state; callers must not write Redis/UI for a superseded call.
	ErrSpeedSuperseded = errors.New("loconet: speed superseded by a newer SetSpeed")
)
