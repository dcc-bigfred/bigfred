package discovery

import (
	"fmt"
	"strings"
)

// InstanceName returns a DNS-SD instance label that stays unique per command station.
func InstanceName(csName string, commandStationID uint) string {
	name := strings.TrimSpace(csName)
	if name == "" {
		name = "BigFred"
	}
	return fmt.Sprintf("%s #%d", name, commandStationID)
}
