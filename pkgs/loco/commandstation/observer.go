package commandstation

// LocoObservation is a state change a driver observed on the command
// station, including changes authored by an EXTERNAL throttle (a
// physical handheld plugged into the same command station, not driven
// through BigFred).
//
// A driver may report a partial update: only fields whose Has* flag is
// set are meaningful. For example a LocoNet OPC_LOCO_SPD packet carries
// only speed, while OPC_LOCO_DIRF carries direction plus F0..F4. The
// consumer is expected to merge the partial update onto the last known
// snapshot for the address.
//
// Speed is reported in the same units the driver's GetSpeed returns, so
// callers can treat observed and polled speeds interchangeably.
type LocoObservation struct {
	Addr LocoAddr

	HasSpeed bool
	Speed    uint8

	HasForward bool
	Forward    bool

	// Functions maps function number -> on for the function bits this
	// observation carries. Empty/nil when the update has no function
	// info. A driver SHOULD report the full bit it knows about (both
	// on and off) so the consumer can detect a function being turned
	// off, not only on.
	Functions map[int]bool
}

// StateObserver is an OPTIONAL capability implemented by command-station
// drivers that can asynchronously report state changes seen on the bus.
//
// Two mechanisms make this possible depending on the hardware:
//
//   - A shared serial/TCP bus such as LocoNet, where every device sees
//     every speed/direction/function packet authored by any throttle.
//   - A station-level push/broadcast such as the Z21
//     LAN_SET_BROADCASTFLAGS + LAN_X_LOCO_INFO subscription.
//
// Callers MUST treat the capability as optional: type-assert for it and
// fall back to polling GetSpeed / ListFunctions when a driver does not
// implement it (see the dcc-bus state feed).
type StateObserver interface {
	// ObserveStates returns a channel that emits a LocoObservation for
	// every state change the driver sees on the bus. The driver stops
	// emitting after CleanUp. Implementations return the same channel
	// on repeated calls.
	ObserveStates() <-chan LocoObservation
}
