package repo

// UsersBundle is a convenience struct that wires every repository
// the service-level tests need. Production code wires each repo
// individually in `cli/root.go`; the bundle is intentionally test-
// only so it never grows wide enough to obscure the dependency graph
// in real callers.
type UsersBundle struct {
	Users               *Users
	Pool                *DCCAddressRanges
	Vehicles            *Vehicles
	Trains              *Trains
	TrainMembers        *TrainMembers
	LayoutVehicles      *LayoutVehicles
	LayoutTrains        *LayoutTrains
	LayoutSignalmen     *LayoutSignalmen
	Layouts             *Layouts
	Interlockings       *Interlockings
	LayoutInterlockings   *LayoutInterlockings
	CommandStations       *CommandStations
	LayoutCommandStations *LayoutCommandStations
	SudoElevations        *SudoElevations
}
