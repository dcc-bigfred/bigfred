package cmd

import (
	"errors"
	"time"

	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// ReadLocoCV reads one CV from a locomotive on the main track (POM / RailCom).
func (r *Router) ReadLocoCV(addr uint16, cvNum commandstation.CVNum) (int, error) {
	if r == nil || r.station == nil {
		return 0, errors.New("dcc-bus: no command station")
	}
	return r.station.ReadCV(commandstation.MainTrackMode, commandstation.LocoCV{
		LocoId: commandstation.LocoAddr(addr),
		Cv:     commandstation.CV{Num: cvNum},
	}, commandstation.Timeout(15*time.Second), commandstation.Retries(1))
}
