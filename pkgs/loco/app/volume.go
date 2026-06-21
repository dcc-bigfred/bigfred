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

	cv := &stationCV{
		station: app.Station,
		mode:    commandstation.ProgrammingTrackMode,
		locoId:  commandstation.LocoAddr(locoId),
		timeout: timeout,
	}

	decoder, err := decoders.Detect(cv)
	if err != nil {
		return err
	}
	return decoder.SetVolume(percent)
}

func (app *LocoApp) GetVolumeAction(locoId uint8, timeout time.Duration) error {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.Station.CleanUp()

	cv := &stationCV{
		station: app.Station,
		mode:    commandstation.ProgrammingTrackMode,
		locoId:  commandstation.LocoAddr(locoId),
		timeout: timeout,
	}

	decoder, err := decoders.Detect(cv)
	if err != nil {
		return err
	}

	percent, err := decoder.GetVolume()
	if err != nil {
		return err
	}

	app.P.Printf("%d\n", percent)
	return nil
}

func (app *LocoApp) DetectDecoderAction(locoId uint8, timeout time.Duration) error {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.Station.CleanUp()

	cv := &stationCV{
		station: app.Station,
		mode:    commandstation.ProgrammingTrackMode,
		locoId:  commandstation.LocoAddr(locoId),
		timeout: timeout,
	}

	id, err := decoders.Identify(cv)
	if err != nil {
		return err
	}

	if id.SoftwareVersion >= 0 {
		app.P.Printf("cv7=%d\n", id.SoftwareVersion)
	}
	app.P.Printf("cv8=%d\n", id.ManufacturerID)
	app.P.Printf("decoder=%s\n", id.Name)
	return nil
}
