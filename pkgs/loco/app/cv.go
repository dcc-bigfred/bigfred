package app

import (
	"fmt"
	"time"

	"github.com/keskad/loco/pkgs/loco/commandstation"
	"github.com/keskad/loco/pkgs/loco/syntax"
	"github.com/sirupsen/logrus"
)

func (app *LocoApp) SendCVAction(mode string, locoId uint8, cvNumRaw string, verify bool, timeout time.Duration, settle time.Duration) error {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.Station.CleanUp()

	entries, parseErr := syntax.ParseCVString(cvNumRaw, ",")
	if parseErr != nil {
		return parseErr
	}

	var writeErr error
	for _, entry := range entries {
		writeErr = app.Station.WriteCV(commandstation.Mode(mode), commandstation.LocoCV{
			LocoId: commandstation.LocoAddr(locoId),
			Cv: commandstation.CV{
				Num:   commandstation.CVNum(entry.Number),
				Value: int(entry.Value),
			},
		},
			commandstation.Verify(verify),
			commandstation.Timeout(timeout))

		time.Sleep(settle)

		if writeErr != nil {
			return writeErr
		}
	}

	return nil
}

func (app *LocoApp) ReadCVAction(mode string, locoId uint8, cvNumRaw string, verify bool, timeout time.Duration, retries uint8) ([]CVRead, error) {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return nil, cmdErr
	}
	defer app.Station.CleanUp()

	entries, parseErr := syntax.ParseCVString(cvNumRaw, ",")
	if parseErr != nil {
		return nil, fmt.Errorf("invalid format: %s", cvNumRaw)
	}

	reads := make([]CVRead, 0, len(entries))
	for _, entry := range entries {
		result, err := app.Station.ReadCV(commandstation.Mode(mode), commandstation.LocoCV{
			LocoId: commandstation.LocoAddr(locoId),
			Cv: commandstation.CV{
				Num: commandstation.CVNum(entry.Number),
			},
		}, commandstation.Verify(verify),
			commandstation.Timeout(timeout),
			commandstation.Retries(retries))

		if len(entries) > 1 && err != nil {
			logrus.Error(err)
		}

		reads = append(reads, CVRead{
			Number: entry.Number,
			Value:  result,
			Err:    err,
		})
	}

	if len(reads) == 1 && reads[0].Err != nil {
		return nil, reads[0].Err
	}

	return reads, nil
}
