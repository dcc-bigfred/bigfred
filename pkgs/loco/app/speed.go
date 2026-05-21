package app

import "github.com/keskad/loco/pkgs/loco/commandstation"

// SetSpeedAction sets the speed and direction of a locomotive
func (app *LocoApp) SetSpeedAction(locoId uint8, speed uint8, forward bool, speedSteps uint8) error {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.Station.CleanUp()

	return app.Station.SetSpeed(commandstation.LocoAddr(locoId), speed, forward, speedSteps)
}

// GetSpeedAction retrieves the current speed and direction of a locomotive
func (app *LocoApp) GetSpeedAction(locoId uint8) (speed uint8, forward bool, err error) {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return 0, false, cmdErr
	}
	defer app.Station.CleanUp()

	return app.Station.GetSpeed(commandstation.LocoAddr(locoId))
}
