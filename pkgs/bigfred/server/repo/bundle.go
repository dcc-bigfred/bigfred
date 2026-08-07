package repo

import "github.com/go-rel/rel"

// UsersBundle is a convenience struct that wires every repository
// the service-level tests need. Production code wires each repo
// individually in `cli/root.go`; the bundle is intentionally test-
// only so it never grows wide enough to obscure the dependency graph
// in real callers.
type UsersBundle struct {
	Repo                  rel.Repository
	Users                 *Users
	Pool                  *DCCAddressRanges
	Vehicles              *Vehicles
	Trains                *Trains
	TrainMembers          *TrainMembers
	LayoutVehicles        *LayoutVehicles
	LayoutTrains          *LayoutTrains
	LayoutSignalmen       *LayoutSignalmen
	Layouts               *Layouts
	Interlockings         *Interlockings
	LayoutInterlockings   *LayoutInterlockings
	CommandStations       *CommandStations
	LayoutCommandStations *LayoutCommandStations
	DccFunctions          *DccFunctions
	SudoElevations        *SudoElevations
	VehicleLeases         VehicleLeaseStore
	TrainLeases           TrainLeaseStore
}
