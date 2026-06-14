package station

import (
	"fmt"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// Describe returns a log-safe summary of how the daemon will dial
// the command station (kind + target, no secrets).
func Describe(cs domain.CommandStation) string {
	switch cs.Kind {
	case domain.CommandStationKindZ21:
		return fmt.Sprintf("z21 uri=%q", cs.ConnectionURI)
	case domain.CommandStationKindLocoNetSerial:
		return fmt.Sprintf("loconet_serial uri=%q", cs.ConnectionURI)
	case domain.CommandStationKindLocoNetTCP:
		return fmt.Sprintf("loconet_tcp uri=%q", cs.ConnectionURI)
	default:
		return fmt.Sprintf("kind=%q uri=%q", cs.Kind, cs.ConnectionURI)
	}
}
