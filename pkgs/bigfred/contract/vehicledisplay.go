package contract

import (
	"strconv"
	"strings"
)

// FormatVehicleDisplayName returns the roster/throttle label for a vehicle.
// It mirrors the BigFred UI name column: catalogue Name, then Number, then
// a generic fallback from the DCC address.
func FormatVehicleDisplayName(name, number string, addr uint16) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	number = strings.TrimSpace(number)
	if number != "" {
		return number
	}
	if addr != 0 {
		return formatLocoAddrFallback(addr)
	}
	return ""
}

func formatLocoAddrFallback(addr uint16) string {
	return "Loco " + strconv.Itoa(int(addr))
}
