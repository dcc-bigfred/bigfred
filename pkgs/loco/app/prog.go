package app

import (
	"github.com/dcc-bigfred/bigfred/pkgs/loco/commandstation"
)

// progModeForLoco selects programming track when locoId is 0, PoM otherwise.
func progModeForLoco(locoId uint8) commandstation.Mode {
	if locoId == 0 {
		return commandstation.ProgrammingTrackMode
	}
	return commandstation.MainTrackMode
}
