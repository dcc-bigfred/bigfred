package app

import (
	"time"

	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// progModeForLoco selects programming track when locoId is 0, PoM otherwise.
func progModeForLoco(locoId uint8) commandstation.Mode {
	if locoId == 0 {
		return commandstation.ProgrammingTrackMode
	}
	return commandstation.MainTrackMode
}

func newProgrammingCV(app *LocoApp, locoId uint8, timeout time.Duration) *stationCV {
	return &stationCV{
		station: app.Station,
		mode:    progModeForLoco(locoId),
		locoId:  commandstation.LocoAddr(locoId),
		timeout: timeout,
	}
}
