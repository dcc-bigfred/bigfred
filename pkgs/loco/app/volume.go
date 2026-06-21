package app

import (
	"time"

	"github.com/keskad/loco/pkgs/loco/commandstation"
	"github.com/keskad/loco/pkgs/loco/decoders"
)

type stationCV struct {
	station commandstation.Station
	mode    commandstation.Mode
	locoId  commandstation.LocoAddr
	timeout time.Duration
}

func (s *stationCV) ReadCV(num uint16) (int, error) {
	return s.station.ReadCV(s.mode, commandstation.LocoCV{
		LocoId: s.locoId,
		Cv:     commandstation.CV{Num: commandstation.CVNum(num)},
	}, commandstation.Timeout(s.timeout))
}

func (s *stationCV) WriteCV(num uint16, value int) error {
	return s.station.WriteCV(s.mode, commandstation.LocoCV{
		LocoId: s.locoId,
		Cv:     commandstation.CV{Num: commandstation.CVNum(num), Value: value},
	}, commandstation.Timeout(s.timeout))
}

func (app *LocoApp) SetVolumeAction(locoId uint8, percent uint8, timeout time.Duration) error {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.Station.CleanUp()

	cv := newProgrammingCV(app, locoId, timeout)

	decoder, err := decoders.Detect(cv)
	if err != nil {
		return err
	}
	return decoder.SetVolume(percent)
}

func (app *LocoApp) GetVolumeAction(locoId uint8, timeout time.Duration) (uint8, error) {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return 0, cmdErr
	}
	defer app.Station.CleanUp()

	cv := newProgrammingCV(app, locoId, timeout)

	decoder, err := decoders.Detect(cv)
	if err != nil {
		return 0, err
	}

	return decoder.GetVolume()
}

func (app *LocoApp) DetectDecoderAction(locoId uint8, timeout time.Duration) (decoders.Identification, error) {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return decoders.Identification{}, cmdErr
	}
	defer app.Station.CleanUp()

	cv := newProgrammingCV(app, locoId, timeout)
	return decoders.Identify(cv)
}
