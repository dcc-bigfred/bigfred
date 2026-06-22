package app

import (
	"fmt"
	"time"

	"github.com/keskad/loco/pkgs/loco/commandstation"
	"github.com/keskad/loco/pkgs/loco/syntax"
	"github.com/sirupsen/logrus"
)

// SendCVAction parses cvNumRaw and writes each CV entry to the decoder on the given programming track.
// It initializes the command station for the duration of the call and waits settle between consecutive writes.
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

// ReadCVAction parses cvNumRaw and reads each listed CV from the decoder on the given programming track.
// For a single CV, a read error is returned directly; for multiple CVs, partial results are returned with per-entry errors.
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

// SetCVBitAction applies bit assignments with a read-modify-write for each affected CV number.
// Assignments targeting the same CV are merged into one read and one write; results include values before and after the change.
func (app *LocoApp) SetCVBitAction(mode string, locoId uint8, assignments []syntax.CVBitAssignment, verify bool, timeout time.Duration, settle time.Duration) ([]CVBitWriteResult, error) {
	if len(assignments) == 0 {
		return nil, fmt.Errorf("no CV bit assignments specified")
	}
	if cmdErr := app.InitializeCommandStation(); cmdErr != nil {
		return nil, cmdErr
	}
	defer app.Station.CleanUp()

	groups, order := groupCVBitAssignments(assignments)
	results := make([]CVBitWriteResult, 0, len(order))

	for _, cvNum := range order {
		bits := groups[cvNum]
		before, err := app.Station.ReadCV(commandstation.Mode(mode), commandstation.LocoCV{
			LocoId: commandstation.LocoAddr(locoId),
			Cv:     commandstation.CV{Num: commandstation.CVNum(cvNum)},
		}, commandstation.Timeout(timeout))
		if err != nil {
			return results, fmt.Errorf("failed to read CV%d: %w", cvNum, err)
		}

		after, err := syntax.ApplyCVBits(before, bits)
		if err != nil {
			return results, err
		}

		if err := app.Station.WriteCV(commandstation.Mode(mode), commandstation.LocoCV{
			LocoId: commandstation.LocoAddr(locoId),
			Cv:     commandstation.CV{Num: commandstation.CVNum(cvNum), Value: after},
		}, commandstation.Verify(verify), commandstation.Timeout(timeout)); err != nil {
			return results, fmt.Errorf("failed to write CV%d: %w", cvNum, err)
		}

		results = append(results, CVBitWriteResult{Number: cvNum, Before: before, After: after})
		time.Sleep(settle)
	}

	return results, nil
}

// groupCVBitAssignments collects bit assignments by CV number while preserving first-seen order.
// The returned slice order determines the sequence of read-modify-write operations in SetCVBitAction.
func groupCVBitAssignments(assignments []syntax.CVBitAssignment) (map[uint16][]syntax.CVBitAssignment, []uint16) {
	groups := make(map[uint16][]syntax.CVBitAssignment)
	order := make([]uint16, 0)
	for _, a := range assignments {
		if _, ok := groups[a.CVNumber]; !ok {
			order = append(order, a.CVNumber)
		}
		groups[a.CVNumber] = append(groups[a.CVNumber], a)
	}
	return groups, order
}
