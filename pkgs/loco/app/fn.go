package app

import "github.com/keskad/loco/pkgs/loco/commandstation"

func (app *LocoApp) SendFnAction(mode string, locoId uint8, fnNum int, toggle bool) error {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.Station.CleanUp()
	return app.Station.SendFn(commandstation.Mode(mode), commandstation.LocoAddr(locoId), commandstation.FuncNum(fnNum), toggle)
}

func (app *LocoApp) ListFnAction(locoId uint8) ([]int, error) {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return nil, cmdErr
	}
	defer app.Station.CleanUp()

	return app.Station.ListFunctions(commandstation.LocoAddr(locoId))
}
