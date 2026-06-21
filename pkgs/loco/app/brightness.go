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

func (app *LocoApp) GetBrightnessAction(locoId uint8, output uint8, timeout time.Duration) (uint8, error) {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return 0, cmdErr
	}
	defer app.Station.CleanUp()

	cv := newProgrammingCV(app, locoId, timeout)

	decoder, err := decoders.DetectBrightness(cv)
	if err != nil {
		return 0, err
	}

	return decoder.GetBrightness(output)
}

func (app *LocoApp) ListBrightnessAction(locoId uint8, timeout time.Duration) ([]OutputBrightnessLevel, error) {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return nil, cmdErr
	}
	defer app.Station.CleanUp()

	cv := newProgrammingCV(app, locoId, timeout)

	decoder, err := decoders.DetectBrightness(cv)
	if err != nil {
		return nil, err
	}

	levels := make([]OutputBrightnessLevel, 0, len(decoder.Outputs()))
	for _, output := range decoder.Outputs() {
		percent, err := decoder.GetBrightness(output)
		if err != nil {
			return nil, err
		}
		levels = append(levels, OutputBrightnessLevel{
			Output:     output,
			Brightness: percent,
		})
	}
	return levels, nil
}

func (app *LocoApp) TestBrightnessAction(locoId uint8, timeout time.Duration) ([]decoders.OutputBrightness, error) {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return nil, cmdErr
	}
	defer app.Station.CleanUp()

	cv := newProgrammingCV(app, locoId, timeout)

	decoder, err := decoders.DetectBrightness(cv)
	if err != nil {
		return nil, err
	}

	return decoders.RunBrightnessTest(decoder, time.Sleep)
}
