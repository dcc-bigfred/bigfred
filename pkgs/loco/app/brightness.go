package app

import (
	"time"

	"github.com/keskad/loco/pkgs/loco/decoders"
)

func (app *LocoApp) SetBrightnessAction(locoId uint8, output uint8, percent uint8, timeout time.Duration) error {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.Station.CleanUp()

	cv := newProgrammingCV(app, locoId, timeout)

	decoder, err := decoders.DetectBrightness(cv)
	if err != nil {
		return err
	}
	return decoder.SetBrightness(output, percent)
}

func (app *LocoApp) GetBrightnessAction(locoId uint8, output uint8, timeout time.Duration) error {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.Station.CleanUp()

	cv := newProgrammingCV(app, locoId, timeout)

	decoder, err := decoders.DetectBrightness(cv)
	if err != nil {
		return err
	}

	percent, err := decoder.GetBrightness(output)
	if err != nil {
		return err
	}

	app.P.Printf("%d\n", percent)
	return nil
}

func (app *LocoApp) ListBrightnessAction(locoId uint8, timeout time.Duration) error {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.Station.CleanUp()

	cv := newProgrammingCV(app, locoId, timeout)

	decoder, err := decoders.DetectBrightness(cv)
	if err != nil {
		return err
	}

	for _, output := range decoder.Outputs() {
		percent, err := decoder.GetBrightness(output)
		if err != nil {
			return err
		}
		app.P.Printf("output=%d brightness=%d\n", output, percent)
	}
	return nil
}

func (app *LocoApp) TestBrightnessAction(locoId uint8, timeout time.Duration, pause time.Duration) error {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.Station.CleanUp()

	cv := newProgrammingCV(app, locoId, timeout)

	decoder, err := decoders.DetectBrightness(cv)
	if err != nil {
		return err
	}

	app.P.Printf("Turn on all light functions on the locomotive before the test starts.\n")
	if pause > 0 {
		app.P.Printf("Starting in %d seconds…\n", int(pause.Seconds()))
		time.Sleep(pause)
	}

	snapshot, err := decoder.SnapshotBrightness()
	if err != nil {
		return err
	}

	app.P.Printf("Saved brightness values:\n")
	for _, state := range snapshot {
		app.P.Printf("output=%d cv%d=%d\n", state.Output, state.CV, state.Value)
	}

	app.P.Printf("Running brightness test…\n")
	_, err = decoders.RunBrightnessTest(decoder, time.Sleep)
	if err != nil {
		return err
	}

	app.P.Printf("Brightness test complete.\n")
	return nil
}
