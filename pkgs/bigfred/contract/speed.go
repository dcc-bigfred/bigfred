package contract

// UISpeedFromWire maps a command-station speed reading to the UI snapshot
// value. Wire speed 1 is DCC EMG-stop; halted locos are always exposed as 0.
func UISpeedFromWire(wire uint8) uint8 {
	if wire == 1 {
		return 0
	}
	return wire
}
