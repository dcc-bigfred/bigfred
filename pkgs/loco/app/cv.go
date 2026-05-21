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

func (app *LocoApp) ReadCVAction(mode string, locoId uint8, cvNumRaw string, verify bool, timeout time.Duration, retries uint8) error {
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.Station.CleanUp()

	// Try to parse as a single CV
	entries, parseErr := syntax.ParseCVString(cvNumRaw, ",")
	if parseErr == nil {
		var lastError error

		for _, entry := range entries {
			result, err := app.Station.ReadCV(commandstation.Mode(mode), commandstation.LocoCV{
				LocoId: commandstation.LocoAddr(locoId),
				Cv: commandstation.CV{
					Num: commandstation.CVNum(entry.Number),
				},
			}, commandstation.Verify(verify),
				commandstation.Timeout(timeout),
				commandstation.Retries(retries))

			// different formatting mode for multiple than for single entry
			if len(entries) > 1 {
				if err != nil {
					app.P.Printf("cv%d=ERROR\n", entry.Number)
					logrus.Error(err)
					lastError = err
				} else {
					app.P.Printf("cv%d=%d\n", entry.Number, result)
				}
			} else {
				if err != nil {
					return err
				}
				app.P.Printf("%d\n", result)
			}
		}
		return lastError
	}

	return fmt.Errorf("invalid format: %s", cvNumRaw)
}
